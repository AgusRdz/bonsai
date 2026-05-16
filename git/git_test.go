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
	output := "  main\n* feat/login\n  fix/bug-123\n"
	branches := parseBranches(output)

	if len(branches) != 3 {
		t.Fatalf("branch count = %d, want 3", len(branches))
	}

	cases := []struct {
		name    string
		current bool
	}{
		{"main", false},
		{"feat/login", true},
		{"fix/bug-123", false},
	}
	for i, c := range cases {
		if branches[i].Name != c.name {
			t.Errorf("branches[%d].Name = %q, want %q", i, branches[i].Name, c.name)
		}
		if branches[i].Current != c.current {
			t.Errorf("branches[%d].Current = %v, want %v", i, branches[i].Current, c.current)
		}
	}
}

func TestParseBranchesEmpty(t *testing.T) {
	if branches := parseBranches(""); len(branches) != 0 {
		t.Errorf("expected empty, got %v", branches)
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
