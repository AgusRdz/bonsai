//go:build windows

package tui

import "golang.org/x/sys/windows"

// processAlive reports whether a process with the given pid is currently
// running. On Windows it opens a query-limited handle and inspects the exit
// code: a live process reports STILL_ACTIVE (259). If the pid cannot be opened
// the process is gone (or the pid was recycled into something we can't touch),
// so we treat it as dead. When in doubt we err toward "alive" so a locked
// worktree is never force-removed out from under a running agent.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return true // handle exists but couldn't read status — assume alive (safe)
	}
	const stillActive = 259 // STILL_ACTIVE
	return code == stillActive
}
