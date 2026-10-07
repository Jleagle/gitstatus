package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/viper"
)

func sortRows(rows []rowItem) {
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].path) < strings.ToLower(rows[j].path)
	})
}

// renderResults lists the repos worth reporting, grouped by category unless
// --flat, then a summary line with --summary
func renderResults(rows []rowItem, elapsed time.Duration) string {

	sortRows(rows)

	var shown []rowItem
	pathW, branchW := 0, 0
	for _, r := range rows {
		if r.show() {
			shown = append(shown, r)
			pathW = max(pathW, lipgloss.Width(r.displayPath()))
			branchW = max(branchW, lipgloss.Width(r.displayBranch()))
		}
	}

	var b strings.Builder
	writeRows := func(rows []rowItem) {
		for _, r := range rows {
			info := categories[r.category()]
			branch := dim.Render(r.displayBranch())
			if !r.isMain() {
				branch = purple.Render(r.displayBranch())
			}
			line := info.style.Render(info.glyph) + " " +
				pad(bright.Render(r.displayPath()), pathW+2) +
				pad(branch, branchW+2) +
				formatChanges(r, true)
			b.WriteString(strings.TrimRight(line, " ") + "\n")

			if reason := r.errorText(); reason != "" {
				b.WriteString(dim.Render("  └ ") + red.Render(reason) + "\n")
			} else if r.skipped != "" {
				b.WriteString(dim.Render("  └ "+r.skipped) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if viper.GetBool(fFlat) {
		if len(shown) > 0 {
			writeRows(shown)
		}
	} else {
		groups := map[category][]rowItem{}
		for _, r := range shown {
			groups[r.category()] = append(groups[r.category()], r)
		}
		for c := catError; c <= catClean; c++ {
			if group := groups[c]; len(group) > 0 {
				info := categories[c]
				b.WriteString(info.style.Bold(true).Render(info.title) + " " + dim.Render(fmt.Sprint(len(group))) + "\n")
				writeRows(group)
			}
		}
	}

	if viper.GetBool(fSummary) {
		b.WriteString(summaryLine(rows, elapsed) + "\n")
	}
	if hidden := len(rows) - len(shown); hidden > 0 {
		noun := "repos"
		if hidden == 1 {
			noun = "repo"
		}
		b.WriteString(dim.Render(fmt.Sprintf("  %d clean %s hidden · ", hidden, noun)) + blue.Render("--all") + dim.Render(" to list them") + "\n")
	}

	return b.String()
}

func summaryLine(rows []rowItem, elapsed time.Duration) string {

	counts := map[category]int{}
	for _, r := range rows {
		counts[r.category()]++
	}

	parts := []string{
		green.Render("✓") + " " + bright.Render(fmt.Sprintf("%d repos", len(rows))) +
			dim.Render(fmt.Sprintf(" in %.1fs", elapsed.Seconds())),
	}
	for c := range catClean {
		if n := counts[c]; n > 0 {
			parts = append(parts, categories[c].style.Render(fmt.Sprintf("%d %s", n, categories[c].summary)))
		}
	}
	return strings.Join(parts, dim.Render(" · "))
}

// printPlain writes one uncoloured line per reported repo, status first, for
// piping into grep, sort or a file
func printPlain(w io.Writer, rows []rowItem) {

	sortRows(rows)

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		if !r.show() {
			continue
		}
		label := categories[r.category()].label
		if r.category() == catOffMain && r.isDetached() {
			label = "detached"
		}
		reason := r.errorText()
		if reason == "" {
			reason = r.skipped
		}
		line := strings.Join([]string{label, r.displayPath(), r.displayBranch(), formatChanges(r, false), reason}, "\t")
		_, _ = fmt.Fprintln(tw, line)
	}
	_ = tw.Flush()

	// Every line keeps all cells so tabwriter aligns them as one block; drop the padding left on empty trailing cells
	for line := range strings.Lines(buf.String()) {
		_, _ = fmt.Fprintln(w, strings.TrimRight(line, " \n"))
	}
}
