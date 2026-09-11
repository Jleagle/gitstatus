package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
)

// orange has no named helper in fatih/color, unlike green/red
var orange = color.RGB(255, 165, 0)

// gitDiff returns a colored summary of new/changed/deleted files
func gitDiff(repoPath string) (string, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := exec.CommandContext(ctx, "git", "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return "", err
	}

	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return "", nil
	}

	var added, modified, deleted int
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(line) < 2 {
			continue
		}
		status := string(line[:2])
		switch {
		case status == "??", strings.ContainsAny(status, "A"):
			added++
		case strings.ContainsAny(status, "D"):
			deleted++
		default:
			modified++
		}
	}

	var strAdded, strModified, strDeleted string

	if added > 0 {
		if added > 99 {
			added = 99
		}
		strAdded = color.GreenString("+%02d", added)
	} else {
		strAdded = "   "
	}

	if modified > 0 {
		if modified > 99 {
			modified = 99
		}
		strModified = orange.Sprintf("~%02d", modified)
	} else {
		strModified = "   "
	}

	if deleted > 0 {
		if deleted > 99 {
			deleted = 99
		}
		strDeleted = color.RedString("-%02d", deleted)
	} else {
		strDeleted = "   "
	}

	return fmt.Sprintf("%s %s %s", strAdded, strModified, strDeleted), nil
}

// gitBranch gets the branch name
func gitBranch(pathx string) (string, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := exec.CommandContext(ctx, "git", "-C", pathx, "branch", "--show-current").Output()
	if err != nil {
		return "", err
	}

	branch := string(bytes.TrimSpace(b))
	if branch != "" {
		return branch, nil
	}

	// Fallback for detached HEAD
	b, _ = exec.CommandContext(ctx, "git", "-C", pathx, "rev-parse", "HEAD").Output()
	return string(bytes.TrimSpace(b)), nil
}

// Reasons a pull was skipped because git refused to fast-forward safely
const (
	skipLocalChanges = "local changes"
	skipUntracked    = "untracked files"
	skipDiverged     = "diverged"
)

// gitPull fast-forwards the repo if git can do so without touching local
// changes, returning whether files were pulled down or why it was skipped
func gitPull(row rowItem) (updated bool, skipped string, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// pull.rebase=false forces the merge path; with a rebase config git refuses
	// any pull on a dirty tree, even one that fast-forwards cleanly
	b, err := exec.CommandContext(ctx, "git", "-C", row.path, "-c", "pull.rebase=false", "pull", "--ff-only").Output()

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		stderr := string(exitError.Stderr)
		switch {
		case strings.Contains(stderr, "Your local changes to the following files would be overwritten"):
			return false, skipLocalChanges, nil
		case strings.Contains(stderr, "untracked working tree files would be overwritten"):
			return false, skipUntracked, nil
		case strings.Contains(stderr, "Not possible to fast-forward"):
			return false, skipDiverged, nil
		case strings.Contains(stderr, "but no such ref was fetched"):
			if !hasLocalCommits(ctx, row.path) {
				// Cloned from an empty remote, nothing to pull
				return false, "", nil
			}
			//goland:noinspection GoErrorStringFormat
			return false, "", errors.New("Remote branch does not exist")
		}
		return false, "", errors.New(stderr)
	} else if err != nil {
		return false, "", err
	}

	b = bytes.TrimSpace(b)

	if string(b) == "Already up to date." {
		return false, "", nil
	}
	return strings.Contains(string(b), "changed"), "", nil
}

// hasLocalCommits reports whether HEAD points at a commit (false in a clone of an empty repo)
func hasLocalCommits(ctx context.Context, repoPath string) bool {
	return exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "-q", "HEAD").Run() == nil
}
