package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	d, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("Load of missing file: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil Data")
	}
	if len(d.Counts) != 0 || len(d.Suppressed) != 0 || len(d.Prompted) != 0 {
		t.Error("expected empty maps on missing file")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	d := &Data{
		Counts:     map[string]int{"commit": 5, "push": 3},
		Suppressed: map[string]bool{"push": true},
		Prompted:   map[string]bool{"commit": true},
	}
	if err := d.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	d2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d2.Counts["commit"] != 5 {
		t.Errorf("Counts[commit] = %d, want 5", d2.Counts["commit"])
	}
	if d2.Counts["push"] != 3 {
		t.Errorf("Counts[push] = %d, want 3", d2.Counts["push"])
	}
	if !d2.Suppressed["push"] {
		t.Error("Suppressed[push] should be true")
	}
	if !d2.Prompted["commit"] {
		t.Error("Prompted[commit] should be true")
	}
}

func TestIncrement(t *testing.T) {
	d := &Data{
		Counts:     map[string]int{},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	if n := d.Increment("commit"); n != 1 {
		t.Errorf("first Increment = %d, want 1", n)
	}
	if n := d.Increment("commit"); n != 2 {
		t.Errorf("second Increment = %d, want 2", n)
	}
	if n := d.Increment("push"); n != 1 {
		t.Errorf("first push Increment = %d, want 1", n)
	}
}

func TestSuppressAndCheck(t *testing.T) {
	d := &Data{
		Counts:     map[string]int{},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	if d.IsSuppressed("commit") {
		t.Error("commit should not be suppressed initially")
	}
	d.Suppress("commit")
	if !d.IsSuppressed("commit") {
		t.Error("commit should be suppressed after Suppress")
	}
	if d.IsSuppressed("push") {
		t.Error("push should not be suppressed")
	}
}

func TestPrompted(t *testing.T) {
	d := &Data{
		Counts:     map[string]int{},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	if d.WasPrompted("rebase") {
		t.Error("rebase should not have been prompted initially")
	}
	d.SetPrompted("rebase")
	if !d.WasPrompted("rebase") {
		t.Error("rebase should be prompted after SetPrompted")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "usage.json")
	d := &Data{
		Counts:     map[string]int{"commit": 1},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	if err := d.Save(path); err != nil {
		t.Fatalf("Save with nested dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
