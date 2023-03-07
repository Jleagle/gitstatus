package main

import (
	"flag"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/jedib0t/go-pretty/v6/table"
)

const maxDepth = 2

var (
	baseDir   = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory to scan")
	doFetch   = flag.Bool("fetch", false, "Fetch repos")
	doPull    = flag.Bool("pull", false, "Pull repos")
	showFiles = flag.Bool("files", false, "Show modified files")
	filter    = flag.String("filter", "", "Filter repos")

	gitIgnore = []string{
		".DS_Store",
		".idea/",
		".tiltbuild/",
	}

	repos []repo
)

func main() {
	flag.Parse()
	scanRepos(*baseDir, 1)
	output()
}

type repo struct {
	path string
	repo *git.Repository
}

func scanRepos(dir string, depth int) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {

		if e.IsDir() {

			d := path.Join(dir, e.Name())

			r, err := git.PlainOpen(d)
			if err == nil {
				if *filter == "" || strings.Contains(d, *filter) {
					repos = append(repos, repo{path: d, repo: r})
					continue
				}
			}

			if depth <= maxDepth {
				scanRepos(path.Join(dir, e.Name()), depth+1)
			}
		}
	}
}

func output() {

	sort.Slice(repos, func(i, j int) bool {
		return strings.ToLower(repos[i].path) < strings.ToLower(repos[j].path)
	})

	bar := pb.New(len(repos))
	bar.SetRefreshRate(time.Millisecond * 200)
	bar.SetWriter(os.Stdout)
	bar.Start()

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(table.Row{"#", "Repo", "Actions", "# Modified Files", "Files"})
	tab.SetStyle(table.StyleRounded)

	for k, repo := range repos {
		func() {

			defer bar.Increment()

			tree, err := repo.repo.Worktree()
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

			var msg []string

			if status.IsClean() {

				if *doPull {
					err = tree.Pull(&git.PullOptions{})
					if err != nil && err.Error() != "already up-to-date" {
						msg = append(msg, err.Error())
					} else {
						msg = append(msg, "git pull")
					}
				} else if *doFetch {
					err := repo.repo.Fetch(&git.FetchOptions{})
					if err != nil && err.Error() != "already up-to-date" {
						msg = append(msg, err.Error())
					} else {
						msg = append(msg, "git fetch")
					}
				}
			}

			tab.AppendRow([]interface{}{
				k + 1,
				strings.TrimPrefix(repo.path, *baseDir),
				strings.Join(msg, ", "),
				strconv.Itoa(changedCount(status)),
				listFiles(status),
			})
		}()
	}

	bar.Finish()
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

func listFiles(s git.Status) string {
	var files []string
	for k := range s {
		if len(files) <= 5 || *showFiles {
			files = append(files, k)
		}
	}
	return strings.Join(files, "\n")
}
