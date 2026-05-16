package tui

import (
	"strings"
	"testing"

	"github.com/AgusRdz/bonsai/config"
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

func TestFileCode(t *testing.T) {
	// Code[0]=staged char, Code[1]=unstaged char
	f := git.FileEntry{Code: "M ", Path: "a.go"}
	// catChanged -> UnstagedCode (index 1 = ' ')
	if got := fileCode(f, catChanged); got != " " {
		t.Errorf("fileCode(catChanged) = %q, want ' '", got)
	}
	// catStaged -> StagedCode (index 0 = 'M')
	if got := fileCode(f, catStaged); got != "M" {
		t.Errorf("fileCode(catStaged) = %q, want 'M'", got)
	}
	// catConflict -> StagedCode
	fu := git.FileEntry{Code: "UU", Path: "b.go"}
	if got := fileCode(fu, catConflict); got != "U" {
		t.Errorf("fileCode(catConflict) = %q, want 'U'", got)
	}
}

func TestCommitDetailLineCount(t *testing.T) {
	// nil -> 0
	if n := commitDetailLineCount(nil); n != 0 {
		t.Errorf("nil = %d, want 0", n)
	}
	// no body, no stat: base 6 + (len(stat)+1)=1 = 7
	if n := commitDetailLineCount(&git.CommitDetail{}); n != 7 {
		t.Errorf("empty = %d, want 7", n)
	}
	// body "line1\nline2" (1 newline): 6 + (1+2) + 1 = 10
	d2 := &git.CommitDetail{Body: "line1\nline2"}
	if n := commitDetailLineCount(d2); n != 10 {
		t.Errorf("body = %d, want 10", n)
	}
	// 3 stat lines, no body: 6 + 0 + (3+1) = 10
	d3 := &git.CommitDetail{Stat: []string{"a.go | 2 +-", "b.go | 1 +", "c.go | 3 ---"}}
	if n := commitDetailLineCount(d3); n != 10 {
		t.Errorf("stat = %d, want 10", n)
	}
}

func TestRenderDiffLine(t *testing.T) {
	cases := []string{
		"@@ -1,2 +1,3 @@",
		"+added line",
		"-removed line",
		" context line",
	}
	for _, input := range cases {
		got := renderDiffLine(input)
		if !strings.Contains(got, input) {
			t.Errorf("renderDiffLine(%q): output does not contain input", input)
		}
	}
}

func TestRenderStatLine(t *testing.T) {
	// File line with | separator - filename part must be preserved
	got := renderStatLine("main.go | 5 ++---")
	if !strings.Contains(got, "main.go |") {
		t.Errorf("renderStatLine file line missing filename: %q", got)
	}
	// Summary line without | - content must be preserved
	got2 := renderStatLine("3 files changed, 5 insertions(+), 2 deletions(-)")
	if !strings.Contains(got2, "3 files changed") {
		t.Errorf("renderStatLine summary missing content: %q", got2)
	}
}

func TestFileActionHintEmpty(t *testing.T) {
	m := model{cfg: &config.Config{}}
	if got := fileActionHint(m); got != "" {
		t.Errorf("fileActionHint(no files) = %q, want empty", got)
	}
}

func TestFileActionHintCategories(t *testing.T) {
	cases := []struct {
		cat      int
		wantKeys []string
	}{
		{catUntracked, []string{"space", "delete from disk", "diff"}},
		{catChanged, []string{"space", "hunks", "discard"}},
		{catStaged, []string{"space", "untrack", "commit"}},
		{catConflict, []string{"take ours", "take theirs"}},
	}
	for _, c := range cases {
		m := model{
			cfg:    &config.Config{},
			files:  []fileItem{{entry: git.FileEntry{Code: "??", Path: "a.go"}, category: c.cat}},
			cursor: 0,
		}
		got := fileActionHint(m)
		for _, key := range c.wantKeys {
			if !strings.Contains(got, key) {
				t.Errorf("fileActionHint(cat=%d) = %q, missing %q", c.cat, got, key)
			}
		}
	}
}

func TestContextTipNilStatus(t *testing.T) {
	m := model{cfg: &config.Config{}}
	if got := contextTip(m); got != "" {
		t.Errorf("contextTip(nil status) = %q, want empty", got)
	}
}

func TestContextTipBehind(t *testing.T) {
	m := model{
		cfg:    &config.Config{},
		status: &git.Status{Behind: 3},
	}
	got := contextTip(m)
	if !strings.Contains(got, "3") || !strings.Contains(got, "pull") {
		t.Errorf("contextTip(behind=3) = %q, want pull tip", got)
	}
}

func TestContextTipDiverged(t *testing.T) {
	m := model{
		cfg:    &config.Config{},
		status: &git.Status{Ahead: 30, Behind: 84},
	}
	got := contextTip(m)
	if !strings.Contains(got, "30") || !strings.Contains(got, "84") {
		t.Errorf("contextTip(diverged) = %q, want counts in tip", got)
	}
	if !strings.Contains(got, "diverged") {
		t.Errorf("contextTip(diverged) = %q, want 'diverged' in tip", got)
	}
}

func TestContextTipChangedUnstaged(t *testing.T) {
	m := model{
		cfg:    &config.Config{},
		status: &git.Status{Changed: []git.FileEntry{{Code: " M", Path: "a.go"}}},
	}
	got := contextTip(m)
	if !strings.Contains(got, "stage") {
		t.Errorf("contextTip(changed only) = %q, want stage tip", got)
	}
}

func TestContextTipStagedAndChanged(t *testing.T) {
	m := model{
		cfg: &config.Config{},
		status: &git.Status{
			Staged:  []git.FileEntry{{Code: "M ", Path: "a.go"}},
			Changed: []git.FileEntry{{Code: " M", Path: "b.go"}},
		},
	}
	got := contextTip(m)
	if !strings.Contains(got, "commit") {
		t.Errorf("contextTip(staged+changed) = %q, want commit tip", got)
	}
}

func TestContextTipStagedOnly(t *testing.T) {
	m := model{
		cfg:    &config.Config{},
		status: &git.Status{Staged: []git.FileEntry{{Code: "M ", Path: "a.go"}}},
	}
	got := contextTip(m)
	if !strings.Contains(got, "commit") {
		t.Errorf("contextTip(staged only) = %q, want commit tip", got)
	}
}

func TestContextTipAheadDefault(t *testing.T) {
	m := model{
		cfg:    &config.Config{Flow: config.FlowConfig{Type: "trunk"}},
		status: &git.Status{Ahead: 2},
	}
	got := contextTip(m)
	if !strings.Contains(got, "2") {
		t.Errorf("contextTip(ahead=2, trunk) = %q, want ahead tip", got)
	}
}

func TestContextTipClean(t *testing.T) {
	m := model{
		cfg:    &config.Config{},
		status: &git.Status{},
	}
	got := contextTip(m)
	if !strings.Contains(got, "clean") {
		t.Errorf("contextTip(clean) = %q, want clean tip", got)
	}
}
