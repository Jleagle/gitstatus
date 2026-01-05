package main

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

const (
	fDir      = "dir"
	fFilter   = "filter"
	fVersion  = "version"
	fMaxdepth = "maxdepth"
	fShort    = "short"
	fPull     = "pull"
	fAll      = "all"
)

// These variables are set by goreleaser's ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	flagDir      string
	flagFilter   string
	flagVersion  bool
	flagMaxDepth int
	flagShort    bool
	flagPull     bool
	flagAll      bool
)

func init() {

	cmd.Flags().StringVarP(&flagDir, fDir, "d", "", "Directory")
	cmd.Flags().StringVarP(&flagFilter, fFilter, "f", "", "Filter")
	cmd.Flags().BoolVarP(&flagVersion, fVersion, "v", false, "Version")
	cmd.Flags().IntVarP(&flagMaxDepth, fMaxdepth, "m", 2, "Max Depth")
	cmd.Flags().BoolVarP(&flagShort, fShort, "s", false, "Short Paths")
	cmd.Flags().BoolVarP(&flagPull, fPull, "p", false, "Pull Repos")
	cmd.Flags().BoolVarP(&flagAll, fAll, "a", false, "Show all Repos")

	cobra.OnInitialize(func() {

		viper.SetEnvPrefix("GITSTATUS")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		_ = viper.BindPFlag(fDir, cmd.Flags().Lookup(fDir))
		_ = viper.BindPFlag(fFilter, cmd.Flags().Lookup(fFilter))
		_ = viper.BindPFlag(fVersion, cmd.Flags().Lookup(fVersion))
		_ = viper.BindPFlag(fMaxdepth, cmd.Flags().Lookup(fMaxdepth))
		_ = viper.BindPFlag(fShort, cmd.Flags().Lookup(fShort))
		_ = viper.BindPFlag(fPull, cmd.Flags().Lookup(fPull))
		_ = viper.BindPFlag(fAll, cmd.Flags().Lookup(fAll))
	})
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var cmd = &cobra.Command{
	Use:  "gitstatus",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		if viper.GetBool(fVersion) {
			fmt.Println("Version: " + version)
			fmt.Println("Commit: " + commit)
			fmt.Println("Date: " + date)
			return
		}

		// Get the base code dir
		baseDir := cmp.Or(viper.GetString(fDir), "/users/"+os.Getenv("USER")+"/code")

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
	},
}

type repoItem struct {
	path string
	size int64
}

func scanAllDirs(dir string, depth int) (ret []repoItem) {

	if depth > viper.GetInt(fMaxdepth) {
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

	var filter = viper.GetString(fFilter)
	if filter == "" {
		return repos
	}

	pieces := strings.Split(filter, ",")
	var includes, excludes []string
	for _, piece := range pieces {
		piece = strings.TrimSpace(strings.ToLower(piece))
		if strings.HasPrefix(piece, "!") {
			excludes = append(excludes, piece)
		} else {
			includes = append(includes, piece)
		}
	}

	for _, repo := range repos {

		repoPath := strings.ToLower(repo.path)

		// Check positives
		if len(includes) > 0 {
			matched := false
			for _, v := range includes {
				if v != "" && strings.Contains(repoPath, v) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Check negatives
		excluded := false
		for _, v := range excludes {
			v = strings.TrimPrefix(v, "!")
			if v != "!" && strings.Contains(repoPath, v) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		ret = append(ret, repo)
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
			if viper.GetBool(fPull) && !row.isDirty() {
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
	if viper.GetBool(fPull) {
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
			if viper.GetBool(fShort) {
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

			if viper.GetBool(fPull) {

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
		fmt.Println(color.BlueString(fmt.Sprintf("%d repos with nothing to report, use -all to show them\n", hidden)))
	}
}
