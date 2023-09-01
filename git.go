package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
)

// gitDiff gets the number of modified files
func gitDiff(path string) (string, error) {

	cmd := fmt.Sprintf(`git -C %s diff --stat`, path)

	b, err := exec.Command("zsh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}

	b = bytes.TrimSpace(b)
	b = lastLine(b)

	if len(b) == 0 {
		return "", nil
	}

	str := string(b)

	parts := strings.Split(str, ",")
	str = strings.TrimSuffix(parts[0], " changed")

	return str, nil
}

// gitBranch gets the branch name
func gitBranch(path string) (string, error) {

	cmd := fmt.Sprintf(`git -C %s branch --show-current`, path)

	b, err := exec.Command("zsh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(b)), nil
}

const staleDays = 180 // Days

// gitLog gets the time of the latest commit
func gitLog(path string) (*time.Time, error) {

	cmd := fmt.Sprintf(`git -C %s log -1 --format="%%at"`, path)

	b, err := exec.Command("zsh", "-c", cmd).Output()
	if err != nil {
		return nil, err
	}

	str := string(bytes.TrimSpace(b))

	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, err
	}

	t := time.Unix(i, 0)

	return &t, nil
}

// gitPull returns if any files were pulled down
func gitPull(row rowItem, bar *pb.ProgressBar) (bool, error) {

	cmd := fmt.Sprintf(`git -C %s pull`, row.path)

	b, err := exec.Command("zsh", "-c", cmd).Output()
	if err != nil {
		return false, err
	}

	b = bytes.TrimSpace(b)
	b = lastLine(b)

	if string(b) == "Already up to date." {
		return false, nil
	}
	return strings.Contains(string(b), "changed"), nil
}
