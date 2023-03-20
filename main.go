package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	maxDepth      = 2
	maxConcurrent = 10
)

var (
	baseDir   = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory to scan")
	filter    = flag.String("filter", "", "Filter repos, comma delimited")
	doPull    = flag.Bool("pull", false, "Pull repos")
	showFiles = flag.Bool("files", false, "Show all modified files")
	showAll   = flag.Bool("all", false, "Show all repos, even if no changes")

	gitIgnore = []string{".DS_Store", ".idea/", ".tiltbuild/"}
)

func main() {

	flag.Parse()

	repos := map[string]*git.Repository{}
	scanRepos(repos, *baseDir, 1)
	handleRepos(repos)
}

func scanRepos(repos map[string]*git.Repository, dir string, depth int) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Println(err)
		return
	}

	for _, e := range entries {

		if e.IsDir() {

			d := path.Join(dir, e.Name())

			// Load repo
			r, err := git.PlainOpen(d)
			if err != nil {
				if err != git.ErrRepositoryNotExists {
					log.Println(err)
				}
				if depth <= maxDepth {
					scanRepos(repos, path.Join(dir, e.Name()), depth+1)
				}
				continue
			}

			// Filter
			include := *filter == ""
			if !include {
				pieces := strings.Split(*filter, ",")
				for _, piece := range pieces {
					if strings.Contains(strings.ToLower(d), strings.TrimSpace(strings.ToLower(piece))) {
						include = true
						break
					}
				}
			}

			// Add to map
			if include {
				repos[d] = r
			}
		}
	}
}

type row struct {
	path   string
	branch string
	action string
	status git.Status
}

func handleRepos(repos map[string]*git.Repository) {

	if len(repos) == 0 {
		fmt.Println("No repos match your directory & filter")
		return
	}

	bar := pb.New(len(repos))
	bar.SetRefreshRate(time.Millisecond * 200)
	bar.SetWriter(os.Stdout)
	bar.SetWidth(100)
	bar.Start()

	var guard = make(chan struct{}, maxConcurrent)
	var wg = sync.WaitGroup{}
	var i = 0
	var rows []row

	for k, v := range repos {

		i++
		wg.Add(1)
		guard <- struct{}{}

		go func(i int, path string, repo *git.Repository) {

			defer func() {
				bar.Increment()
				wg.Done()
				<-guard
			}()

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

			// Add ignored files
			for _, v := range gitIgnore {
				tree.Excludes = append(tree.Excludes, gitignore.ParsePattern(v, nil))
			}

			patterns, err := gitignore.ReadPatterns(tree.Filesystem, nil)
			if err != nil {
				log.Println(err)
				return
			}

			tree.Excludes = append(tree.Excludes, patterns...)

			//
			status, err := tree.Status()
			if err != nil {
				log.Println(err)
				return
			}

			// Pull
			var action string
			if status.IsClean() && *doPull {
				err = tree.Pull(&git.PullOptions{})
				if err != nil {
					if err.Error() == "already up-to-date" {
						action = "pulled"
					} else {
						action = color.RedString("ERROR: " + err.Error())
					}
				} else {
					action = color.GreenString("pulled, updated")
				}
			}

			branch := strings.TrimPrefix(string(head.Name()), "refs/heads/")
			if branch != "master" && branch != "main" {
				branch = color.GreenString(branch)
			}

			rows = append(rows, row{
				path:   path,
				branch: branch,
				action: action,
				status: status,
			})

		}(i, k, v)
	}

	wg.Wait()
	bar.Finish()

	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
	})

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(table.Row{"Repo", "Branch", "Actions", "Modified"})
	tab.SetStyle(table.StyleRounded)
	//tab.SortBy([]table.SortBy{{Name: "Repo", Mode: table.Asc}}) // Is this case insensitive?

	hidden := 0

	for _, row := range rows {

		if *showAll || (row.branch != "master" && row.branch != "main") || (row.action != "pulled" && row.action != "") || len(row.status) > 0 {

			tab.AppendRow(table.Row{
				strings.TrimPrefix(row.path, *baseDir),
				row.branch,
				row.action,
				listFiles(row.status),
			})
		} else {
			hidden++
		}
	}

	if tab.Length() > 0 {
		tab.Render()
	}

	if hidden > 0 {
		fmt.Printf("%d repos with nothing to report\n\n", hidden)
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

	if *showFiles {

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

		var ret = ""
		if count > 0 {
			ret = color.GreenString(strconv.Itoa(count))
		}

		return ret
	}
}
