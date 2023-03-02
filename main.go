package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
)

var (
	baseDir   = flag.String("d", "/users/"+os.Getenv("USER")+"/code", "Directory")
	doPull    = flag.Bool("pull", false, "Pull repos with no changes")
	doInstall = flag.Bool("install", false, "Composer install")

	count int
)

func main() {
	flag.Parse()
	loopDir(*baseDir, 0)
}

func loopDir(dir string, indent int) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {

		if e.IsDir() {

			r, err := git.PlainOpen(path.Join(dir, e.Name()))
			if err == nil {

				count++

				err = r.Fetch(&git.FetchOptions{})
				if err != nil && err.Error() != "already up-to-date" {
					log.Println(err)
				}

				tree, err := r.Worktree()
				if err != nil {
					log.Println(err)
				}

				status, err := tree.Status()
				if err != nil {
					log.Println(err)
				}

				var msg string

				if *doPull && status.IsClean() {

					err = tree.Pull(&git.PullOptions{})
					if err != nil && err.Error() != "already up-to-date" {
						log.Println(err)
					} else {
						msg = "PULLED"
					}
				}

				fmt.Println([]interface{}{
					count,
					strings.TrimPrefix(path.Join(dir, e.Name()), *baseDir+"/"),
					changedCount(status),
					msg,
				})

				//head, _ := r.Head()
				//fmt.Println(head.String())

				// Is composer?
				//d := path.Join(dir, e.Name(), "composer.json")
				//if _, err := os.Stat(d); err == nil {
				//	todo, composer
				//}

				continue
			}

			loopDir(path.Join(dir, e.Name()), indent+1)
		}
	}
}

func changedCount(s git.Status) (c int) {

	for _, status := range s {
		if status.Worktree == git.Unmodified && status.Staging == git.Unmodified {
			c++
		}
	}

	return c
}
