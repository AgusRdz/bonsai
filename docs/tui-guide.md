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
| `d` | View diff for the selected file |
| `x` | Discard all working tree changes to the selected file (confirm required) |
| `o` | Restore the selected file to HEAD or a specific ref |

## Committing

Press `c` to open the commit panel. Type your message and press `enter` to commit. Press `esc` to cancel.

In **guided** mode the education panel appears after the commit and explains what happened in plain language along with the exact command that ran.

## Pushing and pulling

| Key | Action |
|-----|--------|
| `p` | Push to remote |
| `P` | Pull from remote |
| `f` | Fetch menu - choose origin/--all/--prune |

## Branches

| Key | Action |
|-----|--------|
| `b` | Create a new branch (or open the flow picker in gitflow mode) |
| `B` | List all branches - switch, rename, delete |

## Log

Press `l` to open the commit log. From there:

| Key | Action |
|-----|--------|
| `↑`/`↓` | Scroll |
| `enter` | Open commit detail (diff stat, author, date, body) |
| `m` | Merge the selected commit into the current branch |
| `ctrl+/` or `ctrl+r` | Search / filter commits |
| `esc` | Back |

## Stash

| Key | Action |
|-----|--------|
| `s` | Stash all current changes |
| `S` | Open stash list - pop, apply, drop an entry |

## Advanced operations

### Reset

Press `z` to open the reset menu. Choose between soft (keep staged), mixed (keep unstaged), or hard (discard all changes).

### Tags

Press `t` to list tags. From the tag list you can create a new tag at HEAD or delete an existing one.

### Blame

Select a file and press `e` to open the blame view. Each line is annotated with the commit hash, author, and date.

### Bisect

Press `i` to open the bisect panel. Mark commits as good or bad to find the commit that introduced a bug.

### Interactive rebase

Press `R` to open the interactive rebase panel. Enter a base ref (e.g. `HEAD~5` or a commit hash), then reorder and relabel commits (pick, reword, squash, fixup, drop).

### Amend

Press `A` to open the amend panel. You can change the commit message, author, date, or use `--no-edit` to amend without changing the message.

### Worktrees

Press `W` to list linked worktrees. You can add a new worktree (checked out to a different branch) or remove an existing one.

### Remotes

Press `O` to open remote management. From there you can list all configured remotes, add a new one, remove, or rename an existing one.

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

### Aborting in-progress operations

When a merge, rebase, or cherry-pick is in progress, press `a` from the main panel to abort it.

## Education panel

After every action bonsai briefly shows a feedback panel:

- **standard** mode: shows the command that ran (e.g. `git commit -m "..."`)
- **guided** mode: shows the command plus a plain-language explanation and any relevant tips
- **pro** mode: no panel

Press any key to dismiss the panel immediately. The duration is configurable:

```toml
[education]
panel_duration = 4   # seconds; 0 disables it entirely
```
