package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type eduTickMsg struct{}

type educationPanel struct {
	success bool
	title   string
	cmd     string
	explain string
}

func startEduTimer() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return eduTickMsg{}
	})
}

func newEduPanel(cmd string, err error) *educationPanel {
	return &educationPanel{
		success: err == nil,
		title:   actionTitle(cmd, err),
		cmd:     cmd,
		explain: explain(cmd, err),
	}
}

// commandKey maps a raw git command string to a canonical short key used for
// usage tracking. Returns "" for commands we do not track.
func commandKey(cmd string) string {
	switch {
	case strings.HasPrefix(cmd, "git add"):
		return "add"
	case strings.HasPrefix(cmd, "git restore --staged"):
		return "unstage"
	case strings.HasPrefix(cmd, "git restore"):
		return "restore"
	case strings.HasPrefix(cmd, "git commit --amend"):
		return "amend"
	case strings.HasPrefix(cmd, "git commit"):
		return "commit"
	case strings.HasPrefix(cmd, "git push") && strings.Contains(cmd, "--delete"):
		return "remote-branch-delete"
	case strings.HasPrefix(cmd, "git push"):
		return "push"
	case strings.HasPrefix(cmd, "git pull"):
		return "pull"
	case strings.HasPrefix(cmd, "git switch -c"):
		return "branch"
	case strings.HasPrefix(cmd, "git switch"):
		return "switch"
	case strings.HasPrefix(cmd, "git branch -m"):
		return "branch-rename"
	case strings.HasPrefix(cmd, "git stash pop"):
		return "stash-pop"
	case strings.HasPrefix(cmd, "git stash apply"):
		return "stash-apply"
	case strings.HasPrefix(cmd, "git stash drop"):
		return "stash-drop"
	case strings.HasPrefix(cmd, "git stash"):
		return "stash"
	case strings.HasPrefix(cmd, "git branch -d"), strings.HasPrefix(cmd, "git branch -D"):
		return "branch-delete"
	case strings.HasPrefix(cmd, "git rebase"):
		return "rebase"
	case strings.HasPrefix(cmd, "git merge"):
		return "merge"
	case strings.HasPrefix(cmd, "git cherry-pick"):
		return "cherry-pick"
	case strings.HasPrefix(cmd, "git revert --abort"):
		return "revert"
	case strings.HasPrefix(cmd, "git revert --continue"):
		return "revert"
	case strings.HasPrefix(cmd, "git revert"):
		return "revert"
	case strings.HasPrefix(cmd, "git reset --soft"):
		return "reset-soft"
	case strings.HasPrefix(cmd, "git reset --mixed"):
		return "reset-mixed"
	case strings.HasPrefix(cmd, "git reset --hard"):
		return "reset-hard"
	case strings.HasPrefix(cmd, "git tag"):
		return "tag"
	case strings.HasPrefix(cmd, "git worktree"):
		return "worktree"
	case strings.HasPrefix(cmd, "git fetch"):
		return "fetch"
	case strings.HasPrefix(cmd, "git clean"):
		return "clean"
	case strings.HasPrefix(cmd, "git remote"):
		return "remote"
	case strings.HasPrefix(cmd, "git submodule"):
		return "submodule"
	case strings.HasPrefix(cmd, "git notes"):
		return "notes"
	case strings.HasPrefix(cmd, "git apply"):
		return "apply"
	default:
		return ""
	}
}

// masteryThresholds defines how many successful uses are needed before the
// user is considered familiar with a command.
var masteryThresholds = map[string]int{
	"add":                  20,
	"unstage":              20,
	"commit":               20,
	"push":                 15,
	"pull":                 15,
	"branch":               12,
	"switch":               12,
	"stash":                10,
	"stash-pop":            10,
	"fetch":                10,
	"merge":                8,
	"rebase":               8,
	"restore":              8,
	"amend":                8,
	"cherry-pick":          8,
	"revert":               8,
	"reset-soft":           6,
	"reset-mixed":          6,
	"reset-hard":           6,
	"apply":                5,
	"branch-rename":        5,
	"branch-delete":        8,
	"remote-branch-delete": 5,
	"stash-apply":          8,
	"stash-drop":           5,
	"tag":                  5,
	"worktree":             5,
	"clean":                5,
	"remote":               5,
	"submodule":            5,
	"notes":                5,
}

