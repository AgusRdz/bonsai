package tui

import (
	"os"
	"strconv"
	"testing"
)

func TestLockOwnerAlive(t *testing.T) {
	cases := []struct {
		name      string
		reason    string
		wantKnown bool
	}{
		{"epic-orchestrator format", "claude agent agent-a65aadfc4b09df79a (pid 9500)", true},
		{"bare pid", "pid 4242", true},
		{"pid with colon", "locked by tool pid: 4242", true},
		{"no pid", "manual lock while editing", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, known := lockOwnerAlive(c.reason); known != c.wantKnown {
				t.Errorf("lockOwnerAlive(%q) known = %v, want %v", c.reason, known, c.wantKnown)
			}
		})
	}
}

func TestLockOwnerAliveLiveProcess(t *testing.T) {
	// The current test process is guaranteed alive.
	reason := "claude agent test (pid " + strconv.Itoa(os.Getpid()) + ")"
	alive, known := lockOwnerAlive(reason)
	if !known {
		t.Fatalf("expected pid to be parsed from %q", reason)
	}
	if !alive {
		t.Errorf("current process (pid %d) reported not alive", os.Getpid())
	}
}

func TestLockOwnerAliveDeadProcess(t *testing.T) {
	// PID 0 is never a normal user process; processAlive rejects pid <= 0.
	if alive, known := lockOwnerAlive("stale lock (pid 0)"); !known || alive {
		t.Errorf("pid 0 should parse as known-and-dead, got known=%v alive=%v", known, alive)
	}
}
