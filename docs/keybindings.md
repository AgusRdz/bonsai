# Keybindings Reference

## Main panel

### File navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |

### File operations

| Key | Action |
|-----|--------|
| `space` | Stage an unstaged file / unstage a staged file |
| `+` | Stage all changes at once (`git add .`) |
| `h` | Stage / unstage individual hunks within a file |
| `H` | View commit history for the selected file |
| `d` | View diff for the selected file |
| `x` | Discard working tree changes (confirm required) |
| `o` | Restore file to HEAD or a specific ref |

### Sync

| Key | Action |
|-----|--------|
| `c` | Open commit panel |
| `p` | Open push menu (push / force-with-lease / set-upstream) |
| `P` | Pull from remote |
| `f` | Fetch menu |
| `g` | Open branch graph (git log --graph --all) |

### Stash

| Key | Action |
|-----|--------|
| `s` | Stash all changes (opens message input) |
| `S` | Open stash list |

### Branches

| Key | Action |
|-----|--------|
| `b` | Create a new branch (or flow picker) |
| `B` | List all branches |

### History

| Key | Action |
|-----|--------|
| `l` | Open commit log |
| `L` | Open reflog |

### Advanced operations

| Key | Action |
|-----|--------|
| `z` | Reset menu (soft / mixed / hard) |
| `t` | Tag list |
| `e` | Blame for the selected file |
| `i` | Bisect panel |
| `R` | Interactive rebase |
| `A` | Amend HEAD |
| `W` | Worktree list |
| `O` | Remote management |
| `M` | Submodule management |
| `n` | Notes for HEAD commit |
| `X` | Clean untracked files |
| `a` | Abort in-progress merge / rebase / cherry-pick |

### Configuration

| Key | Action |
|-----|--------|
| `C` | Configuration manager |

### Meta

| Key | Action |
|-----|--------|
| `?` | Help panel |
| `q` / `ctrl+c` | Quit |

---

## Log panel

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `enter` | Open commit detail |
| `/` | Open search / filter input |
| `ctrl+/` or `ctrl+r` | Clear active filter |
| `m` | Load more commits (when more are available) |
| `esc` | Back (first `esc` clears the active filter) |

## Commit detail panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `d` | View full diff of this commit |
| `y` | Copy commit hash to clipboard |
| `p` | Cherry-pick this commit onto current branch |
| `esc` | Back |

## File history panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | View commit detail |
| `esc` | Back |

## Branch graph panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `esc` | Back |

## Branch list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Switch to the selected branch |
| `m` | Merge selected branch into current (confirm required) |
| `r` | Rebase current onto selected branch (confirm required) |
| `d` | Delete the selected branch (confirm required) |
| `n` | Rename the selected branch |
| `D` | Delete the remote tracking branch for the selected branch (confirm required) |
| `esc` | Back |

## Stash list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Pop the selected stash |
| `a` | Apply without removing |
| `d` | Drop the selected stash |
| `esc` | Back |

## Diff panel

For regular file diffs:

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `e` | Blame for the file being diffed |
| `esc` | Back |

For PR diffs (opened with `d` from the PR panel):

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `c` | Post an inline comment on the selected line |
| `esc` | Back |

## Conflict panel

Three-way merge editor (base / ours / theirs per hunk):

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move between conflict hunks |
| `o` | Accept ours |
| `t` | Accept theirs |
| `b` | Accept base (common ancestor) |
| `r` | Remove conflict markers (keep both sides) |
| `e` | Manual edit mode - type a custom resolution for this hunk |
| `esc` | Back |

## PR / MR panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Open PR in browser |
| `d` | View full diff |
| `a` | Approve |
| `A` | Request changes (with reason) |
| `c` | Post a general comment |
| `n` | Create a new PR for the current branch |
| `esc` | Back |

## SSH panel (`` ` ``)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `t` | Test SSH connection for selected key |
| `esc` | Back |

## LFS panel (`V`)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `t` | Track a new file pattern |
| `u` | Untrack selected pattern |
| `p` | Pull LFS objects |
| `P` | Push LFS objects |
| `esc` | Back |

## Multi-repo dashboard (`D`)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Open selected repo in bonsai |
| `esc` | Back |

## Tag list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `n` | Create a new tag at HEAD |
| `d` | Delete the selected tag |
| `p` | Push the selected tag to origin (confirm required) |
| `esc` | Back |

## Worktree list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `a` | Add a new worktree |
| `d` | Remove the selected worktree |
| `esc` | Back |

## Remote list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `a` | Add a new remote |
| `d` | Remove the selected remote |
| `r` | Rename the selected remote |
| `esc` | Back |

## Submodule list panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `a` | Add a new submodule |
| `u` | Update --init all submodules |
| `d` | Deinit the selected submodule |
| `esc` | Back |

## Reflog panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `r` | Reset HEAD to the selected entry |
| `y` | Copy hash to clipboard |
| `esc` | Back |

## Interactive rebase panel

Step 1: enter a base ref (e.g. `HEAD~3`) and press `enter` to load commits.

Step 2: edit the todo list, then press `enter` to execute.

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `K` | Move selected commit up (reorder) |
| `J` | Move selected commit down (reorder) |
| `p` | pick |
| `r` | reword |
| `e` | edit |
| `s` | squash |
| `f` | fixup |
| `d` | drop |
| `enter` | Execute rebase |
| `esc` | Cancel |

## Amend panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate options |
| `enter` | Select option / confirm |
| `esc` | Back |

## Note view panel

| Key | Action |
|-----|--------|
| `e` | Edit the note |
| `d` | Delete the note |
| `esc` | Back |

## Fetch panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Select option |
| `enter` | Run fetch |
| `esc` | Back |

## Bisect panel

| Key | Action |
|-----|--------|
| `s` | Start bisect |
| `b` | Mark current as bad |
| `g` | Mark current as good (or enter a specific hash) |
| `r` | Reset bisect |
| `esc` | Back |

## Blame panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `esc` | Back |

## Configuration manager

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate sections |
| `enter` | Open the selected section |
| `esc` | Back |

### Config file view

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `e` | Open in editor |
| `esc` | Back |

### Recommendations view

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Apply the selected recommendation |
| `esc` | Back |

## Hunk stage panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `space` | Toggle hunk selected / deselected |
| `a` | Select all / deselect all |
| `l` | Enter line mode for the focused hunk |
| `enter` | Apply selected hunks |
| `esc` | Back |

### Line mode (within a hunk)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move between lines |
| `space` | Toggle individual `+`/`-` line selected / deselected |
| `enter` | Apply selected lines |
| `esc` | Back to hunk list |

## Push menu

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `enter` | Execute selected push option |
| `esc` | Back |

## Remappable keys

The following keys can be changed in `[keybindings]` in your config:

| Default | Config key |
|---------|------------|
| `c` | `commit` |
| `p` | `push` |
| `l` | `pull` |
| `s` | `stash` |
| `z` | `undo` |
| `q` | `quit` |
| `b` | `branch` |
| `g` | `graph` |

All other keys are fixed.