// masteryThreshold returns the number of uses needed to master a command.
func masteryThreshold(key string) int {
	if t, ok := masteryThresholds[key]; ok {
		return t
	}
	return 10
}

// proComplexCommands is the set of commands that are shown in pro mode for
// users who have rarely run them. Day-to-day commands (add, commit, push...)
// are omitted because a pro user does not need a reminder for those.
var proComplexCommands = map[string]bool{
	"rebase":               true,
	"cherry-pick":          true,
	"revert":               true,
	"amend":                true,
	"restore":              true,
	"reset-soft":           true,
	"reset-mixed":          true,
	"reset-hard":           true,
	"apply":                true,
	"worktree":             true,
	"submodule":            true,
	"notes":                true,
	"remote":               true,
	"clean":                true,
	"branch-rename":        true,
	"branch-delete":        true,
	"remote-branch-delete": true,
}

// isProComplex reports whether a command key should show education in pro mode
// when the user has rarely run it.
func isProComplex(key string) bool {
	return proComplexCommands[key]
}

func actionTitle(cmd string, err error) string {
	if err != nil {
		return "Action failed"
	}
	switch {
	case strings.HasPrefix(cmd, "git add"):
		return "File staged"
	case strings.HasPrefix(cmd, "git restore --staged"):
		return "File unstaged"
	case strings.HasPrefix(cmd, "git restore"):
		return "Changes discarded"
	case strings.HasPrefix(cmd, "git commit --amend --author"):
		return "Author updated"
	case strings.HasPrefix(cmd, "git commit --amend --date"):
		return "Date updated"
	case strings.HasPrefix(cmd, "git commit --amend"):
		return "Commit amended"
	case strings.HasPrefix(cmd, "git commit"):
		return "Commit created"
	case strings.HasPrefix(cmd, "git push") && strings.Contains(cmd, "--delete"):
		return "Remote branch deleted"
	case strings.HasPrefix(cmd, "git push"):
		return "Changes pushed"
	case strings.HasPrefix(cmd, "git pull"):
		return "Changes pulled"
	case strings.HasPrefix(cmd, "git switch -c"):
		return "Branch created"
	case strings.HasPrefix(cmd, "git switch"):
		return "Branch switched"
	case strings.HasPrefix(cmd, "git branch -m"):
		return "Branch renamed"
	case strings.HasPrefix(cmd, "git stash pop"):
		return "Stash popped"
	case strings.HasPrefix(cmd, "git stash apply"):
		return "Stash applied"
	case strings.HasPrefix(cmd, "git stash drop"):
		return "Stash dropped"
	case strings.HasPrefix(cmd, "git stash"):
		return "Changes stashed"
	case strings.HasPrefix(cmd, "git branch -d"), strings.HasPrefix(cmd, "git branch -D"):
		return "Branch deleted"
	case strings.HasPrefix(cmd, "git rebase --continue"):
		return "Rebase continued"
	case strings.HasPrefix(cmd, "git rebase --abort"):
		return "Rebase aborted"
	case strings.HasPrefix(cmd, "git rebase"):
		return "Rebase started"
	case strings.HasPrefix(cmd, "git merge --abort"):
		return "Merge aborted"
	case strings.HasPrefix(cmd, "git merge"):
		return "Branch merged"
	case strings.HasPrefix(cmd, "git cherry-pick --abort"):
		return "Cherry-pick aborted"
	case strings.HasPrefix(cmd, "git cherry-pick"):
		return "Commit cherry-picked"
	case strings.HasPrefix(cmd, "git revert --abort"):
		return "Revert aborted"
	case strings.HasPrefix(cmd, "git revert --continue"):
		return "Revert continued"
	case strings.HasPrefix(cmd, "git revert"):
		return "Commit reverted"
	case strings.HasPrefix(cmd, "git reset --soft"):
		return "Soft reset done"
	case strings.HasPrefix(cmd, "git reset --mixed"):
		return "Mixed reset done"
	case strings.HasPrefix(cmd, "git reset --hard"):
		return "Hard reset done"
	case strings.HasPrefix(cmd, "git tag -d"):
		return "Tag deleted"
	case strings.HasPrefix(cmd, "git tag"):
		return "Tag created"
	case strings.HasPrefix(cmd, "git worktree add"):
		return "Worktree created"
	case strings.HasPrefix(cmd, "git worktree remove"):
		return "Worktree removed"
	default:
		return "Done"
	}
}

