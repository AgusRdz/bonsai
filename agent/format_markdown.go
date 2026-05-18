package agent

import (
	"fmt"
	"strings"
)

// FormatMarkdown returns GitHub-flavored Markdown for any agent output value.
func FormatMarkdown(v any) string {
	switch out := v.(type) {
	case *StatusOut:
		return statusMarkdown(out)
	case []LogEntry:
		return logMarkdown(out)
	case *DiffOut:
		return diffMarkdown(out)
	case *ShowOut:
		return showMarkdown(out)
	case []BlameEntry:
		return blameMarkdown(out)
	case []BranchEntry:
		return branchesMarkdown(out)
	case []StashEntry:
		return stashMarkdown(out)
	case *ReviewOut:
		return reviewMarkdown(out)
	case *ContextOut:
		return contextMarkdown(out)
	default:
		return fmt.Sprintf("<!-- unsupported type %T -->\n", v)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mdTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return "_none_\n"
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	sep := make([]string, len(headers))
	for i := range sep {
		sep[i] = "---"
	}
	b.WriteString("| " + strings.Join(sep, " | ") + " |\n")
	for _, row := range rows {
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}

func mdPlural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// compactCommits writes one line per commit: hash  date  author  subject.
// Cheaper than a markdown table for agent consumers.
func compactCommits(entries []LogEntry, b *strings.Builder) {
	for _, e := range entries {
		fmt.Fprintf(b, "`%s`  %s  %s  %s\n", e.Hash, e.Date, e.Author, e.Subject)
	}
	b.WriteByte('\n')
}

// ---------------------------------------------------------------------------
// Type formatters
// ---------------------------------------------------------------------------

func statusMarkdown(s *StatusOut) string {
	var b strings.Builder

	upstreamInfo := ""
	if s.Upstream != "" {
		upstreamInfo = fmt.Sprintf(" → `%s`", s.Upstream)
	}
	if s.Ahead > 0 || s.Behind > 0 {
		upstreamInfo += fmt.Sprintf(" ↑%d ↓%d", s.Ahead, s.Behind)
	}
	fmt.Fprintf(&b, "# Status: %s%s\n\n", s.Branch, upstreamInfo)

	if s.MergeState != "" {
		fmt.Fprintf(&b, "> **In progress:** %s\n\n", s.MergeState)
	}

	if len(s.Staged) > 0 {
		fmt.Fprintf(&b, "## Staged (%d)\n\n", len(s.Staged))
		rows := make([][]string, len(s.Staged))
		for i, f := range s.Staged {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}

	if len(s.Unstaged) > 0 {
		fmt.Fprintf(&b, "## Unstaged (%d)\n\n", len(s.Unstaged))
		rows := make([][]string, len(s.Unstaged))
		for i, f := range s.Unstaged {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}

	if len(s.Conflicts) > 0 {
		fmt.Fprintf(&b, "## Conflicts (%d)\n\n", len(s.Conflicts))
		rows := make([][]string, len(s.Conflicts))
		for i, f := range s.Conflicts {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}

	if len(s.Untracked) > 0 {
		fmt.Fprintf(&b, "## Untracked (%d)\n\n", len(s.Untracked))
		for _, f := range s.Untracked {
			fmt.Fprintf(&b, "- `%s`\n", f.Path)
		}
		b.WriteByte('\n')
	}

	if s.StashCount > 0 {
		fmt.Fprintf(&b, "**Stash:** %d %s\n", s.StashCount, mdPlural(s.StashCount, "entry", "entries"))
	}

	return b.String()
}

func logMarkdown(entries []LogEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Commits (%d)\n\n", len(entries))
	compactCommits(entries, &b)
	return b.String()
}

func diffMarkdown(d *DiffOut) string {
	var b strings.Builder
	b.WriteString("# Diff\n\n")

	if len(d.Staged) > 0 {
		fmt.Fprintf(&b, "## Staged (%d)\n\n", len(d.Staged))
		b.WriteString(diffFilesMarkdown(d.Staged))
	}

	if len(d.Unstaged) > 0 {
		fmt.Fprintf(&b, "## Unstaged (%d)\n\n", len(d.Unstaged))
		b.WriteString(diffFilesMarkdown(d.Unstaged))
	}

	if len(d.Untracked) > 0 {
		fmt.Fprintf(&b, "## Untracked (%d)\n\n", len(d.Untracked))
		for _, u := range d.Untracked {
			fmt.Fprintf(&b, "- `%s`\n", u.Path)
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func diffFilesMarkdown(files []FileDiff) string {
	if len(files) == 0 {
		return "_none_\n\n"
	}
	var b strings.Builder
	for _, f := range files {
		status := ""
		if f.Status != "" {
			status = " " + f.Status
		}
		fmt.Fprintf(&b, "### `%s`%s  +%d -%d\n\n", f.Path, status, f.Additions, f.Deletions)
		for _, h := range f.Hunks {
			b.WriteString("```diff\n")
			b.WriteString(h.Header + "\n")
			for _, line := range h.Lines {
				b.WriteString(line + "\n")
			}
			b.WriteString("```\n\n")
		}
	}
	return b.String()
}

func showMarkdown(s *ShowOut) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# `%s` · %s\n\n", s.Hash, s.Subject)
	fmt.Fprintf(&b, "**Author:** %s  \n**Date:** %s  \n", s.Author, s.Date)
	fmt.Fprintf(&b, "**Changes:** +%d -%d in %d %s\n\n",
		s.Additions, s.Deletions, s.FilesChanged, mdPlural(s.FilesChanged, "file", "files"))
	if len(s.Diff) > 0 {
		b.WriteString("## Files\n\n")
		b.WriteString(diffFilesMarkdown(s.Diff))
	}
	return b.String()
}

func blameMarkdown(entries []BlameEntry) string {
	var b strings.Builder
	b.WriteString("# Blame\n\n")
	rows := make([][]string, len(entries))
	for i, e := range entries {
		content := strings.ReplaceAll(e.Content, "|", "\\|")
		rows[i] = []string{
			fmt.Sprintf("%d", e.Line),
			"`" + e.Hash + "`",
			e.Date,
			e.Author,
			"`" + content + "`",
		}
	}
	b.WriteString(mdTable([]string{"Line", "Hash", "Date", "Author", "Content"}, rows))
	return b.String()
}

func branchesMarkdown(branches []BranchEntry) string {
	var b strings.Builder
	b.WriteString("# Branches\n\n")
	rows := make([][]string, len(branches))
	for i, br := range branches {
		current := ""
		if br.Current {
			current = "✓"
		}
		rows[i] = []string{br.Name, current, br.Upstream}
	}
	b.WriteString(mdTable([]string{"Branch", "Current", "Upstream"}, rows))
	return b.String()
}

func stashMarkdown(entries []StashEntry) string {
	var b strings.Builder
	b.WriteString("# Stash\n\n")
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e.Ref, e.Description}
	}
	b.WriteString(mdTable([]string{"Ref", "Description"}, rows))
	return b.String()
}

func contextMarkdown(c *ContextOut) string {
	var b strings.Builder

	s := c.Status
	upstreamInfo := ""
	if s.Upstream != "" {
		upstreamInfo = fmt.Sprintf(" → `%s`", s.Upstream)
	}
	if s.Ahead > 0 || s.Behind > 0 {
		upstreamInfo += fmt.Sprintf(" ↑%d ↓%d", s.Ahead, s.Behind)
	}
	fmt.Fprintf(&b, "# Context: %s%s\n\n", s.Branch, upstreamInfo)

	if s.MergeState != "" {
		fmt.Fprintf(&b, "> **In progress:** %s\n\n", s.MergeState)
	}

	// Status summary
	b.WriteString("## Status\n\n")
	if len(s.Staged) > 0 {
		fmt.Fprintf(&b, "**Staged (%d):**\n\n", len(s.Staged))
		rows := make([][]string, len(s.Staged))
		for i, f := range s.Staged {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}
	if len(s.Unstaged) > 0 {
		fmt.Fprintf(&b, "**Unstaged (%d):**\n\n", len(s.Unstaged))
		rows := make([][]string, len(s.Unstaged))
		for i, f := range s.Unstaged {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}
	if len(s.Conflicts) > 0 {
		fmt.Fprintf(&b, "**Conflicts (%d):**\n\n", len(s.Conflicts))
		rows := make([][]string, len(s.Conflicts))
		for i, f := range s.Conflicts {
			rows[i] = []string{f.Status, "`" + f.Path + "`"}
		}
		b.WriteString(mdTable([]string{"Status", "File"}, rows))
		b.WriteByte('\n')
	}
	if len(s.Staged) == 0 && len(s.Unstaged) == 0 && len(s.Conflicts) == 0 {
		b.WriteString("_working tree clean_\n\n")
	}
	if len(s.Untracked) > 0 {
		fmt.Fprintf(&b, "**Untracked (%d):**\n\n", len(s.Untracked))
		for _, f := range s.Untracked {
			fmt.Fprintf(&b, "- `%s`\n", f.Path)
		}
		b.WriteByte('\n')
	}
	if s.StashCount > 0 {
		fmt.Fprintf(&b, "**Stash:** %d %s\n\n", s.StashCount, mdPlural(s.StashCount, "entry", "entries"))
	}

	// Diff changes (only if there is something)
	d := c.Diff
	hasChanges := len(d.Staged) > 0 || len(d.Unstaged) > 0
	if hasChanges {
		b.WriteString("## Changes\n\n")
		if len(d.Staged) > 0 {
			fmt.Fprintf(&b, "### Staged (%d %s)\n\n", len(d.Staged), mdPlural(len(d.Staged), "file", "files"))
			b.WriteString(diffFilesMarkdown(d.Staged))
		}
		if len(d.Unstaged) > 0 {
			fmt.Fprintf(&b, "### Unstaged (%d %s)\n\n", len(d.Unstaged), mdPlural(len(d.Unstaged), "file", "files"))
			b.WriteString(diffFilesMarkdown(d.Unstaged))
		}
	}

	// Recent commits
	if len(c.Log) > 0 {
		fmt.Fprintf(&b, "## Recent Commits (%d)\n\n", len(c.Log))
		compactCommits(c.Log, &b)
	}

	return b.String()
}

func reviewMarkdown(r *ReviewOut) string {
	var b strings.Builder
	base := r.Base
	if base == "" {
		base = "(staged)"
	}
	fmt.Fprintf(&b, "# Review: %s → %s\n\n", base, r.Head)

	fmt.Fprintf(&b, "**Changes:** +%d -%d in %d %s\n\n",
		r.Lines.Added, r.Lines.Removed, r.FilesChanged, mdPlural(r.FilesChanged, "file", "files"))

	if len(r.Commits) > 0 {
		fmt.Fprintf(&b, "## Commits (%d)\n\n", len(r.Commits))
		compactCommits(r.Commits, &b)
	}

	fmt.Fprintf(&b, "## Files Changed (%d)\n\n", len(r.Diff))
	b.WriteString(diffFilesMarkdown(r.Diff))

	return b.String()
}
