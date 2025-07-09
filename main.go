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
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/sync/errgroup"
)

const envDir = "GITSTATUS_DIR"
const envFull = "GITSTATUS_FULL"

var (
	flagDir      = flag.String("dir", "", "Directory to scan")
	flagFilter   = flag.String("filter", "", "Filter repos, comma delimited")
	flagMaxDepth = flag.Int("depth", 2, "Max nested depth to scan for")
	flagFull     = flag.Bool("full", false, "Show the full repo path")
	flagPull     = flag.Bool("pull", false, "Pull repos")
	flagShowAll  = flag.Bool("all", false, "Show all repos, even if no changes")
	flagUpdate   = flag.Bool("update", false, "Update this app before running")
)

type repoItem struct {
	path string
	size int64
}

func main() {

	flag.Parse()

	// Set flags from env
	if os.Getenv(envFull) != "" {
		flagFull = boolP(true)
	}
	if d := os.Getenv(envDir); d != "" {
		flagDir = stringP(d)
	}

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

func scanAllDirs(dir string, depth int) (ret []repoItem) {

	if depth > *flagMaxDepth {
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

func pullRepos(repos []repoItem) (rows []rowItem) {

	// Run large repos first so you are not waiting on them at the end
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].size > repos[j].size
	})

	//
	bar := pb.New(len(repos))
	bar.SetRefreshRate(time.Millisecond * 200)
	bar.SetWriter(os.Stdout)
	bar.SetWidth(100)
	bar.Start()

	wg := errgroup.Group{}
	wg.SetLimit(10)

	for _, r := range repos {

		r := r

		wg.Go(func() error {

			defer bar.Increment()

			// Make row
			row := rowItem{path: r.path}

			defer func() {
				rows = append(rows, row)
			}()

			var err error

			row.changedFiles, err = gitDiff(r.path)
			if err != nil {
				row.error = err
				return nil
			}

			row.branch, err = gitBranch(r.path)
			if err != nil {
				row.error = err
				return nil
			}

			// Pull
			if *flagPull && !row.isDirty() {
				row.updated, err = gitPull(row)
				if err != nil {
					row.error = err
					return nil
				}
			}

			return nil
		})
	}

	err := wg.Wait()
	if err != nil {
		log.Println(err)
	}

	bar.Finish()

	return rows
}

func outputTable(rows []rowItem, baseDir string) {

	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
	})

	var hasErrors bool
	for _, v := range rows {
		if v.error != nil {
			hasErrors = true
			break
		}
	}

	header := table.Row{"Repo", "Branch", "Modified"}
	if *flagPull {
		header = append(header, "Pull")
	}
	if hasErrors {
		header = append(header, "Error")
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
			if row.changedFiles != "" {
				row.changedFiles = color.RedString(row.changedFiles)
			}

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

			if hasErrors {
				if row.error != nil {
					tr = append(tr, row.error.Error())
				} else {
					tr = append(tr, "")
				}
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
		fmt.Println(color.GreenString(fmt.Sprintf("%d repos with nothing to report, use -all to show them\n", hidden)))
	}
}
