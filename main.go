package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	fDir      = "dir"
	fFilter   = "filter"
	fVersion  = "version"
	fMaxdepth = "maxdepth"
	fShort    = "short"
	fPull     = "pull"
	fAll      = "all"
	fPlain    = "plain"
)

const (
	workers     = 10
	pullWorkers = 64 // Pulls spend most of their time waiting on the remote
)

var (
	statusSem = make(chan struct{}, workers)
	pullSem   = make(chan struct{}, pullWorkers)
)

// concurrency is how many repos can be in flight at once
func concurrency() int {
	if viper.GetBool(fPull) {
		return pullWorkers
	}
	return workers
}

// These variables are set by goreleaser's ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {

	log.SetFlags(0)

	cmd.Flags().StringP(fDir, "d", "", "Directory")
	cmd.Flags().StringP(fFilter, "f", "", "Filter")
	cmd.Flags().BoolP(fVersion, "v", false, "Version")
	cmd.Flags().IntP(fMaxdepth, "m", 2, "Max Depth")
	cmd.Flags().BoolP(fShort, "s", false, "Short Paths")
	cmd.Flags().BoolP(fPull, "p", false, "Pull Repos")
	cmd.Flags().BoolP(fAll, "a", false, "Show all Repos")
	cmd.Flags().Bool(fPlain, false, "Plain Output")

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
		_ = viper.BindPFlag(fPlain, cmd.Flags().Lookup(fPlain))
	})
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

var cmd = &cobra.Command{
	Use:  "gitstatus",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		if viper.GetBool(fVersion) {
			log.Println("Version: " + version)
			log.Println("Commit: " + commit)
			log.Println("Date: " + date)
			return
		}

		// Get the base code dir
		baseDir := viper.GetString(fDir)
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Println("unable to determine home directory: " + err.Error())
				return
			}
			baseDir = filepath.Join(home, "code")
		}

		// Get a list of every repo
		repos := scanAllDirs(baseDir, 1)
		if len(repos) == 0 {
			log.Println(baseDir + " does not contain any repos")
			return
		}

		// Filter by filter flag
		repos = filterReposByFilterFlag(repos)
		if len(repos) == 0 {
			log.Println("No repos match your directory & filter")
			return
		}

		if viper.GetBool(fPlain) || !term.IsTerminal(os.Stdout.Fd()) {
			printPlain(os.Stdout, pullRepos(repos, noopReporter{}), baseDir)
			return
		}

		runLive(repos, baseDir)
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

			d := filepath.Join(dir, e.Name())

			if _, err := os.Stat(filepath.Join(d, ".git")); err != nil {
				ret = append(ret, scanAllDirs(d, depth+1)...)
			} else {
				var size int64
				if idx, err := os.Stat(filepath.Join(d, ".git", "index")); err == nil {
					size = idx.Size()
				}
				ret = append(ret, repoItem{path: d, size: size})
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
			if v != "" && strings.Contains(repoPath, v) {
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

// reporter receives per-repo progress from the workers
type reporter interface {
	stage(path, stage string)
	done(row rowItem)
}

type noopReporter struct{}

func (noopReporter) stage(string, string) {}
func (noopReporter) done(rowItem)         {}

func pullRepos(repos []repoItem, rep reporter) (rows []rowItem) {

	// Run large repos first so you are not waiting on them at the end
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].size > repos[j].size
	})

	wg := sync.WaitGroup{}

	var mu sync.Mutex

	for _, r := range repos {

		wg.Go(func() {
			row := processRepo(r.path, rep)

			mu.Lock()
			rows = append(rows, row)
			mu.Unlock()

			rep.done(row)
		})
	}

	wg.Wait()

	return rows
}

// processRepo builds the result row for a single repo, pulling if requested
func processRepo(path string, rep reporter) rowItem {

	row := readRepo(path, rep)
	if row.error != nil {
		return row
	}

	// Pull; gitPull skips repos it cannot fast-forward without conflicts
	if viper.GetBool(fPull) {
		pullSem <- struct{}{}
		defer func() { <-pullSem }()

		rep.stage(path, "pulling…")
		var err error
		row.updated, row.skipped, err = gitPull(row)
		if err != nil {
			row.error = err
		}
	}

	return row
}

// readRepo gets the working tree changes and branch of a repo
func readRepo(path string, rep reporter) (row rowItem) {

	statusSem <- struct{}{}
	defer func() { <-statusSem }()

	row.path = path

	rep.stage(path, "reading status…")

	var err error

	row.added, row.modified, row.deleted, err = gitDiff(path)
	if err != nil {
		row.error = err
		return row
	}

	row.branch, row.detached, err = gitBranch(path)
	if err != nil {
		row.error = err
	}

	return row
}
