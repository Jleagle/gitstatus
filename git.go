package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const gitTimeout = 10 * time.Second

// gitCmd runs git in its own session so a timeout also kills ssh, whose orphan
// would hold the output pipes open; with no tty ssh fails instead of prompting
func gitCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	return cmd
}

// gitDiff counts the new/changed/deleted files in the repo
func gitDiff(repoPath string) (added, modified, deleted int, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	b, err := gitCmd(ctx, "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return 0, 0, 0, err
	}

	for line := range bytes.SplitSeq(bytes.TrimSpace(b), []byte("\n")) {
		if len(line) < 2 {
			continue
		}
		status := string(line[:2])
		if status == "??" {
			added++
			continue
		}
		if strings.ContainsAny(status, "A") {
			added++
		}
		if strings.ContainsAny(status, "D") {
			deleted++
		}
		if strings.ContainsAny(status, "M") || strings.ContainsAny(status, "C") || strings.ContainsAny(status, "U") || strings.ContainsAny(status, "T") {
			modified++
		}
		if strings.ContainsAny(status, "R") {
			added++
			deleted++
		}
	}

	return added, modified, deleted, nil
}

// gitBranch gets the branch name and whether it is detached
func gitBranch(pathx string) (string, bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	b, err := gitCmd(ctx, "-C", pathx, "branch", "--show-current").Output()
	if err != nil {
		return "", false, err
	}

	branch := string(bytes.TrimSpace(b))
	if branch != "" {
		return branch, false, nil
	}

	// Fallback for detached HEAD
	b, _ = gitCmd(ctx, "-C", pathx, "rev-parse", "HEAD").Output()
	return string(bytes.TrimSpace(b)), true, nil
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

	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// pull.rebase=false forces the merge path; with a rebase config git refuses
	// any pull on a dirty tree, even one that fast-forwards cleanly
	b, err := gitCmd(ctx, "-C", row.path, "-c", "pull.rebase=false", "pull", "--ff-only").Output()

	if ctx.Err() != nil {
		return false, "", fmt.Errorf("timed out after %s", gitTimeout)
	}

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
		if strings.TrimSpace(stderr) == "" {
			return false, "", err
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
	return gitCmd(ctx, "-C", repoPath, "rev-parse", "--verify", "-q", "HEAD").Run() == nil
}
