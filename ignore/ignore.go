package ignore

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scope identifies which gitignore file an operation targets.
type Scope int

const (
	ScopeGlobal  Scope = iota // core.excludesfile or ~/.config/git/ignore
	ScopeLocal                // .gitignore in the current repo root
	ScopeExclude              // .git/info/exclude — local personal, never committed
)

// ScopeLabel returns a human-readable name for the scope.
func ScopeLabel(s Scope) string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeLocal:
		return "local"
	case ScopeExclude:
		return "exclude"
	}
	return "unknown"
}

// PatternEntry is a single non-blank, non-comment line from a gitignore file.
type PatternEntry struct {
	Pattern string
	Scope   Scope
	File    string // absolute path to the source file
	Line    int    // 1-based line number
}

// GlobalPath returns the effective path for the global gitignore file.
// Reads core.excludesfile; falls back to ~/.config/git/ignore.
func GlobalPath() (string, error) {
	out, err := exec.Command("git", "config", "--global", "--get", "core.excludesfile").Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return expandTilde(p)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git", "ignore"), nil
}

// FilePath returns the gitignore file path for the given scope.
func FilePath(scope Scope) (string, error) {
	switch scope {
	case ScopeGlobal:
		return GlobalPath()
	case ScopeLocal:
		return ".gitignore", nil
	case ScopeExclude:
		return ".git/info/exclude", nil
	}
	return "", fmt.Errorf("unknown scope")
}

// List returns all non-blank, non-comment patterns across all three scopes.
func List() (global, local, exclude []PatternEntry, err error) {
	globalPath, gerr := GlobalPath()
	if gerr != nil {
		return nil, nil, nil, gerr
	}
	global = readPatterns(globalPath, ScopeGlobal)
	local = readPatterns(".gitignore", ScopeLocal)
	exclude = readPatterns(".git/info/exclude", ScopeExclude)
	return global, local, exclude, nil
}

func readPatterns(path string, scope Scope) []PatternEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []PatternEntry
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entries = append(entries, PatternEntry{
			Pattern: trimmed,
			Scope:   scope,
			File:    path,
			Line:    line,
		})
	}
	return entries
}

// Add appends a pattern to the gitignore file for the given scope.
// Returns an error if the pattern already exists in that file.
func Add(scope Scope, pattern string) error {
	path, err := FilePath(scope)
	if err != nil {
		return err
	}

	// Ensure parent directory exists (e.g. ~/.config/git/ on first run).
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}

	// Check for duplicate.
	existing := readPatterns(path, scope)
	for _, e := range existing {
		if e.Pattern == pattern {
			return fmt.Errorf("%q already in %s", pattern, path)
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure the file ends with a newline before appending.
	if needsNewline(path) {
		fmt.Fprintln(f)
	}
	_, err = fmt.Fprintln(f, pattern)
	return err
}

// Remove deletes a pattern from the gitignore file for the given scope.
// Returns an error if the pattern is not found.
func Remove(scope Scope, pattern string) error {
	path, err := FilePath(scope)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	removed := false
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return fmt.Errorf("%q not found in %s", pattern, path)
	}

	// Trim trailing blank lines left by the removal, keep a final newline.
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}

	return os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0644)
}

// Check runs `git check-ignore -v --no-index` against the given pattern
// and returns the list of files/directories in the working tree that would match.
func Check(pattern string) ([]string, error) {
	// Write pattern to a temp file so we don't modify any real gitignore.
	tmp, err := os.CreateTemp("", "bonsai-ignore-check-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	fmt.Fprintln(tmp, pattern)
	tmp.Close()

	// `git ls-files --others --cached --directory` gives us candidate paths.
	lsOut, err := exec.Command("git", "ls-files", "--others", "--cached", "--directory", "--exclude-from="+tmp.Name()).Output()
	if err != nil {
		// git ls-files exits 0 even with no matches; a real error means no repo.
		return nil, fmt.Errorf("not a git repository or git error: %w", err)
	}

	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(lsOut)), "\n") {
		if line != "" {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

// Seed writes the base patterns (and optionally language patterns) into the
// given scope's gitignore file, skipping patterns that already exist.
// Returns the number of patterns written.
func Seed(scope Scope, langs []string) (int, error) {
	patterns := basePatterns()
	for _, lang := range langs {
		patterns = append(patterns, langPatterns(lang)...)
	}

	path, err := FilePath(scope)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, err
	}

	existing := readPatterns(path, scope)
	existingSet := make(map[string]bool, len(existing))
	for _, e := range existing {
		existingSet[e.Pattern] = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if needsNewline(path) {
		fmt.Fprintln(f)
	}

	written := 0
	for _, block := range patterns {
		if existingSet[block.pattern] {
			continue
		}
		if block.comment != "" && written == 0 {
			fmt.Fprintf(f, "# %s\n", block.comment)
		} else if block.comment != "" {
			fmt.Fprintf(f, "\n# %s\n", block.comment)
		}
		fmt.Fprintln(f, block.pattern)
		existingSet[block.pattern] = true
		written++
	}
	return written, nil
}

// needsNewline reports whether the file exists, is non-empty, and does not
// end with a newline — so we know to add one before appending.
func needsNewline(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return false
	}
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, info.Size()-1); err != nil {
		return false
	}
	return buf[0] != '\n'
}

func expandTilde(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[2:]), nil
}
