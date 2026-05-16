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