func explain(cmd string, err error) string {
	if err != nil {
		return "The command did not complete successfully. Check the error above and try again."
	}
	switch {
	case strings.HasPrefix(cmd, "git add"):
		return "The file moved to the staging area (index). It will be included in your next commit. " +
			"Nothing has been saved permanently yet - that happens when you commit."
	case strings.HasPrefix(cmd, "git restore --staged"):
		return "The file was removed from the staging area. Your changes are still in the working tree - " +
			"they are not lost. Stage it again when you are ready to include it in a commit."
	case strings.HasPrefix(cmd, "git restore --source"):
		return "The file was restored to the state it had at the given ref. " +
			"The change appears as a modification in your working tree - you still need to stage and commit it."
	case strings.HasPrefix(cmd, "git restore"):
		return "The working tree changes for that file were permanently discarded. " +
			"Git cannot recover them - the file now matches the last commit. " +
			"If you need the changes back, check your editor's local history or undo buffer."
	case strings.HasPrefix(cmd, "git commit --amend --author"):
		return "The author of the last commit was rewritten. " +
			"Amend rewrites history - avoid amending commits that have already been pushed to a shared remote, " +
			"as it will require a force-push and may disrupt others."
	case strings.HasPrefix(cmd, "git commit --amend --date"):
		return "The date of the last commit was rewritten. " +
			"Amend rewrites history - avoid amending commits that have already been pushed to a shared remote, " +
			"as it will require a force-push and may disrupt others."
	case strings.HasPrefix(cmd, "git commit --amend"):
		return "The last commit was rewritten with the new content. " +
			"Amend rewrites history - avoid amending commits that have already been pushed to a shared remote, " +
			"as it will require a force-push and may disrupt others."
	case strings.HasPrefix(cmd, "git commit"):
		return "A new commit was created in the current branch. A commit is a permanent snapshot of your " +
			"staged changes. It lives in the branch history and can always be recovered with git log."
	case strings.HasPrefix(cmd, "git push") && strings.Contains(cmd, "--delete"):
		return "The branch was deleted from the remote. " +
			"The local branch still exists - delete it separately with 'git branch -d <branch>' if you no longer need it."
	case strings.HasPrefix(cmd, "git push"):
		return "Your local commits were sent to the remote repository. " +
			"Others can now see and pull your changes. The remote branch is now in sync."
	case strings.HasPrefix(cmd, "git pull"):
		return "Commits from the remote were downloaded and merged into your current branch. " +
			"Your local branch is now up to date with the remote."
	case strings.HasPrefix(cmd, "git switch -c"):
		return "A new branch was created and you are now on it. " +
			"The branch starts from the same commit you were on. Your previous branch is unchanged."
	case strings.HasPrefix(cmd, "git switch"):
		return "You switched branches. The working tree now reflects the latest commit on the new branch. " +
			"Uncommitted changes travel with you unless they conflict."
	case strings.HasPrefix(cmd, "git branch -m"):
		return "The current branch was renamed locally. If this branch was already pushed to the remote, " +
			"push the new name (git push -u origin <new-name>) and delete the old remote branch."
	case strings.HasPrefix(cmd, "git stash pop"):
		return "The stash was applied to your working tree and removed from the stash list. " +
			"If there are conflicts you will need to resolve them manually before committing."
	case strings.HasPrefix(cmd, "git stash apply"):
		return "The stash was applied to your working tree but kept in the stash list. " +
			"Use apply instead of pop when you want to apply the same stash to multiple branches."
	case strings.HasPrefix(cmd, "git stash drop"):
		return "The stash entry was permanently removed from the stash list. " +
			"This cannot be undone - use drop only when you are sure you no longer need those changes."
	case strings.HasPrefix(cmd, "git stash"):
		return "Your uncommitted changes were saved to the stash and the working tree was cleaned. " +
			"The stash is a temporary shelf - use [S] to view stashes and pop them when you are ready to continue."
	case strings.HasPrefix(cmd, "git rebase --continue"):
		return "The rebase resumed after you resolved conflicts and staged the changes. " +
			"Git will continue replaying the remaining commits onto the target branch."
	case strings.HasPrefix(cmd, "git rebase --abort"):
		return "The rebase was cancelled and your branch was restored to its state before the rebase began. " +
			"No commits were changed."
	case strings.HasPrefix(cmd, "git rebase"):
		return "Your commits are being replayed on top of the target branch. " +
			"Rebase rewrites history by moving your commits to a new base, creating a linear history. " +
			"If conflicts arise, resolve them and press [c] to continue, or [a] to abort."
	case strings.HasPrefix(cmd, "git merge --abort"):
		return "The merge was cancelled and your branch was restored to its pre-merge state. " +
			"All in-progress merge changes have been discarded."
	case strings.HasPrefix(cmd, "git merge"):
		return "The specified branch was merged into your current branch. " +
			"A merge commit is created unless the merge can be fast-forwarded. " +
			"If there are conflicts, resolve them and stage the files, then commit to complete the merge."
	case strings.HasPrefix(cmd, "git cherry-pick --abort"):
		return "The cherry-pick was cancelled and your branch was restored to its previous state. " +
			"The operation did not apply any commits."
	case strings.HasPrefix(cmd, "git cherry-pick"):
		return "The selected commit was applied on top of your current branch as a new commit. " +
			"Cherry-pick copies the diff from that commit - the original commit remains unchanged in its branch."
	case strings.HasPrefix(cmd, "git revert --abort"):
		return "The revert was cancelled and your branch was restored to its previous state. " +
			"No new commit was created and your working tree is clean."
	case strings.HasPrefix(cmd, "git revert --continue"):
		return "The revert resumed after you resolved the conflicts. " +
			"A new commit was created that undoes the changes from the original commit."
	case strings.HasPrefix(cmd, "git revert"):
		return "A new commit was created that undoes the changes introduced by the selected commit. " +
			"Unlike reset, revert is safe on shared branches because it adds a commit rather than rewriting history. " +
			"Use [U] to undo the revert commit if you change your mind."
	case strings.HasPrefix(cmd, "git reset --soft"):
		return "The last commit was removed from history but all its changes are still staged. " +
			"You can amend the commit message or add more files before committing again."
	case strings.HasPrefix(cmd, "git reset --mixed"):
		return "The last commit was removed from history and its changes moved back to the working tree (unstaged). " +
			"You can review and selectively re-stage files before creating a new commit."
	case strings.HasPrefix(cmd, "git reset --hard"):
		return "The last commit was permanently removed along with all its changes. " +
			"Hard reset cannot be undone through normal git commands - the changes are gone."
	case strings.HasPrefix(cmd, "git tag -d"):
		return "The tag was deleted from your local repository. " +
			"If the tag was already pushed to a remote, delete it there too with: git push origin --delete <tag>."
	case strings.HasPrefix(cmd, "git tag"):
		return "A lightweight tag was created at the current HEAD commit. " +
			"Tags mark specific points in history, commonly used for releases. " +
			"Push the tag to remote with: git push origin <tag>."
	case strings.HasPrefix(cmd, "git worktree add"):
		return "A linked worktree was created at the given path. " +
			"Worktrees let you check out a different branch in a separate directory without losing your current work. " +
			"Useful for reviewing a PR, running a hot-fix, or comparing versions side by side - all from the same repo."
	case strings.HasPrefix(cmd, "git worktree remove"):
		return "The linked worktree was removed. The branch it contained still exists in the repository. " +
			"Removing a worktree only deletes the working directory link - your commits are safe."
	case strings.HasPrefix(cmd, "git fetch --all --prune"):
		return "All remotes were fetched and stale remote-tracking refs (branches deleted on the server) were pruned. " +
			"Your local branches are untouched."
	case strings.HasPrefix(cmd, "git fetch --all"):
		return "All configured remotes were fetched. Use fetch --prune to also clean up deleted remote branches."
	case strings.HasPrefix(cmd, "git fetch --prune"):
		return "The remote was fetched and stale remote-tracking refs were pruned. " +
			"Remote-tracking refs for branches that no longer exist on the server have been removed."
	case strings.HasPrefix(cmd, "git fetch"):
		return "The remote was fetched - remote-tracking refs are updated but your local branches remain unchanged. " +
			"Use 'git merge' or 'git rebase' to integrate the fetched changes."
	case strings.HasPrefix(cmd, "git clean -fd"):
		return "Untracked files and directories were permanently removed from the working tree. " +
			"This action cannot be undone - untracked files are not in git history."
	case strings.HasPrefix(cmd, "git remote add"):
		return "A new remote was registered. You can now fetch, pull, and push to it. " +
			"Run 'git fetch <name>' to download its refs."
	case strings.HasPrefix(cmd, "git remote remove"):
		return "The remote was removed from your local config. " +
			"This only affects your local repository - the remote itself is unaffected."
	case strings.HasPrefix(cmd, "git remote rename"):
		return "The remote was renamed. Any branches tracking that remote were automatically updated to use the new name."
	case strings.HasPrefix(cmd, "git submodule add"):
		return "A new submodule was registered and cloned into the specified path. " +
			"The .gitmodules file and the submodule directory have been staged - commit them to record the submodule in your repo."
	case strings.HasPrefix(cmd, "git submodule update --init"):
		return "All submodules were initialised and updated to the commit recorded by the parent repo. " +
			"Run this after cloning a repo that contains submodules."
	case strings.HasPrefix(cmd, "git submodule update"):
		return "Submodules were checked out to the commit recorded by the parent repo. " +
			"If the parent has new commits pointing to newer submodule versions, those versions are now active."
	case strings.HasPrefix(cmd, "git submodule deinit"):
		return "The submodule's working directory was cleaned and its section removed from .git/config. " +
			"The .gitmodules entry still exists - remove it manually if you want to fully drop the submodule."
	case strings.HasPrefix(cmd, "git notes add"):
		return "A note was attached to the commit. Notes are stored in refs/notes/commits and are not shown by default " +
			"in 'git log' unless you pass --notes. They are not transferred with push/pull unless you configure the refspec."
	case strings.HasPrefix(cmd, "git notes remove"):
		return "The note was removed from the commit. " +
			"The commit itself is unchanged - notes are metadata stored separately from the commit object."
	case strings.HasPrefix(cmd, "git branch -d"), strings.HasPrefix(cmd, "git branch -D"):
		return "The local branch was deleted. " +
			"If the branch was pushed to a remote, it still exists there - use [D] in the branch list to delete it remotely."
	default:
		return ""
	}
}
