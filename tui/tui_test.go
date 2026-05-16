package tui

import (
	"testing"

	"github.com/AgusRdz/bonsai/git"
	"github.com/AgusRdz/bonsai/usage"
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

func TestBuildEduMgrKeysSortOrder(t *testing.T) {
	u := &usage.Data{
		Counts:     map[string]int{"commit": 20, "push": 15, "pull": 15, "add": 5},
		Suppressed: map[string]bool{"rebase": true},
		Prompted:   map[string]bool{},
	}
	keys := buildEduMgrKeys(u)

	if len(keys) != 5 {
		t.Fatalf("key count = %d, want 5", len(keys))
	}
	// commit (20) must be first
	if keys[0] != "commit" {
		t.Errorf("keys[0] = %q, want commit", keys[0])
	}
	// pull and push both have 15 - alphabetical tie-break
	if keys[1] != "pull" {
		t.Errorf("keys[1] = %q, want pull (alphabetical before push)", keys[1])
	}
	if keys[2] != "push" {
		t.Errorf("keys[2] = %q, want push", keys[2])
	}
	// add (5) before rebase (0, suppressed only)
	if keys[3] != "add" {
		t.Errorf("keys[3] = %q, want add", keys[3])
	}
	if keys[4] != "rebase" {
		t.Errorf("keys[4] = %q, want rebase", keys[4])
	}
}

func TestBuildEduMgrKeysEmpty(t *testing.T) {
	u := &usage.Data{
		Counts:     map[string]int{},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	if keys := buildEduMgrKeys(u); len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}
}

func TestBuildEduMgrKeysSuppressedWithoutCount(t *testing.T) {
	u := &usage.Data{
		Counts:     map[string]int{"commit": 5},
		Suppressed: map[string]bool{"rebase": true, "commit": true},
		Prompted:   map[string]bool{},
	}
	keys := buildEduMgrKeys(u)
	// commit appears in both Counts and Suppressed - should deduplicate
	seen := map[string]int{}
	for _, k := range keys {
		seen[k]++
	}
	if seen["commit"] != 1 {
		t.Errorf("commit appeared %d times, want 1", seen["commit"])
	}
	if seen["rebase"] != 1 {
		t.Errorf("rebase appeared %d times, want 1", seen["rebase"])
	}
}

func TestFormatConfigEntries(t *testing.T) {
	entries := []git.ConfigEntry{
		{Key: "user.name", Value: "Jane"},
		{Key: "user.email", Value: "jane@example.com"},
		{Key: "core.editor", Value: "vim"},
		{Key: "pull.rebase", Value: "true"},
	}
	lines := formatConfigEntries(entries)

	// user.name and user.email - same group, no blank line between
	// blank line between user.* and core.*
	// blank line between core.* and pull.*
	if len(lines) < 6 {
		t.Fatalf("line count = %d, want at least 6 (4 entries + 2 blank separators)", len(lines))
	}
	// first line is user.name
	if lines[0] != "user.name  =  Jane" {
		t.Errorf("lines[0] = %q, want user.name  =  Jane", lines[0])
	}
	// blank line separates user from core
	found := false
	for _, l := range lines {
		if l == "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one blank separator line between groups")
	}
}

func TestFormatConfigEntriesEmpty(t *testing.T) {
	if lines := formatConfigEntries(nil); len(lines) != 0 {
		t.Errorf("expected empty, got %v", lines)
	}
}

func TestFormatConfigEntriesSingleGroup(t *testing.T) {
	entries := []git.ConfigEntry{
		{Key: "user.name", Value: "Alice"},
		{Key: "user.email", Value: "alice@example.com"},
	}
	lines := formatConfigEntries(entries)
	// No blank lines when all entries are in the same group
	for _, l := range lines {
		if l == "" {
			t.Error("unexpected blank line within single group")
		}
	}
	if len(lines) != 2 {
		t.Errorf("line count = %d, want 2", len(lines))
	}
}

func TestBuildFileList(t *testing.T) {
	s := &git.Status{
		Conflicts: []git.FileEntry{{Code: "UU", Path: "conflict.go"}},
		Staged:    []git.FileEntry{{Code: "M ", Path: "staged.go"}},
		Changed:   []git.FileEntry{{Code: " M", Path: "changed.go"}},
		Untracked: []git.FileEntry{{Code: "??", Path: "new.go"}},
	}
	items := buildFileList(s)

	if len(items) != 4 {
		t.Fatalf("item count = %d, want 4", len(items))
	}
	// Order: conflicts → staged → changed → untracked
	if items[0].entry.Path != "conflict.go" || items[0].category != catConflict {
		t.Errorf("items[0] = {%q, %d}, want {conflict.go, catConflict}", items[0].entry.Path, items[0].category)
	}
	if items[1].entry.Path != "staged.go" || items[1].category != catStaged {
		t.Errorf("items[1] = {%q, %d}, want {staged.go, catStaged}", items[1].entry.Path, items[1].category)
	}
	if items[2].entry.Path != "changed.go" || items[2].category != catChanged {
		t.Errorf("items[2] = {%q, %d}, want {changed.go, catChanged}", items[2].entry.Path, items[2].category)
	}
	if items[3].entry.Path != "new.go" || items[3].category != catUntracked {
		t.Errorf("items[3] = {%q, %d}, want {new.go, catUntracked}", items[3].entry.Path, items[3].category)
	}
}

func TestBuildFileListEmpty(t *testing.T) {
	if items := buildFileList(&git.Status{}); len(items) != 0 {
		t.Errorf("expected empty, got %v", items)
	}
}

func TestBuildFileListConflictsFirst(t *testing.T) {
	s := &git.Status{
		Staged:    []git.FileEntry{{Code: "M ", Path: "a.go"}},
		Conflicts: []git.FileEntry{{Code: "UU", Path: "z.go"}},
	}
	items := buildFileList(s)
	if items[0].category != catConflict {
		t.Error("conflicts must come before staged files")
	}
}
