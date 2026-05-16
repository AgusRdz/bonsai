package doctor

import (
	"testing"
)

func TestParseSSHHost(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// SCP-like
		{"git@github.com:user/repo.git", "github.com"},
		{"git@gitlab.com:org/project.git", "gitlab.com"},
		{"git@bitbucket.org:user/repo.git", "bitbucket.org"},
		{"git@gitea.example.com:user/repo.git", "gitea.example.com"},
		// ssh:// with user
		{"ssh://git@github.com/user/repo.git", "github.com"},
		{"ssh://git@gitlab.com/org/project.git", "gitlab.com"},
		// ssh:// without user
		{"ssh://github.com/user/repo.git", "github.com"},
		// ssh:// with port
		{"ssh://git@github.com:22/user/repo.git", "github.com"},
		// HTTPS - should return empty
		{"https://github.com/user/repo.git", ""},
		{"http://github.com/user/repo.git", ""},
		// Local path - should return empty
		{"/home/user/repo.git", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := ParseSSHHost(c.input)
		if got != c.want {
			t.Errorf("ParseSSHHost(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSSHKeyURL(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"github.com", "github.com/settings/keys"},
		{"gitlab.com", "gitlab.com/-/profile/keys"},
		{"bitbucket.org", "bitbucket.org/account/settings/ssh-keys/"},
		{"ssh.dev.azure.com", "dev.azure.com/<org>/_usersSettings/keys"},
		{"gitea.example.com", "gitea.example.com (check your forge's SSH key settings)"},
	}
	for _, c := range cases {
		got := SSHKeyURL(c.host)
		if got != c.want {
			t.Errorf("SSHKeyURL(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}

func TestIsGitVersionSupported(t *testing.T) {
	cases := []struct {
		ver  string
		want bool
	}{
		{"2.39.0", true},
		{"2.28.0", true},
		{"2.28.1", true},
		{"2.27.9", false},
		{"2.0.0", false},
		{"3.0.0", true},
		{"1.9.5", false},
	}
	for _, c := range cases {
		got := isGitVersionSupported(c.ver)
		if got != c.want {
			t.Errorf("isGitVersionSupported(%q) = %v, want %v", c.ver, got, c.want)
		}
	}
}

func TestParseSizePack(t *testing.T) {
	cases := []struct {
		input string
		kb    int64
		ok    bool
	}{
		{"count: 100\nsize: 500\nsize-pack: 2048\nin-pack: 50\n", 2048, true},
		{"count: 1\nsize: 10\n", 0, false},
		{"", 0, false},
		{"size-pack: 150000\n", 150000, true},
		{"size-pack:\n", 0, false},
	}
	for _, c := range cases {
		kb, ok := parseSizePack(c.input)
		if ok != c.ok {
			t.Errorf("parseSizePack(%q) ok=%v, want %v", c.input, ok, c.ok)
			continue
		}
		if ok && kb != c.kb {
			t.Errorf("parseSizePack(%q) kb=%d, want %d", c.input, kb, c.kb)
		}
	}
}
