package tui

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	porcelain := "" +
		"M  staged-modified.go\n" +
		"A  staged-added.go\n" +
		" M unstaged-modified.go\n" +
		"MM both-staged-and-changed.go\n" +
		"?? untracked.go\n" +
		"D  staged-deleted.go\n"

	staged, changed, untracked := parseStatus(porcelain)

	wantStaged := []string{
		"M  staged-modified.go",
		"A  staged-added.go",
		"M  both-staged-and-changed.go",
		"D  staged-deleted.go",
	}
	wantChanged := []string{
		"M  unstaged-modified.go",
		"M  both-staged-and-changed.go",
	}
	wantUntracked := []string{"untracked.go"}

	if len(staged) != len(wantStaged) {
		t.Errorf("staged count = %d, want %d: %v", len(staged), len(wantStaged), staged)
	}
	for i, w := range wantStaged {
		if i < len(staged) && staged[i] != w {
			t.Errorf("staged[%d] = %q, want %q", i, staged[i], w)
		}
	}

	if len(changed) != len(wantChanged) {
		t.Errorf("changed count = %d, want %d: %v", len(changed), len(wantChanged), changed)
	}
	for i, w := range wantChanged {
		if i < len(changed) && changed[i] != w {
			t.Errorf("changed[%d] = %q, want %q", i, changed[i], w)
		}
	}

	if len(untracked) != len(wantUntracked) {
		t.Errorf("untracked count = %d, want %d: %v", len(untracked), len(wantUntracked), untracked)
	}
	for i, w := range wantUntracked {
		if i < len(untracked) && untracked[i] != w {
			t.Errorf("untracked[%d] = %q, want %q", i, untracked[i], w)
		}
	}
}

func TestParseStatusEmpty(t *testing.T) {
	staged, changed, untracked := parseStatus("")
	if len(staged) != 0 || len(changed) != 0 || len(untracked) != 0 {
		t.Errorf("expected all empty, got staged=%v changed=%v untracked=%v", staged, changed, untracked)
	}
}

func TestParseStatusClean(t *testing.T) {
	staged, changed, untracked := parseStatus("\n\n")
	if len(staged) != 0 || len(changed) != 0 || len(untracked) != 0 {
		t.Errorf("expected all empty for clean repo output")
	}
}
