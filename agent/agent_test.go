package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseSingleFileDiff
// ---------------------------------------------------------------------------

func TestParseSingleFileDiff_Modified(t *testing.T) {
	raw := `diff --git a/src/auth.go b/src/auth.go
index abc1234..def5678 100644
--- a/src/auth.go
+++ b/src/auth.go
@@ -1,5 +1,7 @@
 package auth

-func Login() {}
+func Login(user string) {
+	// validate user
+}

 func Logout() {}`

	fd := parseSingleFileDiff(raw)

	if fd.Path != "src/auth.go" {
		t.Errorf("path = %q, want src/auth.go", fd.Path)
	}
	if fd.Additions != 3 {
		t.Errorf("additions = %d, want 3", fd.Additions)
	}
	if fd.Deletions != 1 {
		t.Errorf("deletions = %d, want 1", fd.Deletions)
	}
	if len(fd.Hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(fd.Hunks))
	}
	if fd.Hunks[0].Header != "@@ -1,5 +1,7 @@" {
		t.Errorf("hunk header = %q, want @@ -1,5 +1,7 @@", fd.Hunks[0].Header)
	}
}

func TestParseSingleFileDiff_NewFile(t *testing.T) {
	raw := `diff --git a/src/new.go b/src/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/src/new.go
@@ -0,0 +1,3 @@
+package new
+
+func New() {}`

	fd := parseSingleFileDiff(raw)

	if fd.Path != "src/new.go" {
		t.Errorf("path = %q, want src/new.go", fd.Path)
	}
	if fd.Additions != 3 {
		t.Errorf("additions = %d, want 3", fd.Additions)
	}
	if fd.Deletions != 0 {
		t.Errorf("deletions = %d, want 0", fd.Deletions)
	}
}

func TestParseSingleFileDiff_DeletedFile(t *testing.T) {
	raw := `diff --git a/src/old.go b/src/old.go
deleted file mode 100644
index abc1234..0000000
--- a/src/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package old
-
-func Old() {}`

	fd := parseSingleFileDiff(raw)

	if fd.Path != "src/old.go" {
		t.Errorf("path = %q, want src/old.go", fd.Path)
	}
	if fd.Additions != 0 {
		t.Errorf("additions = %d, want 0", fd.Additions)
	}
	if fd.Deletions != 3 {
		t.Errorf("deletions = %d, want 3", fd.Deletions)
	}
}

