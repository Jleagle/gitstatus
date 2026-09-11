package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// gitDiff counts the new/changed/deleted files in the repo
func gitDiff(repoPath string) (added, modified, deleted int, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := exec.CommandContext(ctx, "git", "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return 0, 0, 0, err
	}

	for line := range bytes.SplitSeq(bytes.TrimSpace(b), []byte("\n")) {
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

	return added, modified, deleted, nil
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
