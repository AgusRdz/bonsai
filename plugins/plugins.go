// Package plugins discovers and invokes bonsai event plugins.
//
// A plugin is an executable named bonsai-plugin-<name> found anywhere in PATH.
// bonsai sends events to it via stdin as JSON and reads a JSON response from stdout.
//
// Request format:
//
//	{"event":"branch.created","branch":"feat/login","repo":"/path/to/repo"}
//
// Response format:
//
//	{"ok":true,"message":"Ticket PROJ-123 linked"}
//
// Available events:
//
//	branch.created   branch.deleted
//	commit.created   push.before   push.after
//	merge.before     merge.after
//
// Plugins that take longer than PluginTimeout are killed silently.
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PluginTimeout is how long bonsai waits for a plugin response.
const PluginTimeout = 5 * time.Second

// Event names sent to plugins.
const (
	EventBranchCreated = "branch.created"
	EventBranchDeleted = "branch.deleted"
	EventCommitCreated = "commit.created"
	EventPushBefore    = "push.before"
	EventPushAfter     = "push.after"
	EventMergeBefore   = "merge.before"
	EventMergeAfter    = "merge.after"
)

// Request is the JSON payload sent to a plugin on stdin.
type Request struct {
	Event  string `json:"event"`
	Branch string `json:"branch,omitempty"`
	Repo   string `json:"repo,omitempty"`
	Hash   string `json:"hash,omitempty"`   // commit hash for commit events
	Remote string `json:"remote,omitempty"` // remote name for push events
}

// Response is the JSON payload read from a plugin on stdout.
type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Plugin represents a discovered bonsai-plugin-<name> binary.
type Plugin struct {
	Name   string
	Binary string
}

var discovered []Plugin

// Discover finds all bonsai-plugin-* executables in PATH and caches them.
// Call once at startup. Safe to call multiple times (re-scans each time).
func Discover() {
	discovered = nil
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		matches, err := filepath.Glob(filepath.Join(dir, "bonsai-plugin-*"))
		if err != nil {
			continue
		}
		for _, match := range matches {
			base := filepath.Base(match)
			name := strings.TrimPrefix(base, "bonsai-plugin-")
			if name == "" || seen[name] {
				continue
			}
			info, err := os.Stat(match)
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}
			seen[name] = true
			discovered = append(discovered, Plugin{Name: name, Binary: match})
		}
	}
}

// Plugins returns the currently discovered plugin list.
func Plugins() []Plugin { return discovered }

// Fire sends an event to all discovered plugins asynchronously.
// Errors and slow plugins are silently ignored - plugins must never block the TUI.
func Fire(req Request) {
	if req.Repo == "" {
		req.Repo, _ = os.Getwd()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return
	}
	for _, p := range discovered {
		go invoke(p.Binary, payload)
	}
}

// FireSync sends an event and returns all plugin responses.
// Used in tests or CLI subcommands where blocking is acceptable.
func FireSync(req Request) []Response {
	if req.Repo == "" {
		req.Repo, _ = os.Getwd()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil
	}
	var results []Response
	for _, p := range discovered {
		if r, ok := invoke(p.Binary, payload); ok {
			results = append(results, r)
		}
	}
	return results
}

// invoke runs a single plugin binary with the JSON payload on stdin.
func invoke(binary string, payload []byte) (Response, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), PluginTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary)
	cmd.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return Response{}, false
	}

	var resp Response
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		return Response{}, false
	}
	return resp, true
}
