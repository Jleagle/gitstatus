package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	maxDepth      = 2
	maxConcurrent = 10
	maxRetries    = 1

	envFullPaths = "GITSTATUS_FULL_PATHS"
	envDir       = "GITSTATUS_DIR"
)

var (
	flagBaseDir   = flag.String("d", "", "Directory to scan")
	flagFilter    = flag.String("filter", "", "Filter repos, comma delimited")
	flagPull      = flag.Bool("pull", false, "Pull repos")
	flagShowFiles = flag.Bool("files", false, "Show all modified files")
	flagShowAll   = flag.Bool("all", false, "Show all repos, even if no changes")
	flagUpdate    = flag.Bool("update", false, "Update this app before running")
	flagFullPaths = flag.Bool("fullpaths", false, "Show the full repo path")
)

type repoItem struct {
	path string
	repo *git.Repository
	size int64
}

type rowItem struct {
	path          string
	branch        string
	branchNonMain bool
	pulled        bool
	pulledChanges bool
	pulledError   error
	status        git.Status
}

func main() {

	flag.Parse()

	if *flagUpdate {
		b, err := exec.Command("go", "install", "github.com/Jleagle/gitstatus@latest").Output()
		if err != nil {
			log.Println(err)
		} else if len(b) == 0 {
			fmt.Println("App Updated")
		} else {
			fmt.Println(string(b))
		}
		return
	}

	baseDir := getBaseDir()

	repos := scanAllDirs(baseDir, 1)
	if len(repos) == 0 {
		fmt.Printf("%s does not contain any repos\n", baseDir)
		return
	}

	repos = filterReposByFilterFlag(repos)
	if len(repos) == 0 {
		fmt.Println("No repos match your directory & filter")
		return
	}

	rows := pullRepos(repos, baseDir)

	outputTable(rows)
}

func getBaseDir() string {

	d := *flagBaseDir
	if d == "" {
		d = os.Getenv(envDir)
	}
	if d == "" {
		d = "/users/" + os.Getenv("USER") + "/code"
	}

	return d
}

func scanAllDirs(dir string, depth int) (ret []repoItem) {

	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Println(err)
		return
	}

	for _, e := range entries {
		if e.IsDir() {

			d := path.Join(dir, e.Name())

			file, err := os.Stat(path.Join(d, ".git", "index"))
			if err != nil {
				ret = append(ret, scanAllDirs(d, depth+1)...)
			} else {
				ret = append(ret, repoItem{path: d, size: file.Size()})
			}
		}
	}

	return ret
}

func filterReposByFilterFlag(repos []repoItem) (ret []repoItem) {

	if *flagFilter == "" {
		return repos
	}

	pieces := strings.Split(*flagFilter, ",")
	for _, r := range repos {
		for _, piece := range pieces {
			if strings.Contains(strings.ToLower(r.path), strings.TrimSpace(strings.ToLower(piece))) {
				ret = append(ret, r)
				break
			}
		}
	}

	return ret
}

