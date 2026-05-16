package tui

import (
	"testing"

	"github.com/AgusRdz/bonsai/git"
)

func TestParseLogFilter(t *testing.T) {
	cases := []struct {
		input string
		want  git.LogOptions
	}{
		{"author:Jane Doe", git.LogOptions{Author: "Jane Doe"}},
		{"author: Jane Doe", git.LogOptions{Author: "Jane Doe"}},
		{"since:2024-01-01", git.LogOptions{Since: "2024-01-01"}},
		{"after:2024-01-01", git.LogOptions{Since: "2024-01-01"}},
		{"until:2024-12-31", git.LogOptions{Until: "2024-12-31"}},
		{"before:2024-12-31", git.LogOptions{Until: "2024-12-31"}},
		{"fix login crash", git.LogOptions{Grep: "fix login crash"}},
		{"", git.LogOptions{Grep: ""}},
		{"  author: Jane  ", git.LogOptions{Author: "Jane"}},
	}
	for _, c := range cases {
		got := parseLogFilter(c.input)
		if got != c.want {
			t.Errorf("parseLogFilter(%q) = %+v, want %+v", c.input, got, c.want)
		}
	}
}
