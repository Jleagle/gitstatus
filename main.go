package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	baseDir      = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory to scan")
	doFetch      = flag.Bool("fetch", false, "Fetch repos")
	doPull       = flag.Bool("pull", false, "Pull repos")
	maxDepth     = flag.Int("depth", 2, "Max directory depth")
	showAllRepos = flag.Bool("all", false, "Show all repos")
	showFiles    = flag.Bool("files", false, "Show modified files")

	repos []repo
)

func main() {
	flag.Parse()
	scanRepos(*baseDir, 1)
	handleRepos()
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
				repos = append(repos, repo{path: d, repo: r})
				continue
			}

			if depth <= *maxDepth {
				scanRepos(path.Join(dir, e.Name()), depth+1)
			}
		}
	}
}

func handleRepos() {

	bar := pb.New(len(repos))
	bar.SetRefreshRate(time.Millisecond * 200)
	bar.SetWriter(os.Stdout)
	bar.Start()

	tab := table.NewWriter()
	tab.SetOutputMirror(os.Stdout)
	tab.AppendHeader(table.Row{"#", "Repo", "Status", "Actions", "Files"})
	tab.SetStyle(table.StyleRounded)

	for k, repo := range repos {

		bar.Increment()

		tree, err := repo.repo.Worktree()
		if err != nil {
			log.Println(err)
		}

		tree.Excludes = append(
			tree.Excludes,
			gitignore.ParsePattern(".idea/", nil),
			gitignore.ParsePattern(".DS_Store", nil),
			gitignore.ParsePattern(".tiltbuild/", nil),
		)

		patterns, _ := gitignore.ReadPatterns(tree.Filesystem, nil)
		tree.Excludes = append(tree.Excludes, patterns...)

		status, err := tree.Status()
		if err != nil {
			log.Println(err)
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

		var changed = changedCount(status)
		var s = fmt.Sprintf("%d modified files", changed)
		if changed == 0 {
			s = ""
		}

		if len(msg) > 0 || s != "" || *showAllRepos {
			tab.AppendRow([]interface{}{
				k + 1,
				strings.TrimPrefix(repo.path, *baseDir),
				s,
				strings.Join(msg, ", "),
				listFiles(status),
			})
		}
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
	if !*showFiles {
		return ""
	}
	var files []string
	for k := range s {
		files = append(files, k)
	}
	return strings.Join(files, "\n")
}
