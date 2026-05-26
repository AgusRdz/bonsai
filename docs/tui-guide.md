# TUI Guide

## Opening bonsai

```sh
cd your-repo
bonsai
```

bonsai must be run from inside a Git repository. If no global config exists it opens the setup wizard first.

## Layout

```
 main  ↑2  [trunk]  [mode:standard]        ← header

  Conflicts (1)
  !  src/auth.go          both modified     ← conflict section (shown first)

  Staged (2)
  M  src/api.go                             ← staged section
  A  src/middleware.go

  Changed (1)
  M  README.md                              ← changed section

  Untracked (1)
  ?  scratch.txt                            ← untracked section

  $ git commit -m "fix: auth token refresh"
  [space] stage/unstage  [c] commit  ...    ← command bar
```

The **header** shows your current branch, how many commits you are ahead/behind the remote, the active workflow flow, and the current mode.

The **file list** is divided into four sections in fixed order: conflicts (if any), staged, changed, untracked. Navigate with `↑`/`↓` or `j`/`k`.

The **command bar** at the bottom shows the most common keys. Press `?` for the full reference.

## Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `esc` | Close current panel / go back |
| `q` / `ctrl+c` | Quit |

## File operations

| Key | Action |
|-----|--------|
| `space` | Stage a changed/untracked file; unstage a staged file |
| `h` | Open the hunk stage panel - stage or unstage individual hunks within a file |
| `d` | View diff for the selected file |
| `H` | Open file history - every commit that touched this file |
| `e` | Open blame - who last changed each line |
| `x` | Discard all working tree changes to the selected file (confirm required) |
| `o` | Restore the selected file to HEAD or a specific ref |

### Hunk staging

Press `h` on any changed file to open the hunk panel. Each hunk shows its `@@ ... @@` header and a short preview. All hunks are selected by default - deselect the ones you want to leave out, then press `enter` to apply.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Move between hunks |
| `space` | Toggle selected / deselected |
| `a` | Select all hunks |
| `l` | Enter line mode for the focused hunk |
| `enter` | Apply selected hunks |
| `esc` | Cancel |

#### Line mode

Press `l` on a hunk to enter line mode. Each `+` and `-` line can be toggled individually - useful when a hunk mixes changes you want to stage with ones you don't. Unselected `-` lines are kept as context (not staged as removals); unselected `+` lines are dropped entirely.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Move between lines |
| `space` | Toggle individual line |
| `enter` | Apply selected lines |
| `esc` | Back to hunk list |

### File history

Press `H` on any file to see every commit that touched it. Press `enter` on a commit to open its full detail view.

### Branch graph

Press `g` to open the branch graph (`git log --graph --all`). Use `↑`/`↓` to scroll, `esc` to close.

## Committing

Press `c` to open the commit panel. Type your message and press `enter` to commit. Press `esc` to cancel.

In **guided** mode the education panel appears after the commit and explains what happened in plain language along with the exact command that ran.

## Pushing and pulling

Press `p` to open the **push menu** instead of pushing immediately. Choose between:

| Option | Command | When to use |
|--------|---------|-------------|
| Push | `git push` | Normal push |
| Force with lease | `git push --force-with-lease` | After a rebase; fails safely if the remote has new commits |
| Set upstream | `git push --set-upstream origin <branch>` | First push of a new branch |

Press `P` to pull, `f` to open the fetch menu (origin / all / prune).

## Branches

| Key | Action |
|-----|--------|
| `b` | Create a new branch (or open the flow picker in gitflow mode) |
| `B` | List all branches - switch, merge, rebase, delete, rename, delete remote |

The branch list shows status badges next to each branch name:

| Badge | Colour | Meaning |
|-------|--------|---------|
| `↑↓ synced` | green | Tracking ref exists and is in sync |
| `↑N` / `↓N` | green / red | Commits ahead or behind remote |
| `gone` | red | Remote tracking ref was deleted (branch can be pruned) |
| `merged` | purple | Already merged into the default branch |
| `(protected)` | red | Protected on the remote — cannot be force-pushed |

When the list is taller than the terminal the title shows your scroll position (e.g. `↑ 5/36 ↓`). Use `↑`/`↓` (or `k`/`j`) to navigate and `/` to filter. See [advanced-git.md](advanced-git.md) for the full key reference.

## Log

Press `l` to open the commit log. From there:

