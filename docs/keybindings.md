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
| `m` | Merge selected commit into current branch |
| `p` | Cherry-pick selected commit |
| `ctrl+/` or `ctrl+r` | Search / filter commits |
| `space` | Load more commits |
| `esc` | Back |

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

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `esc` | Back |

## Conflict panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `o` | Accept ours |
| `t` | Accept theirs |
| `r` | Remove conflict markers (keep both) |
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

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `enter` | Cycle action for selected commit |
| `ctrl+↑` / `ctrl+↓` | Reorder commits |
| `r` | Start the rebase |
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
| `enter` | Apply selected hunks |
| `esc` | Back |

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
