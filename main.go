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
	envDir   = "GITSTATUS_DIR"
	envFull  = "GITSTATUS_FULL"
	envStale = "GITSTATUS_STALE"
)

var (
	flagDir     = flag.String("dir", "", "Directory to scan")
	flagFilter  = flag.String("filter", "", "Filter repos, comma delimited")
	flagFull    = flag.Bool("full", false, "Show the full repo path")
	flagPull    = flag.Bool("pull", false, "Pull repos")
	flagShowAll = flag.Bool("all", false, "Show all repos, even if no changes")
	flagStale   = flag.Bool("stale", false, "Always show stale")
	flagUpdate  = flag.Bool("update", false, "Update this app before running")
	flagVerbose = flag.Bool("v", false, "Log slow repos")
)

type repoItem struct {
	path        string
	repo        *git.Repository
	size        int64
	timeStarted time.Time
	lastCommit  time.Time
}

type rowItem struct {
	path           string
	branch         string
	branchNonMain  bool
	pulled         bool
	pulledChanges  bool
	pulledError    error
	files          string
	commitDays     string
	commitDaysOver bool
}

func main() {

	flag.Parse()

	if os.Getenv(envStale) != "" {
		flagStale = boolP(true)
	}
	if os.Getenv(envFull) != "" {
		flagFull = boolP(true)
	}
	if d := os.Getenv(envDir); d != "" {
		flagDir = stringP(d)
	}

	if *flagUpdate {
		_, err := exec.Command("go", "install", "github.com/Jleagle/gitstatus@latest").Output()
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println("App Updated")
		}
		return
	}

	baseDir := *flagDir
	if baseDir == "" {
		baseDir = "/users/" + os.Getenv("USER") + "/code"
	}

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

const maxDepth = 2

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

const (
	maxConcurrent = 10
	slowRepos     = time.Second * 3
)

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
	if !*flagVerbose {
		bar.Start()
	}

	var guard = make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, r := range repos {

		wg.Add(1)
		guard <- struct{}{}

		go func(ri repoItem) {

			defer func() {
				bar.Increment()
				wg.Done()
				<-guard
			}()

			if *flagVerbose {

				log.Printf(ri.path + " started")
				ri.timeStarted = time.Now()

				defer func(ri repoItem) {
					if dur := time.Now().Sub(ri.timeStarted); dur > slowRepos {
						log.Println(color.RedString(ri.path + " took " + dur.Truncate(time.Second/100).String()))
					}
				}(ri)
			}

			repo, err := git.PlainOpen(ri.path)
			if err != nil {
				log.Println(err)
				return
			}

			if *flagStale {

				gitLog, err := repo.Log(&git.LogOptions{Order: git.LogOrderDefault})
				if err != nil {
					log.Println(err)
				}

				for {
					commit, err := gitLog.Next()
					if err != nil {
						log.Println(err)
					}
					ri.lastCommit = commit.Committer.When
					break
				}
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
			if !*flagFull {
				ri.path = strings.TrimPrefix(ri.path, baseDir)
			}

			// Check if stale is over limit
			days, over := commitDays(ri.lastCommit)

			// Make row
			row := rowItem{
				path:           ri.path,
				branch:         strings.TrimPrefix(string(head.Name()), "refs/heads/"),
				files:          listFiles(status),
				commitDays:     days,
				commitDaysOver: over,
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
		}(r)
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

	header := table.Row{"Repo", "Modified", "Branch"}
	if *flagPull {
		header = append(header, "Pull")
	}
	if *flagStale {
		header = append(header, "Stale")
	}

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(header)
	tab.SetStyle(table.StyleRounded)

	hidden := 0

	for _, row := range rows {

		if *flagShowAll || row.branchNonMain || row.pulledError != nil ||
			(*flagPull && row.pulledChanges) ||
			(*flagStale && row.commitDaysOver) {

			tr := table.Row{row.path, row.files, row.branch}
			if *flagPull {

				var action = ""
				if row.pulledError != nil {
					action = color.RedString(row.pulledError.Error())
				} else if row.pulledChanges {
					action = color.GreenString("Updated")
				} else if row.pulled {
					action = "Pulled"
				}

				tr = append(tr, action)
			}
			if *flagStale {
				tr = append(tr, row.commitDays)
			}

			tab.AppendRow(tr)

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

func listFiles(s git.Status) string {

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

const staleDays = 180 // Days

// commitDays returns daysStale, isStale
func commitDays(t time.Time) (string, bool) {

	if t.IsZero() {
		return "", false
	}

	d := time.Now().Sub(t)
	days := int(d.Hours() / 24)
	daysStr := fmt.Sprintf("%d days", days)

	if days > staleDays {
		return color.RedString(daysStr), true
	}

	return daysStr, false
}

const maxRetries = 1

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

// Helpers
func boolP(b bool) *bool {
	return &b
}

func stringP(b string) *string {
	return &b
}
