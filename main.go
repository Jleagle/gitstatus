package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
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
	path string
	size int64
}

func main() {

	flag.Parse()
	readEnvs()

	// Install the latest version and exit
	if *flagUpdate {
		_, err := exec.Command("go", "install", "github.com/Jleagle/gitstatus@latest").Output()
		if err != nil {
			log.Println(err)
		} else {
			fmt.Println("App Updated")
		}
		return
	}

	// Get the base code dir
	baseDir := *flagDir
	if baseDir == "" {
		baseDir = "/users/" + os.Getenv("USER") + "/code"
	}

	// Get a list of every repo
	repos := scanAllDirs(baseDir, 1)
	if len(repos) == 0 {
		fmt.Println(baseDir + " does not contain any repos")
		return
	}

	// Filter by filter flag
	repos = filterReposByFilterFlag(repos)
	if len(repos) == 0 {
		fmt.Println("No repos match your directory & filter")
		return
	}

	// Pull repos with a loading bar
	rows := pullRepos(repos)

	// Show a table of results
	outputTable(rows, baseDir)
}

func readEnvs() {

	if os.Getenv(envStale) != "" {
		flagStale = boolP(true)
	}
	if os.Getenv(envFull) != "" {
		flagFull = boolP(true)
	}
	if d := os.Getenv(envDir); d != "" {
		flagDir = stringP(d)
	}
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

const maxConcurrent = 10

func pullRepos(repos []repoItem) (ret []rowItem) {

	// Run large repos first so you are not waiting on them at the end
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
			}

			// Make row
			row := rowItem{path: ri.path}

			var err error

			row.changedFiles, err = gitDiff(ri.path)
			if err != nil {
				log.Println(err)
				return
			}

			row.branch, err = gitBranch(ri.path)
			if err != nil {
				log.Println(err)
				return
			}

			if *flagStale {
				row.lastCommit, err = gitLog(ri.path)
				if err != nil {
					log.Println(err)
					return
				}
			}

			// Pull
			if *flagPull && !row.isDirty() {
				row.updated, err = gitPull(row, bar)
				if err != nil {
					log.Println(err)
					return
				}
			}

			ret = append(ret, row)
		}(r)
	}

	wg.Wait()
	bar.Finish()

	return ret
}

func outputTable(rows []rowItem, baseDir string) {

	sort.Slice(rows, func(i, j int) bool {
		if *flagStale {
			return rows[i].lastCommit.Unix() < rows[j].lastCommit.Unix()
		} else {
			return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
		}
	})

	header := table.Row{"Repo", "Branch", "Modified"}
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

		if row.show() {

			// Format path
			if !*flagFull {
				row.path = strings.TrimPrefix(row.path, baseDir)
			}

			// Format branch
			if len(row.branch) > 30 {
				row.branch = row.branch[:30] + "…"
			}
			if !row.isMain() {
				row.branch = color.RedString(row.branch)
			}

			// Format files
			row.changedFiles = color.RedString(row.changedFiles)

			//
			tr := table.Row{row.path, row.branch, row.changedFiles}

			if *flagPull {

				var action = ""
				if row.updated {
					action = color.GreenString("Updated")
				} else if !row.isDirty() {
					action = "Pulled"
				}

				tr = append(tr, action)
			}

			if *flagStale {

				// Format modified files
				modified := fmt.Sprintf("%d days", row.daysStale())
				if row.isStale() {
					modified = color.RedString(modified)
				}

				tr = append(tr, modified)
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
