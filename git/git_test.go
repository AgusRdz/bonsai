package git

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	porcelain := "" +
		"M  staged-modified.go\n" +
		"A  staged-added.go\n" +
		" M unstaged-modified.go\n" +
		"MM both.go\n" +
		"?? untracked.go\n" +
		"D  staged-deleted.go\n"

	s := parseStatus("main", porcelain)

	if s.Branch != "main" {
		t.Errorf("branch = %q, want main", s.Branch)
	}

	wantStaged := []struct{ code, path string }{
		{"M ", "staged-modified.go"},
		{"A ", "staged-added.go"},
		{"MM", "both.go"},
		{"D ", "staged-deleted.go"},
	}
	if len(s.Staged) != len(wantStaged) {
		t.Fatalf("staged count = %d, want %d: %v", len(s.Staged), len(wantStaged), s.Staged)
	}
	for i, w := range wantStaged {
		if s.Staged[i].Code != w.code || s.Staged[i].Path != w.path {
			t.Errorf("staged[%d] = {%q,%q}, want {%q,%q}", i,
				s.Staged[i].Code, s.Staged[i].Path, w.code, w.path)
		}
	}

	wantChanged := []struct{ code, path string }{
		{" M", "unstaged-modified.go"},
		{"MM", "both.go"},
	}
	if len(s.Changed) != len(wantChanged) {
		t.Fatalf("changed count = %d, want %d: %v", len(s.Changed), len(wantChanged), s.Changed)
	}
	for i, w := range wantChanged {
		if s.Changed[i].Code != w.code || s.Changed[i].Path != w.path {
			t.Errorf("changed[%d] = {%q,%q}, want {%q,%q}", i,
				s.Changed[i].Code, s.Changed[i].Path, w.code, w.path)
		}
	}

	if len(s.Untracked) != 1 || s.Untracked[0].Path != "untracked.go" {
		t.Errorf("untracked = %v, want [{?? untracked.go}]", s.Untracked)
	}
}

func TestParseStatusEmpty(t *testing.T) {
	s := parseStatus("main", "")
	if len(s.Staged) != 0 || len(s.Changed) != 0 || len(s.Untracked) != 0 {
		t.Errorf("expected clean status, got %+v", s)
	}
}

func TestParseBranches(t *testing.T) {
	// Format: %(refname:short)\t%(HEAD)\t%(upstream:short)
	output := "main\t \t\nfeat/login\t*\torigin/feat/login\nfix/bug-123\t \t\n"
	branches := parseBranches(output)

	if len(branches) != 3 {
		t.Fatalf("branch count = %d, want 3", len(branches))
	}

	cases := []struct {
		name     string
		current  bool
		upstream string
	}{
		{"main", false, ""},
		{"feat/login", true, "origin/feat/login"},
		{"fix/bug-123", false, ""},
	}
	for i, c := range cases {
		if branches[i].Name != c.name {
			t.Errorf("branches[%d].Name = %q, want %q", i, branches[i].Name, c.name)
		}
		if branches[i].Current != c.current {
			t.Errorf("branches[%d].Current = %v, want %v", i, branches[i].Current, c.current)
		}
		if branches[i].Upstream != c.upstream {
			t.Errorf("branches[%d].Upstream = %q, want %q", i, branches[i].Upstream, c.upstream)
		}
	}
}

func TestParseBranchesEmpty(t *testing.T) {
	if branches := parseBranches(""); len(branches) != 0 {
		t.Errorf("expected empty, got %v", branches)
	}
}

func TestParseBranchLine(t *testing.T) {
	cases := []struct {
		input  string
		branch string
		ahead  int
		behind int
	}{
		{"main...origin/main [ahead 2, behind 1]", "main", 2, 1},
		{"main...origin/main [ahead 3]", "main", 3, 0},
		{"main...origin/main [behind 5]", "main", 0, 5},
		{"main...origin/main", "main", 0, 0},
		{"main", "main", 0, 0},
		{"No commits yet on feat/new", "feat/new", 0, 0},
		{"HEAD (no branch)", "HEAD", 0, 0},
	}
	for _, c := range cases {
		branch, ahead, behind := parseBranchLine(c.input)
		if branch != c.branch || ahead != c.ahead || behind != c.behind {
			t.Errorf("parseBranchLine(%q) = (%q, %d, %d), want (%q, %d, %d)",
				c.input, branch, ahead, behind, c.branch, c.ahead, c.behind)
		}
	}
}

