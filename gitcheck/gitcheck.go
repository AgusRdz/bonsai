package gitcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

// EnsureInstalled checks that git is present in PATH.
// Prints an install hint and exits if git is not found.
func EnsureInstalled() {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: git is not installed\n\n%s\n", installHint())
		os.Exit(1)
	}
}

// SuggestUpdate checks if a newer version of git is available and prints a
// one-line suggestion if so. Silent on any error. Call only in interactive mode
// to avoid the network round-trip for subcommands.
func SuggestUpdate() {
	current, err := currentVersion()
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		return
	}

	if isNewer(latest, current) {
		fmt.Printf("bonsai: git %s is available (you have %s) - update with: %s\n",
			latest, current, upgradeCmd())
	}
}

func currentVersion() (string, error) {
	out, err := exec.Command("git", "version").Output()
	if err != nil {
		return "", err
	}
	v := parseGitVersion(string(out))
	if v == "" {
		return "", fmt.Errorf("could not parse git version output: %q", string(out))
	}
	return v, nil
}

// parseGitVersion extracts "X.Y.Z" from `git version` output.
// Handles platform-specific suffixes:
//   - macOS: "git version 2.39.3 (Apple Git-145)"
//   - Windows: "git version 2.45.0.windows.1"
//   - Linux: "git version 2.45.0"
func parseGitVersion(output string) string {
	s := strings.TrimSpace(output)
	s = strings.TrimPrefix(s, "git version ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	// Take the first three numeric dot-separated components.
	parts := strings.Split(fields[0], ".")
	var nums []string
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			break
		}
		nums = append(nums, p)
		if len(nums) == 3 {
			break
		}
	}
	if len(nums) < 3 {
		return ""
	}
	return strings.Join(nums, ".")
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/git/git/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&release); err != nil {
		return "", err
	}

	v := strings.TrimPrefix(release.TagName, "v")
	if v == "" {
		return "", fmt.Errorf("empty tag_name in GitHub response")
	}
	return v, nil
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "  brew install git\n  xcode-select --install"
	case "windows":
		return "  winget install Git.Git\n  download from https://git-scm.com/download/win"
	default:
		return "  apt install git    # Debian/Ubuntu\n  yum install git    # RHEL/CentOS\n  pacman -S git      # Arch"
	}
}

func upgradeCmd() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew upgrade git"
	case "windows":
		return "winget upgrade Git.Git"
	default:
		return "your package manager (e.g. apt upgrade git)"
	}
}

func isNewer(a, b string) bool {
	pa, ok1 := parseSemver(a)
	pb, ok2 := parseSemver(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range pa {
		if pa[i] > pb[i] {
			return true
		}
		if pa[i] < pb[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		nums[i] = n
	}
	return nums, true
}
