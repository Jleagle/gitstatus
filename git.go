package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
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
func gitBranch(pathx string) (string, error) {

	data, err := os.ReadFile(path.Join(pathx, ".git/HEAD"))
	if err != nil {
		return "", err
	}

	data = bytes.TrimSpace(data)
	data = bytes.TrimPrefix(data, []byte("ref: refs/heads/"))

	return string(data), nil
}

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
func gitPull(row rowItem) (bool, error) {

	cmd := fmt.Sprintf(`git -C %s pull`, row.path)

	b, err := exec.Command("zsh", "-c", cmd).Output()

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return false, errors.New(string(exitError.Stderr))
	} else if err != nil {
		return false, err
	}

	b = bytes.TrimSpace(b)

	if strings.Contains(string(b), "but no such ref was fetched") {
		//goland:noinspection GoErrorStringFormat
		return false, errors.New("Remote branch does not exist")
	}
	if string(b) == "Already up to date." {
		return false, nil
	}
	return strings.Contains(string(b), "changed"), nil
}
