package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	Discover()
	if len(Plugins()) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(Plugins()))
	}
}

func TestDiscoverFindsExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-plugin-jira")
	script := "#!/bin/sh\necho '{\"ok\":true,\"message\":\"linked\"}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)
	Discover()
	if len(Plugins()) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(Plugins()))
	}
	if Plugins()[0].Name != "jira" {
		t.Errorf("Name = %q, want jira", Plugins()[0].Name)
	}
}

func TestDiscoverSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-plugin-slack")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)
	Discover()
	if len(Plugins()) != 0 {
		t.Errorf("expected 0 plugins (non-executable), got %d", len(Plugins()))
	}
}

func TestFireSyncSinglePlugin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-plugin-test")
	script := "#!/bin/sh\necho '{\"ok\":true,\"message\":\"hello\"}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)
	Discover()

	resps := FireSync(Request{Event: EventCommitCreated, Branch: "feat/login"})
	if len(resps) != 1 {
		t.Fatalf("response count = %d, want 1", len(resps))
	}
	if !resps[0].OK || resps[0].Message != "hello" {
		t.Errorf("response = %+v, want {OK:true Message:hello}", resps[0])
	}
}

func TestFireSyncMultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"bonsai-plugin-a", "bonsai-plugin-b"} {
		bin := filepath.Join(dir, name)
		script := "#!/bin/sh\necho '{\"ok\":true,\"message\":\"" + name + "\"}'\n"
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
	Discover()

	resps := FireSync(Request{Event: EventPushAfter})
	if len(resps) != 2 {
		t.Fatalf("response count = %d, want 2 (one per plugin)", len(resps))
	}
	seen := map[string]bool{}
	for _, r := range resps {
		if !r.OK {
			t.Errorf("response not OK: %+v", r)
		}
		seen[r.Message] = true
	}
	if !seen["bonsai-plugin-a"] || !seen["bonsai-plugin-b"] {
		t.Errorf("expected both plugins to respond, got %v", seen)
	}
}

func TestFireSyncBadPlugin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-plugin-bad")
	// Plugin that exits non-zero.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)
	Discover()

	// Should not panic or error - bad plugins are silently skipped.
	resps := FireSync(Request{Event: EventPushAfter})
	if len(resps) != 0 {
		t.Errorf("expected 0 responses from bad plugin, got %d", len(resps))
	}
}

func TestFireSyncInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-plugin-badjson")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'not json'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)
	Discover()

	resps := FireSync(Request{Event: EventPushAfter})
	if len(resps) != 0 {
		t.Errorf("expected 0 responses from plugin with bad JSON output, got %d", len(resps))
	}
}
