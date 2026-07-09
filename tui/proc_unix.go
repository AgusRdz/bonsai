//go:build !windows

package tui

import (
	"errors"
	"os"
	"syscall"
)

// processAlive reports whether a process with the given pid is currently
// running. On Unix it sends signal 0, which performs the permission/existence
// checks without delivering a signal: nil means the process exists, EPERM means
// it exists but is owned by another user, and ErrProcessDone means it is gone.
// When in doubt we err toward "alive" so a locked worktree is never
// force-removed out from under a running agent.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrProcessDone) {
		return false
	}
	return errors.Is(err, syscall.EPERM)
}
