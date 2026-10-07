package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCategory(t *testing.T) {
	tests := []struct {
		name string
		row  rowItem
		want category
	}{
		{"error beats everything", rowItem{branch: "feat", modified: 1, updated: true, error: errors.New("x")}, catError},
		{"skipped beats pulled", rowItem{branch: "main", skipped: skipDiverged}, catSkipped},
		{"pulled off main", rowItem{branch: "feat", updated: true}, catPulled},
		{"dirty off main", rowItem{branch: "feat", added: 1}, catOffMain},
		{"dirty main", rowItem{branch: "main", added: 1}, catDirty},
		{"clean main", rowItem{branch: "main"}, catClean},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.row.category(); got != tt.want {
				t.Errorf("rowItem.category() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatChanges(t *testing.T) {
	tests := []struct {
		row  rowItem
		want string
	}{
		{rowItem{}, ""},
		{rowItem{added: 2, modified: 5, deleted: 1}, "+2 ~5 -1"},
		{rowItem{modified: 3}, "~3"},
		{rowItem{added: 120, deleted: 4}, "+120 -4"},
	}

	for _, tt := range tests {
		if got := formatChanges(tt.row, false); got != tt.want {
			t.Errorf("formatChanges(%+v) = %q, want %q", tt.row, got, tt.want)
		}
	}
}

func TestDisplayBranch(t *testing.T) {
	tests := []struct {
		row  rowItem
		want string
	}{
		{rowItem{branch: "main"}, "main"},
		{rowItem{branch: "4d4e1453b2c9", detached: true}, "detached @ 4d4e145"},
		{rowItem{branch: "feat/" + strings.Repeat("é", 40)}, "feat/" + strings.Repeat("é", 24) + "…"},
	}

	for _, tt := range tests {
		if got := tt.row.displayBranch(); got != tt.want {
			t.Errorf("displayBranch(%q) = %q, want %q", tt.row.branch, got, tt.want)
		}
	}
}

func TestErrorText(t *testing.T) {
	tests := []struct {
		err  string
		want string
	}{
		{"fatal: bad thing\nhint: more detail\n", "fatal: bad thing"},
		{"From /code/remote\n   1253f40..9a1c2e7  main -> origin/main\nYou are not currently on a branch.\n", "You are not currently on a branch."},
		{"   only indented\n", "only indented"},
	}

	for _, tt := range tests {
		if got := (rowItem{error: errors.New(tt.err)}).errorText(); got != tt.want {
			t.Errorf("errorText(%q) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestPrintPlain(t *testing.T) {

	t.Cleanup(func() { viper.Reset() })
	viper.Set(fShort, true)

	rows := []rowItem{
		{path: "/code/b/clean", branch: "main"},
		{path: "/code/a/dirty", branch: "main", added: 1, modified: 3},
		{path: "/code/c/broken", branch: "main", error: errors.New("Remote branch does not exist")},
		{path: "/code/d/head", branch: "4d4e145", detached: true},
	}

	var b strings.Builder
	printPlain(&b, rows, "/code")

	want := "" +
		"dirty     a/dirty   main                +1 ~3\n" +
		"error     c/broken  main                       Remote branch does not exist\n" +
		"detached  d/head    detached @ 4d4e145\n"
	if b.String() != want {
		t.Errorf("printPlain() =\n%s\nwant\n%s", b.String(), want)
	}
}
