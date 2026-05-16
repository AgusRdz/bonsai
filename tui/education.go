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
	default:
		return ""
	}
}
