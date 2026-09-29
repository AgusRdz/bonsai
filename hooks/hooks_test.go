package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setupShared runs the test from a temp dir holding a .githooks/pre-commit hook.
func setupShared(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := Add(ScopeShared, "pre-commit", DefaultScript("pre-commit"), false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	return sharedDir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDisable_RenamesHook(t *testing.T) {
	dir := setupShared(t)

	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if exists(filepath.Join(dir, "pre-commit")) {
		t.Error("pre-commit still present — git would still run it")
	}
	if !exists(filepath.Join(dir, "pre-commit.disabled")) {
		t.Error("pre-commit.disabled missing — hook content lost")
	}
}

func TestDisable_AlreadyDisabledIsNoop(t *testing.T) {
	setupShared(t)

	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("first Disable: %v", err)
	}
	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Errorf("second Disable: %v, want nil", err)
	}
}

func TestDisable_MissingHook(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := Disable(ScopeShared, "pre-commit"); err == nil {
		t.Error("Disable of missing hook: want error, got nil")
	}
}

func TestEnable_RestoresDisabledHook(t *testing.T) {
	dir := setupShared(t)

	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if err := Enable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !exists(filepath.Join(dir, "pre-commit")) {
		t.Error("pre-commit missing after Enable")
	}
	if exists(filepath.Join(dir, "pre-commit.disabled")) {
		t.Error("pre-commit.disabled still present after Enable")
	}
}

func TestEnable_RestoresLegacyChmodDisabledHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit not supported on Windows")
	}
	dir := setupShared(t)
	path := filepath.Join(dir, "pre-commit")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Enable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("mode = %v, want executable", info.Mode())
	}
}

func TestScanDir_ReportsDisabledHook(t *testing.T) {
	dir := setupShared(t)
	if err := Add(ScopeShared, "commit-msg", DefaultScript("commit-msg"), false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	got := map[string]bool{}
	for _, e := range scanDir(dir, ScopeShared) {
		got[e.Name] = e.Active
	}
	want := map[string]bool{"commit-msg": true, "pre-commit": false}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for name, active := range want {
		if a, ok := got[name]; !ok || a != active {
			t.Errorf("%s: active = %v (present %v), want %v", name, a, ok, active)
		}
	}
}

func TestShowAndRemove_WorkOnDisabledHook(t *testing.T) {
	dir := setupShared(t)
	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	content, err := Show(ScopeShared, "pre-commit")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if content != DefaultScript("pre-commit") {
		t.Errorf("Show = %q, want default script", content)
	}
	if err := Remove(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if exists(filepath.Join(dir, "pre-commit.disabled")) {
		t.Error("pre-commit.disabled still present after Remove")
	}
}

func TestAdd_RejectsWhenDisabledCopyExists(t *testing.T) {
	setupShared(t)
	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	if err := Add(ScopeShared, "pre-commit", "new", false); err == nil {
		t.Error("Add without force: want error, got nil")
	}
}

func TestAdd_ForceReplacesDisabledCopy(t *testing.T) {
	dir := setupShared(t)
	if err := Disable(ScopeShared, "pre-commit"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	if err := Add(ScopeShared, "pre-commit", "new", true); err != nil {
		t.Fatalf("Add with force: %v", err)
	}
	if exists(filepath.Join(dir, "pre-commit.disabled")) {
		t.Error("pre-commit.disabled left behind after forced Add")
	}
	if got, _ := Show(ScopeShared, "pre-commit"); got != "new" {
		t.Errorf("Show = %q, want %q", got, "new")
	}
}
