package git

import (
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	porcelain := "" +
		"M  staged-modified.go\n" +
		"A  staged-added.go\n" +
		" M unstaged-modified.go\n" +
		"MM both.go\n" +
		"?? untracked.go\n" +
		"D  staged-deleted.go\n"

	s := parseStatus("main", porcelain)

	if s.Branch != "main" {
		t.Errorf("branch = %q, want main", s.Branch)
	}

	wantStaged := []struct{ code, path string }{
		{"M ", "staged-modified.go"},
		{"A ", "staged-added.go"},
		{"MM", "both.go"},
		{"D ", "staged-deleted.go"},
	}
	if len(s.Staged) != len(wantStaged) {
		t.Fatalf("staged count = %d, want %d: %v", len(s.Staged), len(wantStaged), s.Staged)
	}
	for i, w := range wantStaged {
		if s.Staged[i].Code != w.code || s.Staged[i].Path != w.path {
			t.Errorf("staged[%d] = {%q,%q}, want {%q,%q}", i,
				s.Staged[i].Code, s.Staged[i].Path, w.code, w.path)
		}
	}

	wantChanged := []struct{ code, path string }{
		{" M", "unstaged-modified.go"},
		{"MM", "both.go"},
	}
	if len(s.Changed) != len(wantChanged) {
		t.Fatalf("changed count = %d, want %d: %v", len(s.Changed), len(wantChanged), s.Changed)
	}
	for i, w := range wantChanged {
		if s.Changed[i].Code != w.code || s.Changed[i].Path != w.path {
			t.Errorf("changed[%d] = {%q,%q}, want {%q,%q}", i,
				s.Changed[i].Code, s.Changed[i].Path, w.code, w.path)
		}
	}

	if len(s.Untracked) != 1 || s.Untracked[0].Path != "untracked.go" {
		t.Errorf("untracked = %v, want [{?? untracked.go}]", s.Untracked)
	}
}

func TestParseStatusQuotedPath(t *testing.T) {
	// git emits octal-escaped, double-quoted paths when core.quotePath=true
	// and the filename contains non-ASCII bytes (e.g. U+F03A → UTF-8 EF 80 BA).
	porcelain := "?? \"C\\357\\200\\272Tempe555da2_info.txt\"\n"
	s := parseStatus("main", porcelain)
	if len(s.Untracked) != 1 {
		t.Fatalf("untracked count = %d, want 1", len(s.Untracked))
	}
	want := "C\xef\x80\xbaTempe555da2_info.txt"
	if s.Untracked[0].Path != want {
		t.Errorf("path = %q, want %q", s.Untracked[0].Path, want)
	}
}

func TestGitUnquotePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Not quoted — pass through unchanged
		{"file.go", "file.go"},
		// Empty quotes
		{`""`, ""},
		// Plain ASCII inside quotes
		{`"hello.go"`, "hello.go"},
		// Octal sequences (UTF-8 bytes for U+F03A: EF 80 BA)
		{`"C\357\200\272Tempe555da2_info.txt"`, "C\xef\x80\xbaTempe555da2_info.txt"},
		// Standard escapes
		{`"\t\n\r\\\""`, "\t\n\r\\\""},
		// Mixed
		{`"foo\357bar"`, "foo\xefbar"},
	}
	for _, c := range cases {
		got := gitUnquotePath(c.in)
		if got != c.want {
			t.Errorf("gitUnquotePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseStatusEmpty(t *testing.T) {
	s := parseStatus("main", "")
	if len(s.Staged) != 0 || len(s.Changed) != 0 || len(s.Untracked) != 0 {
		t.Errorf("expected clean status, got %+v", s)
	}
}

func TestParseBranches(t *testing.T) {
	// Format: %(refname:short)\t%(HEAD)\t%(upstream:short)
	output := "main\t \t\nfeat/login\t*\torigin/feat/login\nfix/bug-123\t \t\n"
	branches := parseBranches(output)

	if len(branches) != 3 {
		t.Fatalf("branch count = %d, want 3", len(branches))
	}

	cases := []struct {
		name     string
		current  bool
		upstream string
	}{
		{"main", false, ""},
		{"feat/login", true, "origin/feat/login"},
		{"fix/bug-123", false, ""},
	}
	for i, c := range cases {
		if branches[i].Name != c.name {
			t.Errorf("branches[%d].Name = %q, want %q", i, branches[i].Name, c.name)
		}
		if branches[i].Current != c.current {
			t.Errorf("branches[%d].Current = %v, want %v", i, branches[i].Current, c.current)
		}
		if branches[i].Upstream != c.upstream {
			t.Errorf("branches[%d].Upstream = %q, want %q", i, branches[i].Upstream, c.upstream)
		}
	}
}

func TestParseBranchesEmpty(t *testing.T) {
	if branches := parseBranches(""); len(branches) != 0 {
		t.Errorf("expected empty, got %v", branches)
	}
}

func TestParseBranchLine(t *testing.T) {
	cases := []struct {
		input    string
		branch   string
		upstream string
		ahead    int
		behind   int
	}{
		{"main...origin/main [ahead 2, behind 1]", "main", "origin/main", 2, 1},
		{"main...origin/main [ahead 3]", "main", "origin/main", 3, 0},
		{"main...origin/main [behind 5]", "main", "origin/main", 0, 5},
		{"main...origin/main", "main", "origin/main", 0, 0},
		{"main", "main", "", 0, 0},
		{"No commits yet on feat/new", "feat/new", "", 0, 0},
		{"HEAD (no branch)", "HEAD", "", 0, 0},
	}
	for _, c := range cases {
		branch, upstream, ahead, behind := parseBranchLine(c.input)
		if branch != c.branch || upstream != c.upstream || ahead != c.ahead || behind != c.behind {
			t.Errorf("parseBranchLine(%q) = (%q, %q, %d, %d), want (%q, %q, %d, %d)",
				c.input, branch, upstream, ahead, behind, c.branch, c.upstream, c.ahead, c.behind)
		}
	}
}

func TestFileEntryHelpers(t *testing.T) {
	f := FileEntry{Code: "MM", Path: "file.go"}
	if f.StagedCode() != 'M' {
		t.Errorf("StagedCode = %q, want M", f.StagedCode())
	}
	if f.UnstagedCode() != 'M' {
		t.Errorf("UnstagedCode = %q, want M", f.UnstagedCode())
	}

	untracked := FileEntry{Code: "??", Path: "new.go"}
	if untracked.StagedCode() != '?' {
		t.Errorf("StagedCode = %q, want ?", untracked.StagedCode())
	}
}

func TestConflictDesc(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"UU", "both modified"},
		{"AA", "both added"},
		{"DD", "both deleted"},
		{"AU", "added by us"},
		{"UA", "added by them"},
		{"DU", "deleted by us"},
		{"UD", "deleted by them"},
		{"XY", "conflict"},
		{"", "conflict"},
	}
	for _, c := range cases {
		if got := ConflictDesc(c.code); got != c.want {
			t.Errorf("ConflictDesc(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestParseWorktrees(t *testing.T) {
	output := "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/project-hotfix\nHEAD def456\nbranch refs/heads/hotfix/fix\n\nworktree /home/user/project-detached\nHEAD ghi789\ndetached\n"
	entries := parseWorktrees(output)

	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}

	cases := []struct {
		path    string
		branch  string
		current bool
	}{
		{"/home/user/project", "main", true},
		{"/home/user/project-hotfix", "hotfix/fix", false},
		{"/home/user/project-detached", "(detached)", false},
	}
	for i, c := range cases {
		if entries[i].Path != c.path {
			t.Errorf("entries[%d].Path = %q, want %q", i, entries[i].Path, c.path)
		}
		if entries[i].Branch != c.branch {
			t.Errorf("entries[%d].Branch = %q, want %q", i, entries[i].Branch, c.branch)
		}
		if entries[i].Current != c.current {
			t.Errorf("entries[%d].Current = %v, want %v", i, entries[i].Current, c.current)
		}
	}
}

func TestParseWorktreesEmpty(t *testing.T) {
	if entries := parseWorktrees(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseStashList(t *testing.T) {
	// legacy plain format (fallback)
	output := "stash@{0}: On main: WIP on login flow\nstash@{1}: On feat/x: half-done refactor\n"
	entries := parseStashList(output)

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	cases := []struct {
		ref  string
		desc string
	}{
		{"stash@{0}", "On main: WIP on login flow"},
		{"stash@{1}", "On feat/x: half-done refactor"},
	}
	for i, c := range cases {
		if entries[i].Ref != c.ref {
			t.Errorf("entries[%d].Ref = %q, want %q", i, entries[i].Ref, c.ref)
		}
		if entries[i].Description != c.desc {
			t.Errorf("entries[%d].Description = %q, want %q", i, entries[i].Description, c.desc)
		}
	}
}

func TestParseStashListNulFormat(t *testing.T) {
	// new NUL-delimited format produced by --format=%gd%x00%ci%x00%s
	output := "stash@{0}\x002026-05-28 14:30:00 +0000\x00On main: WIP on login flow\nstash@{1}\x002026-05-25 09:00:00 +0000\x00On feat/x: half-done refactor\n"
	entries := parseStashList(output)

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if entries[0].Ref != "stash@{0}" {
		t.Errorf("entries[0].Ref = %q, want stash@{0}", entries[0].Ref)
	}
	if entries[0].Description != "On main: WIP on login flow" {
		t.Errorf("entries[0].Description = %q", entries[0].Description)
	}
	if entries[0].Date.IsZero() {
		t.Error("entries[0].Date should not be zero")
	}
	if entries[1].Ref != "stash@{1}" {
		t.Errorf("entries[1].Ref = %q, want stash@{1}", entries[1].Ref)
	}
	// Stale is set by StashList() via merge-base; parseStashList itself never sets it.
	if entries[0].Stale || entries[1].Stale {
		t.Error("parseStashList should not set Stale — that is StashList()'s responsibility")
	}
}

func TestParseStashListEmpty(t *testing.T) {
	if entries := parseStashList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseConfigList(t *testing.T) {
	output := "user.name=Jane Doe\nuser.email=jane@example.com\ncore.editor\npull.rebase=true\n"
	entries := parseConfigList(output)

	if len(entries) != 4 {
		t.Fatalf("entry count = %d, want 4", len(entries))
	}
	cases := []struct {
		key   string
		value string
	}{
		{"user.name", "Jane Doe"},
		{"user.email", "jane@example.com"},
		{"core.editor", ""},
		{"pull.rebase", "true"},
	}
	for i, c := range cases {
		if entries[i].Key != c.key {
			t.Errorf("entries[%d].Key = %q, want %q", i, entries[i].Key, c.key)
		}
		if entries[i].Value != c.value {
			t.Errorf("entries[%d].Value = %q, want %q", i, entries[i].Value, c.value)
		}
	}
}

func TestParseConfigListEmpty(t *testing.T) {
	if entries := parseConfigList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseReflogOutput(t *testing.T) {
	output := "abc1234\x1eHEAD@{0}\x1ecommit: add login\nabc5678\x1eHEAD@{1}\x1echeckout: moving from main to feat/x\nabc9012\x1eHEAD@{2}\x1ereset: moving to HEAD~1\n"
	entries := parseReflogOutput(output)

	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	cases := []struct {
		hash    string
		ref     string
		action  string
		subject string
	}{
		{"abc1234", "HEAD@{0}", "commit", "commit: add login"},
		{"abc5678", "HEAD@{1}", "checkout", "checkout: moving from main to feat/x"},
		{"abc9012", "HEAD@{2}", "reset", "reset: moving to HEAD~1"},
	}
	for i, c := range cases {
		if entries[i].Hash != c.hash {
			t.Errorf("entries[%d].Hash = %q, want %q", i, entries[i].Hash, c.hash)
		}
		if entries[i].Ref != c.ref {
			t.Errorf("entries[%d].Ref = %q, want %q", i, entries[i].Ref, c.ref)
		}
		if entries[i].Action != c.action {
			t.Errorf("entries[%d].Action = %q, want %q", i, entries[i].Action, c.action)
		}
		if entries[i].Subject != c.subject {
			t.Errorf("entries[%d].Subject = %q, want %q", i, entries[i].Subject, c.subject)
		}
	}
}

func TestParseReflogOutputEmpty(t *testing.T) {
	if entries := parseReflogOutput(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestParseReflogOutputSkipsMalformed(t *testing.T) {
	output := "nocolons\nabc1234\x1eHEAD@{0}\x1ecommit: ok\n"
	entries := parseReflogOutput(output)
	if len(entries) != 1 {
		t.Errorf("entry count = %d, want 1", len(entries))
	}
}

func TestParseRemoteList(t *testing.T) {
	output := "origin\tgit@github.com:org/repo.git (fetch)\norigin\tgit@github.com:org/repo.git (push)\nupstream\thttps://github.com/upstream/repo.git (fetch)\nupstream\thttps://github.com/upstream/repo.git (push)\n"
	entries := parseRemoteList(output)

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	cases := []struct {
		name     string
		fetchURL string
		pushURL  string
	}{
		{"origin", "git@github.com:org/repo.git", "git@github.com:org/repo.git"},
		{"upstream", "https://github.com/upstream/repo.git", "https://github.com/upstream/repo.git"},
	}
	for i, c := range cases {
		if entries[i].Name != c.name {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, c.name)
		}
		if entries[i].FetchURL != c.fetchURL {
			t.Errorf("entries[%d].FetchURL = %q, want %q", i, entries[i].FetchURL, c.fetchURL)
		}
		if entries[i].PushURL != c.pushURL {
			t.Errorf("entries[%d].PushURL = %q, want %q", i, entries[i].PushURL, c.pushURL)
		}
	}
}

func TestParseRemoteListDifferentFetchPush(t *testing.T) {
	output := "origin\tgit@github.com:org/repo.git (fetch)\norigin\thttps://github.com/org/repo.git (push)\n"
	entries := parseRemoteList(output)
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].FetchURL != "git@github.com:org/repo.git" {
		t.Errorf("FetchURL = %q", entries[0].FetchURL)
	}
	if entries[0].PushURL != "https://github.com/org/repo.git" {
		t.Errorf("PushURL = %q", entries[0].PushURL)
	}
}

func TestParseRemoteListEmpty(t *testing.T) {
	if entries := parseRemoteList(""); len(entries) != 0 {
		t.Errorf("expected empty, got %v", entries)
	}
}

func TestExtractCommitHash(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"abc1234 feat: add login", "abc1234"},
		{"* abc1234 feat: add login", "abc1234"},
		{"| * abc1234 fix: null pointer", "abc1234"},
		{"| \\", ""},
		{"|/", ""},
		{"", ""},
		{"notahash", ""},
		{"abcdef0 feat: seven valid hex chars", "abcdef0"},
		{"ABCDEF0 uppercase hex is not matched", ""},
	}
	for _, c := range cases {
		got := extractCommitHash(c.line)
		if got != c.want {
			t.Errorf("extractCommitHash(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestMostFrequentName(t *testing.T) {
	cases := []struct {
		names map[string]int
		want  string
	}{
		{map[string]int{"Alice": 5, "Bob": 3}, "Alice"},
		{map[string]int{"Alice": 2, "Bob": 2}, "Alice"}, // tie-break: alphabetical
		{map[string]int{"Bob": 2, "Alice": 2}, "Alice"}, // same tie, alphabetical wins
		{map[string]int{"Jane": 1}, "Jane"},
		{map[string]int{}, ""},
	}
	for _, c := range cases {
		got := mostFrequentName(c.names)
		if got != c.want {
			t.Errorf("mostFrequentName(%v) = %q, want %q", c.names, got, c.want)
		}
	}
}

func TestParseDiffOutput(t *testing.T) {
	raw := "diff --git a/foo.go b/foo.go\nindex abc..def 100644\n--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,4 @@\n context\n-removed\n+added\n+extra\n context2\n@@ -10,2 +11,2 @@\n other\n-old\n+new\n"
	header, hunks, err := parseDiffOutput(raw)
	if err != nil {
		t.Fatalf("parseDiffOutput error: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("hunk count = %d, want 2", len(hunks))
	}
	if !strings.HasPrefix(header, "diff --git") {
		t.Errorf("header = %q, want diff --git prefix", header)
	}
	if hunks[0].Header != "@@ -1,3 +1,4 @@" {
		t.Errorf("hunks[0].Header = %q", hunks[0].Header)
	}
	if len(hunks[0].Body) != 5 {
		t.Errorf("hunks[0].Body len = %d, want 5", len(hunks[0].Body))
	}
	if hunks[1].Header != "@@ -10,2 +11,2 @@" {
		t.Errorf("hunks[1].Header = %q", hunks[1].Header)
	}
}

func TestParseDiffOutputEmpty(t *testing.T) {
	header, hunks, err := parseDiffOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if header != "" || len(hunks) != 0 {
		t.Errorf("expected empty result, got header=%q hunks=%d", header, len(hunks))
	}
}

func TestParseDiffOutputNoHunks(t *testing.T) {
	raw := "diff --git a/foo.go b/foo.go\nnew file mode 100644\n"
	header, hunks, err := parseDiffOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("hunk count = %d, want 0", len(hunks))
	}
	if header == "" {
		t.Error("header should not be empty for header-only diff")
	}
}

func TestHunkRaw(t *testing.T) {
	h := Hunk{Header: "@@ -1,2 +1,3 @@", Body: []string{" context", "-old", "+new"}}
	got := h.raw()
	want := "@@ -1,2 +1,3 @@\n context\n-old\n+new\n"
	if got != want {
		t.Errorf("Hunk.raw() = %q, want %q", got, want)
	}
}

func TestIsAllHex(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"abc123", true},
		{"0123456789abcdef", true},
		{"", true},
		{"ABCDEF", false},
		{"abcdefg", false},
		{"abc 123", false},
		{"abcdef0", true},
	}
	for _, c := range cases {
		got := isAllHex(c.input)
		if got != c.want {
			t.Errorf("isAllHex(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseBlamePorcelain(t *testing.T) {
	raw := strings.Join([]string{
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2 1 1 1",
		"author Jane Doe",
		"author-mail <jane@example.com>",
		"author-time 1700000000",
		"author-tz +0000",
		"committer Jane Doe",
		"committer-mail <jane@example.com>",
		"committer-time 1700000000",
		"committer-tz +0000",
		"summary feat: add feature",
		"filename main.go",
		"\thello world",
		"b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3 2 2 1",
		"author Bob Smith",
		"author-mail <bob@example.com>",
		"author-time 1700001000",
		"author-tz +0000",
		"committer Bob Smith",
		"committer-mail <bob@example.com>",
		"committer-time 1700001000",
		"committer-tz +0000",
		"summary fix: bug",
		"filename main.go",
		"\tfoo bar",
	}, "\n")

	lines := parseBlamePorcelain(raw)
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if lines[0].Hash != "a1b2c3d4" {
		t.Errorf("lines[0].Hash = %q, want a1b2c3d4", lines[0].Hash)
	}
	if lines[0].Author != "Jane Doe" {
		t.Errorf("lines[0].Author = %q, want Jane Doe", lines[0].Author)
	}
	if lines[0].LineNum != 1 {
		t.Errorf("lines[0].LineNum = %d, want 1", lines[0].LineNum)
	}
	if lines[0].Text != "hello world" {
		t.Errorf("lines[0].Text = %q, want hello world", lines[0].Text)
	}
	if lines[0].Date != "2023-11-14" {
		t.Errorf("lines[0].Date = %q, want 2023-11-14", lines[0].Date)
	}
	if lines[1].Hash != "b2c3d4e5" {
		t.Errorf("lines[1].Hash = %q, want b2c3d4e5", lines[1].Hash)
	}
	if lines[1].Author != "Bob Smith" {
		t.Errorf("lines[1].Author = %q, want Bob Smith", lines[1].Author)
	}
}

func TestParseBlamePorcelainEmpty(t *testing.T) {
	if lines := parseBlamePorcelain(""); len(lines) != 0 {
		t.Errorf("empty input: got %d lines, want 0", len(lines))
	}
}

func TestParseBlamePorcelainSkipsMalformed(t *testing.T) {
	raw := "not-a-hash 1 1 1\nauthor Jane\n\thello"
	if lines := parseBlamePorcelain(raw); len(lines) != 0 {
		t.Errorf("malformed: got %d lines, want 0", len(lines))
	}
}

func TestParseDiffNumstat(t *testing.T) {
	output := "10\t5\tsrc/main.go\n3\t1\tsrc/auth.go\n0\t2\tdocs/README.md\n"
	files := parseDiffNumstat(output)

	if len(files) != 3 {
		t.Fatalf("file count = %d, want 3", len(files))
	}
	cases := []struct {
		path string
		add  int
		del  int
	}{
		{"src/main.go", 10, 5},
		{"src/auth.go", 3, 1},
		{"docs/README.md", 0, 2},
	}
	for i, c := range cases {
		if files[i].Path != c.path {
			t.Errorf("files[%d].Path = %q, want %q", i, files[i].Path, c.path)
		}
		if files[i].Additions != c.add {
			t.Errorf("files[%d].Additions = %d, want %d", i, files[i].Additions, c.add)
		}
		if files[i].Deletions != c.del {
			t.Errorf("files[%d].Deletions = %d, want %d", i, files[i].Deletions, c.del)
		}
	}
}

func TestParseDiffNumstatEmpty(t *testing.T) {
	if files := parseDiffNumstat(""); len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
	if files := parseDiffNumstat("   \n  \n"); len(files) != 0 {
		t.Errorf("expected empty for whitespace, got %v", files)
	}
}

func TestParseDiffNumstatTabPath(t *testing.T) {
	// Path contains a tab (unusual but valid in some renames)
	output := "5\t3\told.go => new.go\n"
	files := parseDiffNumstat(output)
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Path != "old.go => new.go" {
		t.Errorf("path = %q, want old.go => new.go", files[0].Path)
	}
	if files[0].Additions != 5 || files[0].Deletions != 3 {
		t.Errorf("counts = +%d -%d, want +5 -3", files[0].Additions, files[0].Deletions)
	}
}

func TestParseDiffNumstatSkipsMalformed(t *testing.T) {
	output := "notanumber\t5\tfile.go\nabc\tdef\tghi.go\n"
	files := parseDiffNumstat(output)
	// strconv.Atoi("notanumber") returns 0, not an error - entries are still produced
	// but with 0 additions. The function is lenient.
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Additions != 0 {
		t.Errorf("files[0].Additions = %d, want 0 for invalid number", files[0].Additions)
	}
}

func TestBuildPartialHunk(t *testing.T) {
	hunk := Hunk{
		Header: "@@ -5,4 +5,4 @@",
		Body: []string{
			" context",
			"-removed1",
			"-removed2",
			"+added1",
			"+added2",
		},
	}

	// Select only the first removal and first addition.
	// lineSel: context=true, removed1=true, removed2=false, added1=true, added2=false
	sel := []bool{true, true, false, true, false}
	got := buildPartialHunk(hunk, sel)
	if got == nil {
		t.Fatal("buildPartialHunk returned nil")
	}

	// oldCount = C + D = 1 + 2 = 3
	// newCount = C + (D-d) + a = 1 + 1 + 1 = 3
	wantHeader := "@@ -5,3 +5,3 @@"
	if got.Header != wantHeader {
		t.Errorf("header = %q, want %q", got.Header, wantHeader)
	}

	wantBody := []string{" context", "-removed1", " removed2", "+added1"}
	if len(got.Body) != len(wantBody) {
		t.Fatalf("body len = %d, want %d: %v", len(got.Body), len(wantBody), got.Body)
	}
	for i, line := range wantBody {
		if got.Body[i] != line {
			t.Errorf("body[%d] = %q, want %q", i, got.Body[i], line)
		}
	}

	// Select nothing: should return nil.
	nothingSel := []bool{true, false, false, false, false}
	if buildPartialHunk(hunk, nothingSel) != nil {
		t.Error("expected nil when no change lines selected")
	}
}
