package gitcheck

import (
	"testing"
)

func TestParseGitVersion(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"git version 2.45.0\n", "2.45.0"},
		{"git version 2.39.3 (Apple Git-145)\n", "2.39.3"},
		{"git version 2.45.0.windows.1\n", "2.45.0"},
		{"git version 2.39.0 (Apple Git-145)\r\n", "2.39.0"},
		{"git version 2.0.0\n", "2.0.0"},
		{"git version 10.20.30\n", "10.20.30"},
		{"", ""},
		{"git version abc\n", ""},
		{"git version 2.45\n", ""},
		{"git version 2.45.x\n", ""},
	}
	for _, c := range cases {
		got := parseGitVersion(c.input)
		if got != c.want {
			t.Errorf("parseGitVersion(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	cases := []struct {
		input string
		want  [3]int
		ok    bool
	}{
		{"2.45.0", [3]int{2, 45, 0}, true},
		{"v2.45.0", [3]int{2, 45, 0}, true},
		{"1.2.3", [3]int{1, 2, 3}, true},
		{"0.0.0", [3]int{0, 0, 0}, true},
		{"10.20.30", [3]int{10, 20, 30}, true},
		{"invalid", [3]int{}, false},
		{"2.45", [3]int{}, false},
		{"2.45.x", [3]int{}, false},
		{"", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parseSemver(c.input)
		if ok != c.ok {
			t.Errorf("parseSemver(%q) ok = %v, want %v", c.input, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseSemver(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2.46.0", "2.45.0", true},
		{"2.45.1", "2.45.0", true},
		{"3.0.0", "2.99.99", true},
		{"2.45.0", "2.45.0", false},
		{"2.44.0", "2.45.0", false},
		{"2.45.0", "2.45.1", false},
		{"1.0.0", "2.0.0", false},
		{"invalid", "2.45.0", false},
		{"2.45.0", "invalid", false},
	}
	for _, c := range cases {
		if got := isNewer(c.a, c.b); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestInstallHintNotEmpty(t *testing.T) {
	if installHint() == "" {
		t.Error("installHint() returned empty string")
	}
}

func TestUpgradeCmdNotEmpty(t *testing.T) {
	if upgradeCmd() == "" {
		t.Error("upgradeCmd() returned empty string")
	}
}
