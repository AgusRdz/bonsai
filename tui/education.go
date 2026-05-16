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
	case strings.HasPrefix(cmd, "git commit"):
		return "Commit created"
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
		return "Stash applied"
	case strings.HasPrefix(cmd, "git stash"):
		return "Changes stashed"
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
	case strings.HasPrefix(cmd, "git restore"):
		return "The working tree changes for that file were permanently discarded. " +
			"Git cannot recover them - the file now matches the last commit. " +
			"If you need the changes back, check your editor's local history or undo buffer."
	case strings.HasPrefix(cmd, "git commit"):
		return "A new commit was created in the current branch. A commit is a permanent snapshot of your " +
			"staged changes. It lives in the branch history and can always be recovered with git log."
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
	default:
		return ""
	}
}
