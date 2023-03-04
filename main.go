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
	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	baseDir    = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory to scan")
	doFetch    = flag.Bool("gf", false, "Fetch repos")
	doPull     = flag.Bool("gp", false, "Pull repos clean repos")
	doComposer = flag.Bool("ci", false, "Composer install")

	repos []repo
)

func main() {
	flag.Parse()
	scanRepos(*baseDir)
	handleRepos()
}

type repo struct {
	path string
	repo *git.Repository
}

func scanRepos(dir string) {

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

			scanRepos(path.Join(dir, e.Name()))
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
	tab.AppendHeader(table.Row{"#", "Repo", "Status", "Actions"})
	tab.SetStyle(table.StyleRounded)

	for k, repo := range repos {

		bar.Increment()

		tree, err := repo.repo.Worktree()
		if err != nil {
			log.Println(err)
		}

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

		if *doComposer {

			// Is composer?
			//d := path.Join(dir, e.Name(), "composer.json")
			//if _, err := os.Stat(d); err == nil {
			//	todo, composer
			//}

			msg = append(msg, "composer install")
		}

		var changed = changedCount(status)
		var s = fmt.Sprintf("%d modified files", changed)
		if changed == 0 {
			s = "Clean"
		}

		//if len(msg) == 0 {
		//	msg = append(msg, "none")
		//}

		tab.AppendRow([]interface{}{
			k + 1,
			strings.TrimPrefix(repo.path, *baseDir),
			s,
			strings.Join(msg, ", "),
		})
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
