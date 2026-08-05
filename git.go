package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
)

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

	var parts []string
	if added > 0 {
		parts = append(parts, color.GreenString("+%d", added))
	}
	if modified > 0 {
		parts = append(parts, color.RGB(255, 165, 0).Sprintf("~%d", modified))
	}
	if deleted > 0 {
		parts = append(parts, color.RedString("-%d", deleted))
	}

	return strings.Join(parts, " "), nil
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

// gitPull returns if any files were pulled down
func gitPull(row rowItem) (bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := exec.CommandContext(ctx, "git", "-C", row.path, "pull").Output()

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if strings.Contains(string(exitError.Stderr), "but no such ref was fetched") {
			if !hasLocalCommits(ctx, row.path) {
				// Cloned from an empty remote, nothing to pull
				return false, nil
			}
			//goland:noinspection GoErrorStringFormat
			return false, errors.New("Remote branch does not exist")
		}
		return false, errors.New(string(exitError.Stderr))
	} else if err != nil {
		return false, err
	}

	b = bytes.TrimSpace(b)

	if string(b) == "Already up to date." {
		return false, nil
	}
	return strings.Contains(string(b), "changed"), nil
}

// hasLocalCommits reports whether HEAD points at a commit (false in a clone of an empty repo)
func hasLocalCommits(ctx context.Context, repoPath string) bool {
	return exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "-q", "HEAD").Run() == nil
}
