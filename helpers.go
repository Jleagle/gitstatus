package main

import (
	"github.com/fatih/color"
)

var (
	green = color.New(color.FgGreen)
	// orange has no named helper in fatih/color, unlike green/red
	orange = color.RGB(255, 165, 0)
	red    = color.New(color.FgRed)
)

// formatCount renders one changes-column segment, e.g. "+07"; counts are
// clamped to two digits and zero renders as blanks to keep the column aligned
func formatCount(sign string, count int, c *color.Color) string {
	if count <= 0 {
		return "   "
	}
	if count > 99 {
		count = 99
	}
	return c.Sprintf("%s%02d", sign, count)
}
