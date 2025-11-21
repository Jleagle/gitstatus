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