func TestFileEntryHelpers(t *testing.T) {
	f := FileEntry{Code: "MM", Path: "file.go"}
	if f.StagedCode() != 'M' {
		t.Errorf("StagedCode = %q, want M", f.StagedCode())
	}
	if f.UnstagedCode() != 'M' {
		t.Errorf("UnstagedCode = %q, want M", f.UnstagedCode())
	}

	untracked := FileEntry{Code: "??", Path: "new.go"}
	if untracked.StagedCode() != '?' {
		t.Errorf("StagedCode = %q, want ?", untracked.StagedCode())
	}
}

func TestConflictDesc(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"UU", "both modified"},
		{"AA", "both added"},
		{"DD", "both deleted"},
		{"AU", "added by us"},
		{"UA", "added by them"},
		{"DU", "deleted by us"},
		{"UD", "deleted by them"},
		{"XY", "conflict"},
		{"", "conflict"},
	}
	for _, c := range cases {
		if got := ConflictDesc(c.code); got != c.want {
			t.Errorf("ConflictDesc(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestParseWorktrees(t *testing.T) {
	output := "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/project-hotfix\nHEAD def456\nbranch refs/heads/hotfix/fix\n\nworktree /home/user/project-detached\nHEAD ghi789\ndetached\n"
	entries := parseWorktrees(output)

	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}

	cases := []struct {
		path    string
		branch  string
		current bool
	}{
		{"/home/user/project", "main", true},
		{"/home/user/project-hotfix", "hotfix/fix", false},
		{"/home/user/project-detached", "(detached)", false},
	}
	for i, c := range cases {
		if entries[i].Path != c.path {
			t.Errorf("entries[%d].Path = %q, want %q", i, entries[i].Path, c.path)
		}
		if entries[i].Branch != c.branch {
			t.Errorf("entries[%d].Branch = %q, want %q", i, entries[i].Branch, c.branch)
		}
		if entries[i].Current != c.current {
			t.Errorf("entries[%d].Current = %v, want %v", i, entries[i].Current, c.current)
		}
	}
}

func TestParseWorktreesEmpty(t *testing.T) {
	if entries := parseWorktrees(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseStashList(t *testing.T) {
	output := "stash@{0}: On main: WIP on login flow\nstash@{1}: On feat/x: half-done refactor\n"
	entries := parseStashList(output)

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	cases := []struct {
		ref  string
		desc string
	}{
		{"stash@{0}", "On main: WIP on login flow"},
		{"stash@{1}", "On feat/x: half-done refactor"},
	}
	for i, c := range cases {
		if entries[i].Ref != c.ref {
			t.Errorf("entries[%d].Ref = %q, want %q", i, entries[i].Ref, c.ref)
		}
		if entries[i].Description != c.desc {
			t.Errorf("entries[%d].Description = %q, want %q", i, entries[i].Description, c.desc)
		}
	}
}

func TestParseStashListEmpty(t *testing.T) {
	if entries := parseStashList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseConfigList(t *testing.T) {
	output := "user.name=Jane Doe\nuser.email=jane@example.com\ncore.editor\npull.rebase=true\n"
	entries := parseConfigList(output)

	if len(entries) != 4 {
		t.Fatalf("entry count = %d, want 4", len(entries))
	}
	cases := []struct {
		key   string
		value string
	}{
		{"user.name", "Jane Doe"},
		{"user.email", "jane@example.com"},
		{"core.editor", ""},
		{"pull.rebase", "true"},
	}
	for i, c := range cases {
		if entries[i].Key != c.key {
			t.Errorf("entries[%d].Key = %q, want %q", i, entries[i].Key, c.key)
		}
		if entries[i].Value != c.value {
			t.Errorf("entries[%d].Value = %q, want %q", i, entries[i].Value, c.value)
		}
	}
}

func TestParseConfigListEmpty(t *testing.T) {
	if entries := parseConfigList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseReflogOutput(t *testing.T) {
	output := "abc1234\x1eHEAD@{0}\x1ecommit: add login\nabc5678\x1eHEAD@{1}\x1echeckout: moving from main to feat/x\nabc9012\x1eHEAD@{2}\x1ereset: moving to HEAD~1\n"
	entries := parseReflogOutput(output)

	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	cases := []struct {
		hash    string
		ref     string
		action  string
		subject string
	}{
		{"abc1234", "HEAD@{0}", "commit", "commit: add login"},
		{"abc5678", "HEAD@{1}", "checkout", "checkout: moving from main to feat/x"},
		{"abc9012", "HEAD@{2}", "reset", "reset: moving to HEAD~1"},
	}
	for i, c := range cases {
		if entries[i].Hash != c.hash {
			t.Errorf("entries[%d].Hash = %q, want %q", i, entries[i].Hash, c.hash)
		}
		if entries[i].Ref != c.ref {
			t.Errorf("entries[%d].Ref = %q, want %q", i, entries[i].Ref, c.ref)
		}
		if entries[i].Action != c.action {
			t.Errorf("entries[%d].Action = %q, want %q", i, entries[i].Action, c.action)
		}
		if entries[i].Subject != c.subject {
			t.Errorf("entries[%d].Subject = %q, want %q", i, entries[i].Subject, c.subject)
		}
	}
}

func TestParseReflogOutputEmpty(t *testing.T) {
	if entries := parseReflogOutput(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseReflogOutputSkipsMalformed(t *testing.T) {
	output := "nocolons\nabc1234\x1eHEAD@{0}\x1ecommit: ok\n"
	entries := parseReflogOutput(output)
	if len(entries) != 1 {
		t.Errorf("entry count = %d, want 1", len(entries))
	}
}

func TestParseRemoteList(t *testing.T) {
	output := "origin\tgit@github.com:org/repo.git (fetch)\norigin\tgit@github.com:org/repo.git (push)\nupstream\thttps://github.com/upstream/repo.git (fetch)\nupstream\thttps://github.com/upstream/repo.git (push)\n"
	entries := parseRemoteList(output)

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	cases := []struct {
		name     string
		fetchURL string
		pushURL  string
	}{
		{"origin", "git@github.com:org/repo.git", "git@github.com:org/repo.git"},
		{"upstream", "https://github.com/upstream/repo.git", "https://github.com/upstream/repo.git"},
	}
	for i, c := range cases {
		if entries[i].Name != c.name {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, c.name)
		}
		if entries[i].FetchURL != c.fetchURL {
			t.Errorf("entries[%d].FetchURL = %q, want %q", i, entries[i].FetchURL, c.fetchURL)
		}
		if entries[i].PushURL != c.pushURL {
			t.Errorf("entries[%d].PushURL = %q, want %q", i, entries[i].PushURL, c.pushURL)
		}
	}
}

func TestParseRemoteListDifferentFetchPush(t *testing.T) {
	output := "origin\tgit@github.com:org/repo.git (fetch)\norigin\thttps://github.com/org/repo.git (push)\n"
	entries := parseRemoteList(output)
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].FetchURL != "git@github.com:org/repo.git" {
		t.Errorf("FetchURL = %q", entries[0].FetchURL)
	}
	if entries[0].PushURL != "https://github.com/org/repo.git" {
		t.Errorf("PushURL = %q", entries[0].PushURL)
	}
}

func TestParseRemoteListEmpty(t *testing.T) {
	if entries := parseRemoteList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestExtractCommitHash(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"abc1234 feat: add login", "abc1234"},
		{"* abc1234 feat: add login", "abc1234"},
		{"| * abc1234 fix: null pointer", "abc1234"},
		{"| \\", ""},
		{"|/", ""},
		{"", ""},
		{"notahash", ""},
		{"abcdef0 feat: seven valid hex chars", "abcdef0"},
		{"ABCDEF0 uppercase hex is not matched", ""},
	}
	for _, c := range cases {
		got := extractCommitHash(c.line)
		if got != c.want {
			t.Errorf("extractCommitHash(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}
