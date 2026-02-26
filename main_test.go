package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestFilterReposByFilterFlag(t *testing.T) {

	repos := []repoItem{
		{path: "/work/foo"},
		{path: "/work/bar"},
		{path: "/work/bazqux"},
	}

	type tc struct {
		name     string
		filter   string
		expected []string
	}

	cases := []tc{
		{
			name:     "empty filter returns all",
			filter:   "",
			expected: []string{"/work/foo", "/work/bar", "/work/bazqux"},
		},
		{
			name:     "simple include (foo)",
			filter:   "foo",
			expected: []string{"/work/foo"},
		},
		{
			name:     "simple include (baz)",
			filter:   "baz",
			expected: []string{"/work/bazqux"},
		},
		{
			name:     "simple exclude (!bar)",
			filter:   "!bar",
			expected: []string{"/work/foo", "/work/bazqux"},
		},
		{
			name:     "multiple excludes (!bar, !baz)",
			filter:   "!bar, !baz",
			expected: []string{"/work/foo"},
		},
		{
			name:     "multiple includes (bar, baz)",
			filter:   "bar, baz",
			expected: []string{"/work/bar", "/work/bazqux"},
		},
		{
			name:     "include with exclude (baz, !qux)",
			filter:   "baz, !qux",
			expected: []string{},
		},
		{
			name:     "case insensitive matching (FOO)",
			filter:   "FOO",
			expected: []string{"/work/foo"},
		},
		{
			name:     "whitespace handling ( bar , baz )",
			filter:   " bar , baz ",
			expected: []string{"/work/bar", "/work/bazqux"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			viper.Set(fFilter, c.filter)

			filtered := filterReposByFilterFlag(repos)

			// Convert to string slice
			got := make([]string, 0, len(filtered))
			for _, r := range filtered {
				got = append(got, r.path)
			}

			if !reflect.DeepEqual(got, c.expected) {
				t.Fatalf("filter=%q\nexpected: %v\n     got: %v", c.filter, c.expected, got)
			}
		})
	}
}

