package doctor

import "testing"

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
