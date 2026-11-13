// Package errlog appends failed operations to a plain-text log file so they
// can be inspected after the TUI has moved on. The file is local-only and
// rotated once when it grows past maxSize.
package errlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxSize = 1 << 20 // 1 MiB

// DefaultPath returns the path to the error log. It lives next to the bonsai
// config file and metrics database.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bonsai", "error.log"), nil
}

// Entry is one failed operation.
type Entry struct {
	Time   time.Time
	Repo   string
	Branch string
	Cmd    string
	Err    string
}

// Append writes e to the log at path, rotating the file to path+".1" first if
// it has reached maxSize.
func Append(path string, e Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() >= maxSize {
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(format(e))
	return err
}

// format renders an entry as a header line followed by indented detail lines.
// Every continuation line is indented, so an unindented line always starts a
// new entry.
func format(e Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  repo=%s  branch=%s\n", e.Time.UTC().Format(time.RFC3339), e.Repo, e.Branch)
	if e.Cmd != "" {
		fmt.Fprintf(&b, "  cmd: %s\n", e.Cmd)
	}
	for i, line := range strings.Split(strings.TrimRight(e.Err, "\r\n"), "\n") {
		if i == 0 {
			fmt.Fprintf(&b, "  error: %s\n", strings.TrimRight(line, "\r"))
		} else {
			fmt.Fprintf(&b, "    %s\n", strings.TrimRight(line, "\r"))
		}
	}
	return b.String()
}

// Tail returns the last n entries from the log at path, oldest first, each as
// its raw multi-line text. A missing file yields no entries and no error.
func Tail(path string, n int) ([]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []string
	var cur strings.Builder
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && cur.Len() > 0 {
			entries = append(entries, cur.String())
			cur.Reset()
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
	}
	if cur.Len() > 0 {
		entries = append(entries, cur.String())
	}
	if n > 0 && len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	return entries, nil
}

// Clear removes the log and its rotated predecessor.
func Clear(path string) error {
	for _, p := range []string{path, path + ".1"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
