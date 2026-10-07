package main

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD38D"))
	orange = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2A54A"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F27878"))
	blue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7"))
	purple = lipgloss.NewStyle().Foreground(lipgloss.Color("#C792EA"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C8394"))
	bright = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECEEF2"))
	track  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2A2E37"))
	badge  = lipgloss.NewStyle().Background(lipgloss.Color("#7AA2F7")).Foreground(lipgloss.Color("#0E1014")).Bold(true).Padding(0, 1)
)

// truncate shortens s to at most n runes, marking the cut with an ellipsis
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// pad right-pads a rendered cell to width w, measuring visible width so styled
// strings align
func pad(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// formatChanges renders the non-zero added/modified/deleted counts, e.g. "+2 ~5 -1"
func formatChanges(r rowItem, styled bool) string {
	var parts []string
	add := func(sign string, n int, s lipgloss.Style) {
		if n <= 0 {
			return
		}
		part := sign + strconv.Itoa(n)
		if styled {
			part = s.Render(part)
		}
		parts = append(parts, part)
	}
	add("+", r.added, green)
	add("~", r.modified, orange)
	add("-", r.deleted, red)
	return strings.Join(parts, " ")
}
