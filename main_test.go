package main

import (
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

func TestFirstLine(t *testing.T) {
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
			want:  []byte("first"),
		},
		{
			name:  "empty string",
			input: []byte(""),
			want:  []byte(""),
		},
		{
			name:  "line with trailing newline",
			input: []byte("first\n"),
			want:  []byte("first"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstLine(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("firstLine() = %q, want %q", got, tt.want)
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

func TestBoolP(t *testing.T) {
	tests := []struct {
		name string
		val  bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boolP(tt.val)
			if got == nil {
				t.Fatal("boolP() returned nil")
			}
			if *got != tt.val {
				t.Errorf("boolP(%v) = %v, want %v", tt.val, *got, tt.val)
			}
		})
	}
}

func TestStringP(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"empty", ""},
		{"non-empty", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringP(tt.val)
			if got == nil {
				t.Fatal("stringP() returned nil")
			}
			if *got != tt.val {
				t.Errorf("stringP(%q) = %q, want %q", tt.val, *got, tt.val)
			}
		})
	}
}