| Key | Action |
|-----|--------|
| `↑`/`↓` | Scroll |
| `enter` | Open commit detail (diff stat, author, date, body) |
| `/` | Open search / filter input |
| `ctrl+/` or `ctrl+r` | Clear active filter |
| `m` | Load more commits (shown when more are available) |
| `esc` | Back (first `esc` clears any active filter) |

### Commit detail

From the commit detail view:

| Key | Action |
|-----|--------|
| `↑`/`↓` | Scroll |
| `d` | View the full diff for this commit |
| `p` | Cherry-pick this commit onto the current branch |
| `R` | Cherry-pick a range — enter the starting commit hash; bonsai picks from that commit through the selected commit onto the current branch |
| `r` | Revert this commit — creates a new commit that undoes the changes (safe on shared branches) |
| `y` | Copy the commit hash to the clipboard |
| `esc` | Back |

### Reverting a commit

Press `r` on any commit in the log or commit detail view to create a revert commit. A banner appears at the top of the main panel while the revert is in progress.

If the revert produces conflicts, resolve them in the file list, then:

| Key | Action |
|-----|--------|
| `c` | Continue the revert after resolving conflicts |
| `a` | Abort the revert and restore the previous state |

Unlike `reset`, revert does not rewrite history — it adds a new commit. This makes it safe to use on branches that have already been pushed to a shared remote.

## Stash

| Key | Action |
|-----|--------|
| `s` | Stash all current changes (opens a message input so you can name the stash) |
| `S` | Open stash list - pop, apply, or drop an entry |

## Pull requests

Press `K` to open the PR / MR panel. bonsai detects the remote host and picks the right provider (gh for GitHub, glab for GitLab, bb for Bitbucket).

### PR list

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `enter` | Open PR detail view |
| `o` | Open PR in browser |
| `d` | View full diff |
| `n` | Create a new PR for the current branch |
| `r` | Refresh list |
| `esc` | Back |

### PR detail

Press `enter` on a PR to open the detail panel. It shows state, CI status, labels, requested reviewers, assignees, and URL.

| Key | Action |
|-----|--------|
| `o` | Open in browser |
| `d` | View full diff |
| `a` | Approve |
| `A` | Request changes (prompts for a reason) |
| `c` | Post a general comment |
| `m` | Open merge picker |
| `y` | Copy URL to clipboard |
| `esc` | Back to PR list |

### Creating a PR

Press `n` from the PR list to open the create form. Three fields are shown - title (pre-filled from the HEAD commit), description (optional textarea), and base branch (optional, defaults to the provider default).

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Move between fields |
| `d` | Toggle draft mode on/off before submitting |
| `enter` | Submit (from title or base branch field) |
| `ctrl+s` | Submit from any field |
| `esc` | Cancel |

Press `d` to mark the PR as a draft before submitting. Draft PRs are supported by GitHub and GitLab; Bitbucket does not support draft status.

### Merging a PR

Press `m` from the PR list or from the detail panel to open the merge picker. Choose between merge commit, squash, or rebase. Confirming deletes the source branch automatically.

| Key | Action |
|-----|--------|
| `↑` / `↓` | Select merge method |
| `enter` | Execute merge |
| `esc` | Cancel |

## Advanced operations

### Undo

Press `U` to undo the last undoable operation (commit, merge, rebase). This only works immediately after the action - bonsai remembers one step. For a full reset menu, press `z`.

### Untrack a file

Select a staged file and press `u` to remove it from the git index (`git rm --cached`) while keeping it on disk. Useful when you accidentally staged a file that should be in `.gitignore`.

### Reset

Press `z` to open the reset menu. Choose between soft (keep staged), mixed (keep unstaged), or hard (discard all changes). All three modes require confirmation — each dialog describes exactly what will happen to your staged and working tree changes before anything is applied.

### Tags

Press `t` to list tags. From the tag list you can create a new tag at HEAD or delete an existing one.

In the tag create form, press `tab` to toggle between **lightweight** and **annotated** tag types. Annotated tags require a message and store the tagger identity, date, and an optional signature — they are preferred for releases. Lightweight tags are just a named pointer to a commit.

### Blame

Select a file and press `e` to open the blame view. Each line is annotated with the commit hash, author, and date.

### Bisect

Press `i` to open the bisect panel. Mark commits as good or bad to find the commit that introduced a bug.

When a bisect session is active, press `s` to skip the current commit if it cannot be tested (e.g. it does not compile or is otherwise untestable). Git will move to a different commit in the search range.

### Interactive rebase

Press `R` to open the interactive rebase panel. Enter a base ref (e.g. `HEAD~5` or a commit hash), then reorder and relabel commits (pick, reword, squash, fixup, drop).

