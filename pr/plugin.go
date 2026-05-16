package pr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExternalProvider wraps a bonsai-pr-<name> binary found in PATH.
// Protocol (all communication via stdin/stdout):
//
//	bonsai-pr-<name> detect <remote-url>        -> "yes" or "no"
//	bonsai-pr-<name> status <branch>            -> JSON PRStatus
//	bonsai-pr-<name> create <branch>            -> any output (opens flow)
//	bonsai-pr-<name> list                       -> JSON []PRStatus
//	bonsai-pr-<name> open <branch>              -> any output (opens browser)
type ExternalProvider struct {
	name   string // the part after "bonsai-pr-"
	binary string // full path to the binary
}

func (e *ExternalProvider) Name() string { return e.name }

func (e *ExternalProvider) CLIAvailable() bool {
	_, err := os.Stat(e.binary)
	return err == nil
}

func (e *ExternalProvider) DetectRemote(remoteURL string) bool {
	out, err := exec.Command(e.binary, "detect", remoteURL).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "yes"
}

func (e *ExternalProvider) CurrentPR(ctx context.Context, branch string) (*PRStatus, error) {
	out, err := exec.CommandContext(ctx, e.binary, "status", branch).Output()
	if err != nil {
		return nil, fmt.Errorf("plugin %s status: %w", e.name, err)
	}
	var s PRStatus
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&s); err != nil {
		return nil, fmt.Errorf("plugin %s status parse: %w", e.name, err)
	}
	return &s, nil
}

func (e *ExternalProvider) CreatePR(ctx context.Context, branch string) error {
	cmd := exec.CommandContext(ctx, e.binary, "create", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *ExternalProvider) ListPRs(ctx context.Context) ([]PRStatus, error) {
	out, err := exec.CommandContext(ctx, e.binary, "list").Output()
	if err != nil {
		return nil, fmt.Errorf("plugin %s list: %w", e.name, err)
	}
	var list []PRStatus
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&list); err != nil {
		return nil, fmt.Errorf("plugin %s list parse: %w", e.name, err)
	}
	return list, nil
}

func (e *ExternalProvider) Open(ctx context.Context, branch string) error {
	return exec.CommandContext(ctx, e.binary, "open", branch).Run()
}

// discoverPlugins finds all bonsai-pr-* binaries in PATH directories and registers them.
func discoverPlugins() {
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	seen := map[string]bool{}

	for _, dir := range pathDirs {
		matches, err := filepath.Glob(filepath.Join(dir, "bonsai-pr-*"))
		if err != nil {
			continue
		}
		for _, match := range matches {
			base := filepath.Base(match)
			pluginName := strings.TrimPrefix(base, "bonsai-pr-")
			if pluginName == "" || seen[pluginName] {
				continue
			}
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			// On Unix, check the executable bit.
			if info.Mode()&0o111 == 0 {
				continue
			}
			seen[pluginName] = true
			Register(&ExternalProvider{name: pluginName, binary: match})
		}
	}
}