func pullRepos(repos []repoItem, baseDir string) (ret []rowItem) {

	// Get global gitignore patterns
	globalPatterns, err := gitignore.LoadGlobalPatterns(osfs.New("/"))
	if err != nil {
		log.Println(err)
	}

	// Run large repos first for speed
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].size > repos[j].size
	})

	//
	bar := pb.New(len(repos))
	bar.SetRefreshRate(time.Millisecond * 200)
	bar.SetWriter(os.Stdout)
	bar.SetWidth(100)
	bar.Start()

	var guard = make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, r := range repos {

		wg.Add(1)
		guard <- struct{}{}

		go func(path string) {

			defer func() {
				bar.Increment()
				wg.Done()
				<-guard
			}()

			repo, err := git.PlainOpen(path)
			if err != nil {
				log.Println(err)
				return
			}

			tree, err := repo.Worktree()
			if err != nil {
				log.Println(err)
				return
			}

			head, err := repo.Head()
			if err != nil {
				log.Println(err)
				return
			}

			// Load gitignores
			patterns, err := gitignore.ReadPatterns(tree.Filesystem, nil)
			if err != nil {
				log.Println(err)
				return
			}

			tree.Excludes = append(tree.Excludes, globalPatterns...)
			tree.Excludes = append(tree.Excludes, patterns...)

			// Get modified files
			status, err := tree.Status()
			if err != nil {
				log.Println(err)
				return
			}

			// Trim the base bath off the rep paths
			if !*flagFullPaths && os.Getenv(envFullPaths) == "" {
				path = strings.TrimPrefix(path, baseDir)
			}

			// Make row
			row := rowItem{
				path:   path,
				branch: strings.TrimPrefix(string(head.Name()), "refs/heads/"),
				status: status,
			}

			if row.branch != "master" && row.branch != "main" {
				row.branch = color.RedString(row.branch)
				row.branchNonMain = true
			}

			// Pull
			if status.IsClean() {
				if *flagPull {
					row = pull(tree, bar, row, 1)
				}
			} else {
				//goland:noinspection GoErrorStringFormat
				row.pulledError = errors.New("Unclean")
			}

			ret = append(ret, row)

		}(r.path)
	}

	wg.Wait()
	bar.Finish()

	return ret
}

func outputTable(rows []rowItem) {

	// Alphabetical for display
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
	})

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(table.Row{"Repo", "Branch", "Status", "Modified"})
	tab.SetStyle(table.StyleRounded)
	//tab.SortBy([]table.SortBy{{Name: "Repo", Mode: table.Asc}}) // Is this case insensitive?

	hidden := 0

	for _, row := range rows {

		if *flagShowAll || row.branchNonMain || row.pulledChanges || len(row.status) > 0 || row.pulledError != nil {

			var action = ""
			if row.pulledError != nil {
				action = color.RedString(row.pulledError.Error())
			} else if row.pulledChanges {
				action = color.GreenString("Updated")
			} else if row.pulled {
				action = "Pulled"
			}

			tab.AppendRow(table.Row{
				row.path,
				row.branch,
				action,
				listFiles(row.status),
			})

			continue
		}

		hidden++
	}

	if tab.Length() > 0 {
		tab.Render()
	}

	if hidden > 0 {
		fmt.Println(color.GreenString(fmt.Sprintf("%d repos with nothing to report\n", hidden)))
	}
}

var statusNames = map[git.StatusCode]string{
	' ': "Unmodified",
	'?': "Untracked",
	'M': "Modified",
	'A': "Added",
	'D': "Deleted",
	'R': "Renamed",
	'C': "Copied",
	'U': "UpdatedButUnmerged",
}

func listFiles(s git.Status) string {

	if *flagShowFiles {

		var files []string
		for k, v := range s {
			files = append(files, strings.ToUpper(statusNames[v.Worktree])+": "+k)
		}
		return strings.Join(files, "\n")

	} else {

		var count int
		for _, status := range s {
			if status.Worktree != git.Unmodified || status.Staging != git.Unmodified {
				count++
			}
		}

		if count > 0 {
			return color.RedString(strconv.Itoa(count))
		} else {
			return strconv.Itoa(count)
		}
	}
}

func pull(tree *git.Worktree, bar *pb.ProgressBar, row rowItem, attempt int) rowItem {

	err := tree.Pull(&git.PullOptions{})
	if err != nil {

		if err.Error() == "already up-to-date" {
			row.pulled = true
			return row
		} else if strings.HasPrefix(err.Error(), "ssh:") {
			bar.Finish()
			fmt.Println(color.RedString(err.Error()))
			os.Exit(0)
		} else if attempt <= maxRetries {
			return pull(tree, bar, row, attempt+1)
		}

		row.pulledError = err
		return row
	}

	row.pulledChanges = true

	return row
}
