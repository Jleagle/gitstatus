package main

import (
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	maxDepth      = 2
	maxConcurrent = 10
)

var (
	//doFetch   = flag.Bool("fetch", false, "Fetch repos")
	baseDir   = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory to scan")
	doPull    = flag.Bool("pull", false, "Pull repos")
	showFiles = flag.Bool("files", false, "Show all modified files")
	filter    = flag.String("filter", "", "Filter repos, comma delimited")

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
	path    string
	actions []string
	status  git.Status
}

func handleRepos(repos map[string]*git.Repository) {

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

			for _, v := range gitIgnore {
				tree.Excludes = append(tree.Excludes, gitignore.ParsePattern(v, nil))
			}

			patterns, err := gitignore.ReadPatterns(tree.Filesystem, nil)
			if err != nil {
				log.Println(err)
				return
			}

			tree.Excludes = append(tree.Excludes, patterns...)

			status, err := tree.Status()
			if err != nil {
				log.Println(err)
				return
			}

			var actions []string

			if status.IsClean() {

				if *doPull {
					err = tree.Pull(&git.PullOptions{})
					if err != nil && err.Error() != "already up-to-date" {
						actions = append(actions, err.Error())
					} else {
						actions = append(actions, "git pull")
					}
				}
				//else if *doFetch {
				//	err := repo.Fetch(&git.FetchOptions{})
				//	if err != nil && err.Error() != "already up-to-date" {
				//		actions = append(actions, err.Error())
				//	} else {
				//		actions = append(actions, "git fetch")
				//	}
				//}
			}

			rows = append(rows, row{path: path, actions: actions, status: status})

		}(i, k, v)
	}

	wg.Wait()
	bar.Finish()

	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
	})

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(table.Row{"#", "Repo", "Actions", "# Modified Files", "Files"})
	tab.SetStyle(table.StyleRounded)

	for k, row := range rows {
		tab.AppendRow(table.Row{
			strconv.Itoa(k + 1),
			strings.TrimPrefix(row.path, *baseDir),
			strings.Join(row.actions, ", "),
			strconv.Itoa(changedCount(row.status)),
			strings.Join(listFiles(row.status), "\n"),
		})
	}

	tab.Render()
}

func changedCount(s git.Status) (c int) {
	for _, status := range s {
		if status.Worktree != git.Unmodified || status.Staging != git.Unmodified {
			c++
		}
	}
	return c
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

func listFiles(s git.Status) []string {
	var files []string
	for k, v := range s {
		if len(files) < 3 || *showFiles {

			if !*showFiles {
				k = filepath.Base(k)
			}

			files = append(files, strings.ToUpper(statusNames[v.Worktree])+": "+k)
		}
	}
	return files
}