func TestParseSingleFileDiff_MultipleHunks(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
index aaa..bbb 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

+// added comment
 func main() {
@@ -10,3 +11,3 @@
 	fmt.Println("hello")
-	return
+	os.Exit(0)
 }`

	fd := parseSingleFileDiff(raw)

	if fd.Path != "main.go" {
		t.Errorf("path = %q, want main.go", fd.Path)
	}
	if len(fd.Hunks) != 2 {
		t.Fatalf("hunks = %d, want 2", len(fd.Hunks))
	}
	if fd.Additions != 2 {
		t.Errorf("additions = %d, want 2", fd.Additions)
	}
	if fd.Deletions != 1 {
		t.Errorf("deletions = %d, want 1", fd.Deletions)
	}
}

func TestParseSingleFileDiff_Empty(t *testing.T) {
	fd := parseSingleFileDiff("")
	if fd.Path != "" {
		t.Errorf("expected empty FileDiff for empty input, got path %q", fd.Path)
	}
	if fd.Additions != 0 || fd.Deletions != 0 {
		t.Errorf("expected zero counts, got +%d -%d", fd.Additions, fd.Deletions)
	}
}

func TestParseSingleFileDiff_HunksNotNil(t *testing.T) {
	// Hunks slice must be [] not null in JSON output.
	raw := `diff --git a/empty.go b/empty.go
index aaa..bbb 100644
--- a/empty.go
+++ b/empty.go`

	fd := parseSingleFileDiff(raw)
	if fd.Hunks == nil {
		t.Error("Hunks should be an empty slice, not nil")
	}
}

// ---------------------------------------------------------------------------
// parseMultiFileDiff
// ---------------------------------------------------------------------------

func TestParseMultiFileDiff_Empty(t *testing.T) {
	if files := parseMultiFileDiff(""); files != nil {
		t.Errorf("expected nil for empty input, got %v", files)
	}
	if files := parseMultiFileDiff("   \n  "); files != nil {
		t.Errorf("expected nil for whitespace input, got %v", files)
	}
}

func TestParseMultiFileDiff_SingleFile(t *testing.T) {
	raw := `diff --git a/src/api.go b/src/api.go
index abc..def 100644
--- a/src/api.go
+++ b/src/api.go
@@ -1,3 +1,4 @@
 package api

+// new comment
 func Handle() {}`

	files := parseMultiFileDiff(raw)

	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Path != "src/api.go" {
		t.Errorf("path = %q, want src/api.go", files[0].Path)
	}
	if files[0].Additions != 1 {
		t.Errorf("additions = %d, want 1", files[0].Additions)
	}
}

func TestParseMultiFileDiff_TwoFiles(t *testing.T) {
	raw := `diff --git a/a.go b/a.go
index 111..222 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 package a
+// added
 func A() {}
diff --git a/b.go b/b.go
index 333..444 100644
--- a/b.go
+++ b/b.go
@@ -1,3 +1,2 @@
 package b
-// removed
 func B() {}`

	files := parseMultiFileDiff(raw)

	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Path != "a.go" {
		t.Errorf("files[0].path = %q, want a.go", files[0].Path)
	}
	if files[1].Path != "b.go" {
		t.Errorf("files[1].path = %q, want b.go", files[1].Path)
	}
	if files[0].Additions != 1 || files[0].Deletions != 0 {
		t.Errorf("a.go counts = +%d -%d, want +1 -0", files[0].Additions, files[0].Deletions)
	}
	if files[1].Additions != 0 || files[1].Deletions != 1 {
		t.Errorf("b.go counts = +%d -%d, want +0 -1", files[1].Additions, files[1].Deletions)
	}
}

func TestParseMultiFileDiff_ThreeFiles(t *testing.T) {
	raw := `diff --git a/one.go b/one.go
--- a/one.go
+++ b/one.go
@@ -1,1 +1,2 @@
 x
+y
diff --git a/two.go b/two.go
--- a/two.go
+++ b/two.go
@@ -1,2 +1,1 @@
-a
 b
diff --git a/three.go b/three.go
new file mode 100644
--- /dev/null
+++ b/three.go
@@ -0,0 +1,1 @@
+package three`

	files := parseMultiFileDiff(raw)

	if len(files) != 3 {
		t.Fatalf("file count = %d, want 3", len(files))
	}
	paths := []string{"one.go", "two.go", "three.go"}
	for i, want := range paths {
		if files[i].Path != want {
			t.Errorf("files[%d].path = %q, want %q", i, files[i].Path, want)
		}
	}
}

func TestParseMultiFileDiff_MixedNewAndDeleted(t *testing.T) {
	raw := `diff --git a/deleted.go b/deleted.go
deleted file mode 100644
--- a/deleted.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package x
-func X() {}
diff --git a/created.go b/created.go
new file mode 100644
--- /dev/null
+++ b/created.go
@@ -0,0 +1,2 @@
+package y
+func Y() {}`

	files := parseMultiFileDiff(raw)

	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Path != "deleted.go" {
		t.Errorf("files[0].path = %q, want deleted.go", files[0].Path)
	}
	if files[0].Deletions != 2 || files[0].Additions != 0 {
		t.Errorf("deleted.go counts = +%d -%d, want +0 -2", files[0].Additions, files[0].Deletions)
	}
	if files[1].Path != "created.go" {
		t.Errorf("files[1].path = %q, want created.go", files[1].Path)
	}
	if files[1].Additions != 2 || files[1].Deletions != 0 {
		t.Errorf("created.go counts = +%d -%d, want +2 -0", files[1].Additions, files[1].Deletions)
	}
}

// ---------------------------------------------------------------------------
// applyStatuses
// ---------------------------------------------------------------------------

func TestApplyStatuses(t *testing.T) {
	files := []FileDiff{
		{Path: "src/auth.go"},
		{Path: "src/new.go"},
		{Path: "src/old.go"},
		{Path: "src/other.go"},
	}
	nameStatus := map[string]string{
		"src/auth.go": "M",
		"src/new.go":  "A",
		"src/old.go":  "D",
	}

	applyStatuses(files, nameStatus)

	cases := []struct{ path, want string }{
		{"src/auth.go", "M"},
		{"src/new.go", "A"},
		{"src/old.go", "D"},
		{"src/other.go", ""},
	}
	for i, c := range cases {
		if files[i].Status != c.want {
			t.Errorf("files[%d].Status = %q, want %q", i, files[i].Status, c.want)
		}
	}
}

func TestApplyStatuses_Empty(t *testing.T) {
	files := []FileDiff{{Path: "x.go"}}
	applyStatuses(files, nil)
	if files[0].Status != "" {
		t.Errorf("expected empty status for nil map, got %q", files[0].Status)
	}
	applyStatuses(files, map[string]string{})
	if files[0].Status != "" {
		t.Errorf("expected empty status for empty map, got %q", files[0].Status)
	}
}

// ---------------------------------------------------------------------------
// FileDiff JSON serialisation — progressive disclosure
// ---------------------------------------------------------------------------

func TestFileDiffHunksOmittedWhenNil(t *testing.T) {
	fd := FileDiff{Path: "foo.go", Additions: 1, Deletions: 0}
	data, err := json.Marshal(fd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "hunks") {
		t.Errorf("expected hunks to be omitted for nil slice, got %s", string(data))
	}
}

func TestFileDiffHunksOmittedWhenEmpty(t *testing.T) {
	fd := FileDiff{Path: "foo.go", Additions: 1, Deletions: 0, Hunks: []HunkOut{}}
	data, err := json.Marshal(fd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "hunks") {
		t.Errorf("expected hunks to be omitted for empty slice, got %s", string(data))
	}
}

func TestFileDiffHunksIncludedWhenPopulated(t *testing.T) {
	fd := FileDiff{
		Path:      "foo.go",
		Additions: 1,
		Hunks:     []HunkOut{{Header: "@@ -1,1 +1,2 @@", Lines: []string{"+new"}}},
	}
	data, err := json.Marshal(fd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "hunks") {
		t.Errorf("expected hunks to be present when populated, got %s", string(data))
	}
}

func TestDiffOutStructure(t *testing.T) {
	out := DiffOut{
		Staged:    []FileDiff{},
		Unstaged:  []FileDiff{},
		Untracked: []UntrackedEntry{},
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{"staged", "unstaged", "untracked"} {
		if !strings.Contains(s, `"`+key+`"`) {
			t.Errorf("expected %q key in DiffOut JSON, got %s", key, s)
		}
	}
}

func TestShowOutStructure(t *testing.T) {
	out := ShowOut{
		Hash:    "abc1234",
		Subject: "feat: add thing",
		Author:  "Alice",
		Date:    "2026-05-17",
		Diff:    []FileDiff{},
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, key := range []string{"hash", "subject", "author", "date", "diff"} {
		if !strings.Contains(s, `"`+key+`"`) {
			t.Errorf("expected %q key in ShowOut JSON, got %s", key, s)
		}
	}
}
