package tui

import (
	"fmt"
	"testing"
)

func TestCommandKey(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"git add -- file.go", "add"},
		{"git restore --staged -- file.go", "unstage"},
		{"git restore -- file.go", "restore"},
		{"git restore --source HEAD -- file.go", "restore"},
		{"git commit --amend", "amend"},
		{"git commit --amend --author=foo", "amend"},
		{"git commit -m msg", "commit"},
		{"git push origin --delete feat/x", "remote-branch-delete"},
		{"git push", "push"},
		{"git push --force-with-lease", "push"},
		{"git pull", "pull"},
		{"git switch -c feat/x", "branch"},
		{"git switch main", "switch"},
		{"git branch -m new-name", "branch-rename"},
		{"git stash pop", "stash-pop"},
		{"git stash apply stash@{0}", "stash-apply"},
		{"git stash drop stash@{0}", "stash-drop"},
		{"git stash", "stash"},
		{"git branch -d feat/old", "branch-delete"},
		{"git branch -D feat/old", "branch-delete"},
		{"git rebase main", "rebase"},
		{"git rebase --continue", "rebase"},
		{"git rebase --abort", "rebase"},
		{"git merge feat/x", "merge"},
		{"git merge --abort", "merge"},
		{"git cherry-pick abc123", "cherry-pick"},
		{"git cherry-pick --abort", "cherry-pick"},
		{"git reset --soft HEAD~1", "reset-soft"},
		{"git reset --mixed HEAD~1", "reset-mixed"},
		{"git reset --hard HEAD~1", "reset-hard"},
		{"git tag v1.0.0", "tag"},
		{"git tag -d v1.0.0", "tag"},
		{"git worktree add ../path branch", "worktree"},
		{"git worktree remove ../path", "worktree"},
		{"git fetch origin", "fetch"},
		{"git fetch --all --prune", "fetch"},
		{"git clean -fd", "clean"},
		{"git remote add upstream url", "remote"},
		{"git remote remove upstream", "remote"},
		{"git submodule update --init", "submodule"},
		{"git notes add -m note HEAD", "notes"},
		{"git apply 0001.patch", "apply"},
		{"git unknown-command", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := commandKey(c.cmd)
		if got != c.want {
			t.Errorf("commandKey(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestCommandKeyPushDeleteBeforeGenericPush(t *testing.T) {
	if got := commandKey("git push origin --delete feat/x"); got != "remote-branch-delete" {
		t.Errorf("commandKey(push --delete) = %q, want remote-branch-delete", got)
	}
	if got := commandKey("git push"); got != "push" {
		t.Errorf("commandKey(push) = %q, want push", got)
	}
}

func TestCommandKeyAmendBeforeGenericCommit(t *testing.T) {
	if got := commandKey("git commit --amend"); got != "amend" {
		t.Errorf("commandKey(commit --amend) = %q, want amend", got)
	}
	if got := commandKey("git commit -m msg"); got != "commit" {
		t.Errorf("commandKey(commit -m) = %q, want commit", got)
	}
}

func TestActionTitle(t *testing.T) {
	cases := []struct {
		cmd  string
		err  error
		want string
	}{
		{"git add -- file.go", nil, "File staged"},
		{"git restore --staged -- file.go", nil, "File unstaged"},
		{"git commit -m msg", nil, "Commit created"},
		{"git push", nil, "Changes pushed"},
		{"git push origin --delete feat/login", nil, "Remote branch deleted"},
		{"git pull", nil, "Changes pulled"},
		{"git switch -c feat/x", nil, "Branch created"},
		{"git switch main", nil, "Branch switched"},
		{"git stash pop", nil, "Stash popped"},
		{"git stash apply", nil, "Stash applied"},
		{"git stash drop", nil, "Stash dropped"},
		{"git branch -d feat/old", nil, "Branch deleted"},
		{"git branch -D feat/old", nil, "Branch deleted"},
		{"git push", fmt.Errorf("rejected"), "Action failed"},
		{"git unknown", nil, "Done"},
	}
	for _, c := range cases {
		got := actionTitle(c.cmd, c.err)
		if got != c.want {
			t.Errorf("actionTitle(%q, %v) = %q, want %q", c.cmd, c.err, got, c.want)
		}
	}
}

func TestExplainReturnsContentForKnownCommands(t *testing.T) {
	cmds := []string{
		"git add -- file.go",
		"git restore --staged -- file.go",
		"git restore --source HEAD -- file.go",
		"git restore -- file.go",
		"git commit -m msg",
		"git commit --amend",
		"git commit --amend --author=foo",
		"git commit --amend --date=now",
		"git push",
		"git push origin --delete feat/login",
		"git pull",
		"git switch -c feat/x",
		"git switch main",
		"git branch -m new-name",
		"git stash",
		"git stash pop",
		"git stash apply stash@{0}",
		"git stash drop stash@{0}",
		"git rebase main",
		"git rebase --continue",
		"git rebase --abort",
		"git merge feat/x",
		"git merge --abort",
		"git cherry-pick abc123",
		"git cherry-pick --abort",
		"git reset --soft HEAD~1",
		"git reset --mixed HEAD~1",
		"git reset --hard HEAD~1",
		"git tag v1.0.0",
		"git tag -d v1.0.0",
		"git worktree add ../hotfix hotfix/fix",
		"git worktree remove ../hotfix",
		"git fetch origin",
		"git fetch --all",
		"git fetch --prune",
		"git fetch --all --prune",
		"git clean -fd",
		"git remote add upstream https://github.com/org/repo.git",
		"git remote remove upstream",
		"git remote rename origin upstream",
		"git submodule add https://github.com/org/lib.git lib",
		"git submodule update",
		"git submodule update --init",
		"git submodule deinit lib",
		"git notes add -m note HEAD",
		"git notes remove HEAD",
		"git branch -d feat/old",
		"git branch -D feat/old",
	}
	for _, cmd := range cmds {
		got := explain(cmd, nil)
		if got == "" {
			t.Errorf("explain(%q, nil) returned empty string", cmd)
		}
	}
}

func TestExplainPushDeleteBeforeGenericPush(t *testing.T) {
	got := explain("git push origin --delete feat/login", nil)
	if got == "" {
		t.Fatal("explain returned empty for push --delete")
	}
	generic := explain("git push", nil)
	if got == generic {
		t.Error("push --delete returned same text as generic push - ordering bug")
	}
}

func TestExplainRestoreSourceBeforeGenericRestore(t *testing.T) {
	got := explain("git restore --source HEAD -- file.go", nil)
	if got == "" {
		t.Fatal("explain returned empty for restore --source")
	}
	generic := explain("git restore -- file.go", nil)
	if got == generic {
		t.Error("restore --source returned same text as generic restore - ordering bug")
	}
}

func TestExplainErrorCase(t *testing.T) {
	got := explain("git push", fmt.Errorf("rejected"))
	if got == "" {
		t.Error("explain with error returned empty string")
	}
}

func TestNewEduPanel(t *testing.T) {
	p := newEduPanel("git commit -m feat", nil)
	if !p.success {
		t.Error("expected success=true")
	}
	if p.title != "Commit created" {
		t.Errorf("title = %q, want Commit created", p.title)
	}
	if p.cmd != "git commit -m feat" {
		t.Errorf("cmd = %q", p.cmd)
	}
	if p.explain == "" {
		t.Error("explain should not be empty")
	}
}

func TestNewEduPanelError(t *testing.T) {
	p := newEduPanel("git push", fmt.Errorf("rejected"))
	if p.success {
		t.Error("expected success=false")
	}
	if p.title != "Action failed" {
		t.Errorf("title = %q, want Action failed", p.title)
	}
}
