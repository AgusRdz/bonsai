package errlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "error.log")
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	if err := Append(path, Entry{Time: ts, Repo: "/r", Branch: "main", Cmd: "git push", Err: "rejected\nhint: pull first\r\n"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Entry{Time: ts, Repo: "/r", Branch: "dev", Err: "boom"}); err != nil {
		t.Fatal(err)
	}

	got, err := Tail(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d: %q", len(got), got)
	}
	want0 := "2026-01-02T03:04:05Z  repo=/r  branch=main\n  cmd: git push\n  error: rejected\n    hint: pull first\n"
	if got[0] != want0 {
		t.Errorf("entry 0:\n got %q\nwant %q", got[0], want0)
	}
	if strings.Contains(got[1], "cmd:") {
		t.Errorf("entry without cmd should omit cmd line: %q", got[1])
	}

	last, _ := Tail(path, 1)
	if len(last) != 1 || !strings.Contains(last[0], "branch=dev") {
		t.Errorf("Tail(1) = %q", last)
	}
}

func TestTailMissingFile(t *testing.T) {
	got, err := Tail(filepath.Join(t.TempDir(), "nope.log"), 10)
	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error.log")
	if err := os.WriteFile(path, make([]byte, maxSize), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Entry{Time: time.Now(), Err: "x"}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path + ".1"); err != nil || fi.Size() != maxSize {
		t.Fatalf("rotated file missing or wrong size: %v", err)
	}
	if fi, _ := os.Stat(path); fi.Size() >= maxSize {
		t.Fatalf("active log not rotated, size %d", fi.Size())
	}

	if err := Clear(path); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + ".1"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after Clear", p)
		}
	}
}
