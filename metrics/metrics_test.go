package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metrics.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenCreatesSchema(t *testing.T) {
	db := openTemp(t)
	s, err := db.Summarize()
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.TotalCommits != 0 || s.TotalErrors != 0 {
		t.Errorf("fresh db should have zero counts, got %+v", s)
	}
}

func TestRecordError(t *testing.T) {
	db := openTemp(t)
	if err := db.RecordError("git push", "rejected", "retry"); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	s, _ := db.Summarize()
	if s.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", s.TotalErrors)
	}
	if len(s.TopErrorCmds) != 1 || s.TopErrorCmds[0].Name != "git push" {
		t.Errorf("TopErrorCmds = %v, want [{git push 1}]", s.TopErrorCmds)
	}
}

func TestRecordViolation(t *testing.T) {
	db := openTemp(t)
	if err := db.RecordViolation("/repo", "johns-branch", "feat/"); err != nil {
		t.Fatalf("RecordViolation: %v", err)
	}
	s, _ := db.Summarize()
	if s.TotalViolations != 1 {
		t.Errorf("TotalViolations = %d, want 1", s.TotalViolations)
	}
}

func TestRecordCommit(t *testing.T) {
	db := openTemp(t)
	if err := db.RecordCommit("/repo", "feat/login", "standard"); err != nil {
		t.Fatalf("RecordCommit: %v", err)
	}
	s, _ := db.Summarize()
	if s.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", s.TotalCommits)
	}
}

func TestRecordHabit(t *testing.T) {
	db := openTemp(t)
	if err := db.RecordHabit("/repo"); err != nil {
		t.Fatalf("RecordHabit: %v", err)
	}
	s, _ := db.Summarize()
	if s.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", s.Sessions)
	}
}

func TestHourDistribution(t *testing.T) {
	db := openTemp(t)
	for i := 0; i < 3; i++ {
		_ = db.RecordCommit("/repo", "main", "standard")
	}
	s, _ := db.Summarize()
	total := 0
	for _, n := range s.HourDist {
		total += n
	}
	if total != 3 {
		t.Errorf("HourDist total = %d, want 3", total)
	}
}

func TestDefaultPath(t *testing.T) {
	// Redirect XDG_CONFIG_HOME to a temp dir so we don't touch ~/.config.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(dir, "bonsai", "metrics.db")
	if p != want {
		t.Errorf("DefaultPath = %q, want %q", p, want)
	}
}

func TestOpenIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	// Opening again should not fail even if schema already exists.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("db file should exist: %v", err)
	}
}
