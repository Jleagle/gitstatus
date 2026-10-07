package main

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/viper"
)

type rowItem struct {
	path     string //
	branch   string //
	detached bool   //
	added    int    // New files
	modified int    // Modified files
	deleted  int    // Deleted files
	updated  bool   // If something was pulled down
	skipped  string // Why the pull was skipped
	error    error  //
}

func (r rowItem) show() bool {
	return viper.GetBool(fAll) || !r.isMain() || r.isDirty() || r.updated || r.skipped != "" || (r.error != nil)
}

func (r rowItem) isMain() bool {
	return r.branch == "master" || r.branch == "main" || r.branch == "trunk" || r.branch == "develop" || r.branch == "dev"
}

func (r rowItem) isDetached() bool {
	return r.detached
}

func (r rowItem) isDirty() bool {
	return r.added+r.modified+r.deleted > 0
}

// displayPath is the repo path as printed, relative to baseDir with --short
func (r rowItem) displayPath(baseDir string) string {
	if viper.GetBool(fShort) {
		return strings.TrimPrefix(strings.TrimPrefix(r.path, baseDir), "/")
	}
	return r.path
}

func (r rowItem) displayBranch() string {
	if r.isDetached() {
		return "detached @ " + string([]rune(r.branch)[:min(7, len([]rune(r.branch)))])
	}
	return truncate(r.branch, 30)
}

// errorText is the first meaningful line of the error; git's stderr can span
// several and opens with fetch output when the pull fetched before failing
func (r rowItem) errorText() string {
	if r.error == nil {
		return ""
	}
	for line := range strings.Lines(r.error.Error()) {
		if strings.HasPrefix(line, "From ") || strings.HasPrefix(line, " ") || strings.TrimSpace(line) == "" {
			continue
		}
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(r.error.Error())
}

// category is the single group a repo is listed under, most urgent first
type category int

const (
	catError category = iota
	catSkipped
	catPulled
	catOffMain
	catDirty
	catClean
)

type categoryInfo struct {
	title   string
	label   string // status word in plain output
	summary string // count suffix in the summary line
	glyph   string
	style   lipgloss.Style
}

var categories = map[category]categoryInfo{
	catError:   {"Errors", "error", "failed", "✗", red},
	catSkipped: {"Skipped", "skipped", "skipped", "↷", orange},
	catPulled:  {"Pulled", "pulled", "pulled", "↓", green},
	catOffMain: {"Off main", "branch", "off main", "⎇", purple},
	catDirty:   {"Uncommitted", "dirty", "uncommitted", "●", orange},
	catClean:   {"Clean", "clean", "clean", "✓", dim},
}

func (r rowItem) category() category {
	switch {
	case r.error != nil:
		return catError
	case r.skipped != "":
		return catSkipped
	case r.updated:
		return catPulled
	case !r.isMain():
		return catOffMain
	case r.isDirty():
		return catDirty
	default:
		return catClean
	}
}