func TestLastLine(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "single line",
			input: []byte("hello"),
			want:  []byte("hello"),
		},
		{
			name:  "multiple lines",
			input: []byte("first\nsecond\nthird"),
			want:  []byte("third"),
		},
		{
			name:  "empty string",
			input: []byte(""),
			want:  []byte(""),
		},
		{
			name:  "line with trailing newline",
			input: []byte("first\nsecond\n"),
			want:  []byte("second"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastLine(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lastLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsMain(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{
			name:   "master branch",
			branch: "master",
			want:   true,
		},
		{
			name:   "main branch",
			branch: "main",
			want:   true,
		},
		{
			name:   "feature branch",
			branch: "feature/new-thing",
			want:   false,
		},
		{
			name:   "empty branch",
			branch: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rowItem{branch: tt.branch}
			if got := r.isMain(); got != tt.want {
				t.Errorf("rowItem.isMain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDirty(t *testing.T) {
	tests := []struct {
		name         string
		changedFiles string
		want         bool
	}{
		{
			name:         "no changes",
			changedFiles: "",
			want:         false,
		},
		{
			name:         "has changes",
			changedFiles: "3 files",
			want:         true,
		},
		{
			name:         "single file changed",
			changedFiles: "1 file",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rowItem{changedFiles: tt.changedFiles}
			if got := r.isDirty(); got != tt.want {
				t.Errorf("rowItem.isDirty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShow(t *testing.T) {
	tests := []struct {
		name string
		row  rowItem
		all  bool
		want bool
	}{
		{
			name: "clean main branch, all=false",
			row: rowItem{
				branch:       "main",
				changedFiles: "",
				updated:      false,
				error:        nil,
			},
			all:  false,
			want: false,
		},
		{
			name: "clean main branch, all=true",
			row: rowItem{
				branch:       "main",
				changedFiles: "",
				updated:      false,
				error:        nil,
			},
			all:  true,
			want: true,
		},
		{
			name: "dirty main branch",
			row: rowItem{
				branch:       "main",
				changedFiles: "3 files",
				updated:      false,
				error:        nil,
			},
			all:  false,
			want: true,
		},
		{
			name: "clean feature branch",
			row: rowItem{
				branch:       "feature/test",
				changedFiles: "",
				updated:      false,
				error:        nil,
			},
			all:  false,
			want: true,
		},
		{
			name: "clean main branch with updates",
			row: rowItem{
				branch:       "main",
				changedFiles: "",
				updated:      true,
				error:        nil,
			},
			all:  false,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(fAll, tt.all)
			if got := tt.row.show(); got != tt.want {
				t.Errorf("rowItem.show() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanAllDirs(t *testing.T) {
	// Create a temp directory structure with nested git repos
	tmpDir := t.TempDir()

	// Create a git repo at depth 1
	repo1 := filepath.Join(tmpDir, "repo1")
	os.MkdirAll(filepath.Join(repo1, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo1, ".git", "index"), []byte("fake index"), 0o644)

	// Create a non-repo dir with a nested git repo at depth 2
	org := filepath.Join(tmpDir, "org")
	repo2 := filepath.Join(org, "repo2")
	os.MkdirAll(filepath.Join(repo2, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo2, ".git", "index"), []byte("fake index data"), 0o644)

	// Create a deeply nested repo at depth 3 (should be excluded at maxdepth=2)
	deep := filepath.Join(org, "sub", "repo3")
	os.MkdirAll(filepath.Join(deep, ".git"), 0o755)
	os.WriteFile(filepath.Join(deep, ".git", "index"), []byte("x"), 0o644)

	viper.Set(fMaxdepth, 2)

	repos := scanAllDirs(tmpDir, 1)

	paths := make(map[string]bool)
	for _, r := range repos {
		paths[r.path] = true
	}

	if !paths[repo1] {
		t.Errorf("expected repo1 at %s to be found", repo1)
	}
	if !paths[repo2] {
		t.Errorf("expected repo2 at %s to be found", repo2)
	}
	if paths[deep] {
		t.Errorf("expected repo3 at %s to be excluded (depth > maxdepth)", deep)
	}
	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
}

func TestScanAllDirsInvalidDir(t *testing.T) {
	repos := scanAllDirs("/nonexistent/path/that/does/not/exist", 1)
	if len(repos) != 0 {
		t.Errorf("expected 0 repos for invalid dir, got %d", len(repos))
	}
}

// initTestRepo creates a minimal git repo in a temp directory and returns the path.
func initTestRepo(t *testing.T) string {

	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create an initial commit so HEAD exists
	testFile := filepath.Join(dir, "file.txt")
	os.WriteFile(testFile, []byte("hello"), 0o644)

	cmds = [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	return dir
}

func TestGitDiff(t *testing.T) {

	dir := initTestRepo(t)

	// Clean repo should have no diff
	diff, err := gitDiff(dir)
	if err != nil {
		t.Fatalf("gitDiff on clean repo: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff on clean repo, got %q", diff)
	}

	// Modify a file
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644)

	diff, err = gitDiff(dir)
	if err != nil {
		t.Fatalf("gitDiff on dirty repo: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff on dirty repo")
	}
}

func TestGitBranch(t *testing.T) {

	dir := initTestRepo(t)

	branch, err := gitBranch(dir)
	if err != nil {
		t.Fatalf("gitBranch: %v", err)
	}

	// Default branch may be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got %q", branch)
	}
}

func TestGitBranchDetachedHead(t *testing.T) {

	dir := initTestRepo(t)

	// Detach HEAD
	cmd := exec.Command("git", "checkout", "--detach")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach: %v\n%s", err, out)
	}

	branch, err := gitBranch(dir)
	if err != nil {
		t.Fatalf("gitBranch on detached HEAD: %v", err)
	}

	// In detached HEAD, .git/HEAD contains a commit hash (40 hex chars), not a branch ref
	if len(branch) != 40 {
		t.Errorf("expected 40-char commit hash for detached HEAD, got %q (len=%d)", branch, len(branch))
	}
}