### Amend

Press `A` to open the amend panel. You can change the commit message, author, date, or use `--no-edit` to amend without changing the message.

### Worktrees

Press `W` to list linked worktrees. You can add a new worktree (checked out to a different branch) or remove an existing one. Press `p` to prune stale worktree refs — this removes administrative files for worktrees whose directories no longer exist on disk (`git worktree prune`).

### Remotes

Press `O` to open remote management. From there you can list all configured remotes, add a new one, remove, or rename an existing one. With a remote selected, press `p` to prune stale remote-tracking refs — this removes local refs for branches that have been deleted on that remote (`git remote prune <name>`).

### Submodules

Press `M` to open the submodule panel. You can list all submodules, add a new one, run `update --init`, or deinit a submodule.

### Git notes

Press `n` to view the note attached to the HEAD commit. You can edit the note inline or delete it.

### Reflog

Press `L` to open the reflog. Scroll through all HEAD movements, copy a hash with `y`, or reset to an entry with `r`.

### Clean

Press `X` to preview untracked files that would be removed by `git clean -fd`. A confirmation is required before anything is deleted.

### Conflicts

When merge conflicts exist they appear at the top of the file list marked with `!`. Select a conflicted file and press `d` to open the conflict viewer.

| Key | Action |
|-----|--------|
| `o` | Accept ours (discard their changes) |
| `t` | Accept theirs (discard our changes) |
| `r` | Remove conflict markers, keep both sides |

### Configuration manager

Press `C` to open the configuration manager:

- **Global config** - view and edit `~/.gitconfig`
- **Local config** - view and edit `.git/config`
- **Global gitignore** - view and edit the global ignore file
- **Local .gitignore** - view and edit the project ignore file
- **Recommendations** - best-practice settings with one-key apply
- **Profiles (includeIf)** - manage conditional config includes

### SSH key manager

Press `` ` `` to open the SSH key manager. It lists your local SSH keys and lets you test connectivity to the repo's remote host.

| Key | Action |
|-----|--------|
| `t` | Test SSH connection for the selected key |
| `esc` | Back |

### LFS

Press `V` to open the LFS panel. Shows tracked patterns and pending objects.

| Key | Action |
|-----|--------|
| `t` | Track a new file pattern |
| `u` | Untrack the selected pattern |
| `p` | Pull LFS objects |
| `P` | Push LFS objects |
| `esc` | Back |

### Multi-repo dashboard

Press `D` to open the multi-repo dashboard. Lists all repos registered in your bonsai config. Press `enter` to open a repo in bonsai, `esc` to close.

### Issues

Press `I` to open the issues panel (GitHub / GitLab). Lists open issues. Press `enter` on an issue to create a linked branch.

### Gitflow finish

In gitflow mode, press `F` on a feature, bugfix, release, or hotfix branch to finish it - merges into the appropriate targets (main/develop) following the gitflow convention.

### Aborting in-progress operations

When a merge, rebase, or cherry-pick is in progress, press `a` from the main panel to abort it.

## Education panel

After every action bonsai briefly shows a feedback panel:

- **standard** mode: shows the command that ran (e.g. `git commit -m "..."`)
- **guided** mode: shows the command plus a plain-language explanation and any relevant tips
- **pro** mode: shows the panel only for complex operations (rebase, cherry-pick, amend, bisect, worktrees, etc.)

Press any key to dismiss the panel immediately. The duration is configurable:

```toml
[education]
panel_duration = 4   # seconds; 0 disables it entirely
```

### Usage tracking and mastery

bonsai tracks how many times you run each command in `~/.config/bonsai/usage.json`. When you reach the mastery threshold for a command, it asks you once whether you want to keep seeing the education panel for that command or suppress it.

Thresholds vary by complexity:

| Command | Threshold |
|---------|-----------|
| `add`, `commit` | 20 uses |
| `push`, `pull` | 15 uses |
| `branch`, `checkout` | 12 uses |
| `stash`, `merge` | 10 uses |
| `rebase`, `cherry-pick` | 8 uses |
| `reset`, `restore` | 6 uses |
| `bisect`, `worktree`, `notes` | 5 uses |

Choosing "suppress" sets a flag for that command in `usage.json`. You can re-enable it at any time from the **Education & Usage** section of the configuration manager (`C`).

### Education manager (`C` → Education & Usage)

The education manager shows every command bonsai has recorded, your current usage count, the mastery threshold, and whether the panel is suppressed. Select a command and press `enter` to toggle suppression.

