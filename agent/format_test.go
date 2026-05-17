package agent

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------

func TestFormatMarkdown_Status(t *testing.T) {
	s := &StatusOut{
		Branch:     "main",
		Upstream:   "origin/main",
		Ahead:      2,
		Behind:     1,
		Staged:     []FileEntry{{Status: "M", Path: "src/auth.go"}},
		Unstaged:   []FileEntry{},
		Conflicts:  []FileEntry{},
		Untracked:  []FileEntry{{Status: "??", Path: "docs/new.md"}},
		StashCount: 1,
	}
	out := FormatMarkdown(s)
	for _, want := range []string{
		"# Status: main",
		"origin/main",
		"↑2 ↓1",
		"## Staged",
		"src/auth.go",
		"## Untracked",
		"docs/new.md",
		"**Stash:**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown status: expected %q in output\n%s", want, out)
		}
	}
}

func TestFormatMarkdown_Log(t *testing.T) {
	entries := []LogEntry{
		{Hash: "abc1234", Subject: "feat: login", Author: "Alice", Date: "2026-05-17"},
	}
	out := FormatMarkdown(entries)
	for _, want := range []string{"# Commits", "abc1234", "feat: login", "Alice"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown log: expected %q in output", want)
		}
	}
}

func TestFormatMarkdown_Diff(t *testing.T) {
	d := &DiffOut{
		Staged:    []FileDiff{{Path: "src/api.go", Status: "M", Additions: 5, Deletions: 2}},
		Unstaged:  []FileDiff{},
		Untracked: []UntrackedEntry{{Path: "scratch.txt"}},
	}
	out := FormatMarkdown(d)
	for _, want := range []string{"# Diff", "## Staged", "src/api.go", "+5 -2", "## Untracked", "scratch.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown diff: expected %q in output", want)
		}
	}
}

func TestFormatMarkdown_Show(t *testing.T) {
	s := &ShowOut{
		Hash:         "abc1234",
		Subject:      "feat: add auth",
		Author:       "Alice",
		Date:         "2026-05-17",
		Additions:    10,
		Deletions:    3,
		FilesChanged: 2,
		Diff:         []FileDiff{{Path: "auth.go", Additions: 10, Deletions: 3}},
	}
	out := FormatMarkdown(s)
	for _, want := range []string{"# `abc1234`", "feat: add auth", "Alice", "+10 -3", "## Files", "auth.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown show: expected %q in output", want)
		}
	}
}

func TestFormatMarkdown_Review(t *testing.T) {
	r := &ReviewOut{
		Base:         "main",
		Head:         "feat/login",
		Lines:        ReviewLines{Added: 15, Removed: 5, TotalChanged: 20},
		FilesChanged: 2,
		CommitsCount: 1,
		Commits:      []LogEntry{{Hash: "abc1234", Subject: "feat: login", Author: "Alice", Date: "2026-05-17"}},
		Diff:         []FileDiff{{Path: "auth.go", Status: "M", Additions: 15, Deletions: 5}},
		Status:       &StatusOut{Branch: "feat/login"},
	}
	out := FormatMarkdown(r)
	for _, want := range []string{
		"# Review: main → feat/login",
		"+15 -5",
		"## Commits",
		"abc1234",
		"## Files Changed",
		"auth.go",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown review: expected %q in output", want)
		}
	}
}

func TestFormatMarkdown_DiffWithHunks(t *testing.T) {
	d := &DiffOut{
		Staged: []FileDiff{{
			Path:      "main.go",
			Status:    "M",
			Additions: 1,
			Deletions: 1,
			Hunks: []HunkOut{
				{Header: "@@ -1,1 +1,1 @@", Lines: []string{"-old", "+new"}},
			},
		}},
		Unstaged:  []FileDiff{},
		Untracked: []UntrackedEntry{},
	}
	out := FormatMarkdown(d)
	for _, want := range []string{"```diff", "@@ -1,1 +1,1 @@", "-old", "+new", "```"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown diff hunks: expected %q in output", want)
		}
	}
}

// ---------------------------------------------------------------------------
// XML
// ---------------------------------------------------------------------------

func TestFormatXML_Status(t *testing.T) {
	s := &StatusOut{
		Branch:    "main",
		Upstream:  "origin/main",
		Staged:    []FileEntry{{Status: "M", Path: "src/auth.go"}},
		Unstaged:  []FileEntry{},
		Conflicts: []FileEntry{},
		Untracked: []FileEntry{},
	}
	out := FormatXML(s)
	for _, want := range []string{
		`<?xml`,
		"<status>",
		"<branch>main</branch>",
		"<upstream>origin/main</upstream>",
		`status="M"`,
		"src/auth.go",
		"</status>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("xml status: expected %q in output\n%s", want, out)
		}
	}
}

func TestFormatXML_Log(t *testing.T) {
	entries := []LogEntry{
		{Hash: "abc1234", Subject: "feat: login", Author: "Alice", Date: "2026-05-17"},
	}
	out := FormatXML(entries)
	for _, want := range []string{"<commits>", "<commit>", "<hash>abc1234</hash>", "<subject>feat: login</subject>"} {
		if !strings.Contains(out, want) {
			t.Errorf("xml log: expected %q in output", want)
		}
	}
}

func TestFormatXML_Diff(t *testing.T) {
	d := &DiffOut{
		Staged:    []FileDiff{{Path: "src/api.go", Status: "M", Additions: 5, Deletions: 2}},
		Unstaged:  []FileDiff{},
		Untracked: []UntrackedEntry{{Path: "scratch.txt"}},
	}
	out := FormatXML(d)
	for _, want := range []string{
		"<diff>",
		"<staged>",
		`path="src/api.go"`,
		`additions="5"`,
		`status="M"`,
		"<untracked>",
		"scratch.txt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("xml diff: expected %q in output", want)
		}
	}
}

func TestFormatXML_XMLEscaping(t *testing.T) {
	s := &StatusOut{
		Branch:    "feat/<branch>",
		Staged:    []FileEntry{{Status: "M", Path: "src/a&b.go"}},
		Unstaged:  []FileEntry{},
		Conflicts: []FileEntry{},
		Untracked: []FileEntry{},
	}
	out := FormatXML(s)
	if strings.Contains(out, "feat/<branch>") {
		t.Error("xml: unescaped < in branch name")
	}
	if strings.Contains(out, "src/a&b.go") {
		t.Error("xml: unescaped & in file path")
	}
	if !strings.Contains(out, "&amp;") {
		t.Error("xml: expected &amp; for ampersand in path")
	}
}

func TestFormatXML_Review(t *testing.T) {
	r := &ReviewOut{
		Base:   "main",
		Head:   "feat/login",
		Lines:  ReviewLines{Added: 10, Removed: 2, TotalChanged: 12},
		Diff:   []FileDiff{{Path: "auth.go", Additions: 10, Deletions: 2}},
		Status: &StatusOut{Branch: "feat/login"},
	}
	out := FormatXML(r)
	for _, want := range []string{
		"<review>",
		"<base>main</base>",
		"<head>feat/login</head>",
		"<added>10</added>",
		`path="auth.go"`,
		"</review>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("xml review: expected %q in output", want)
		}
	}
}
