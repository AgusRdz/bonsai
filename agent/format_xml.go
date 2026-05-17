package agent

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// FormatXML returns an XML document for any agent output value.
func FormatXML(v any) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	switch out := v.(type) {
	case *StatusOut:
		statusXML(&b, out)
	case []LogEntry:
		logXML(&b, out)
	case *DiffOut:
		diffXML(&b, out)
	case *ShowOut:
		showXML(&b, out)
	case []BlameEntry:
		blameXML(&b, out)
	case []BranchEntry:
		branchesXML(&b, out)
	case []StashEntry:
		stashXML(&b, out)
	case *ReviewOut:
		reviewXML(&b, out)
	case *ContextOut:
		contextXML(&b, out)
	default:
		fmt.Fprintf(&b, "<!-- unsupported type %T -->\n", v)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// xe escapes s for safe use in XML text content or attribute values.
func xe(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s)) //nolint:errcheck
	return b.String()
}

func xmlTag(b *strings.Builder, indent, name, value string) {
	fmt.Fprintf(b, "%s<%s>%s</%s>\n", indent, name, xe(value), name)
}

func xmlInt(b *strings.Builder, indent, name string, value int) {
	fmt.Fprintf(b, "%s<%s>%d</%s>\n", indent, name, value, name)
}

func xmlFileEntry(b *strings.Builder, indent string, f FileEntry) {
	fmt.Fprintf(b, `%s<file status="%s">%s</file>`+"\n", indent, xe(f.Status), xe(f.Path))
}

func xmlFileDiff(b *strings.Builder, indent string, f FileDiff) {
	fmt.Fprintf(b, `%s<file path="%s" additions="%d" deletions="%d"`, indent, xe(f.Path), f.Additions, f.Deletions)
	if f.Status != "" {
		fmt.Fprintf(b, ` status="%s"`, xe(f.Status))
	}
	if len(f.Hunks) == 0 {
		b.WriteString("/>\n")
		return
	}
	b.WriteString(">\n")
	for _, h := range f.Hunks {
		fmt.Fprintf(b, `%s  <hunk header="%s">`+"\n", indent, xe(h.Header))
		for _, line := range h.Lines {
			fmt.Fprintf(b, "%s    <line>%s</line>\n", indent, xe(line))
		}
		fmt.Fprintf(b, "%s  </hunk>\n", indent)
	}
	fmt.Fprintf(b, "%s</file>\n", indent)
}

// ---------------------------------------------------------------------------
// Type formatters
// ---------------------------------------------------------------------------

func statusXML(b *strings.Builder, s *StatusOut) {
	b.WriteString("<status>\n")
	xmlTag(b, "  ", "repo", s.Repo)
	xmlTag(b, "  ", "branch", s.Branch)
	if s.Upstream != "" {
		xmlTag(b, "  ", "upstream", s.Upstream)
	}
	xmlInt(b, "  ", "ahead", s.Ahead)
	xmlInt(b, "  ", "behind", s.Behind)
	if s.MergeState != "" {
		xmlTag(b, "  ", "merge_state", s.MergeState)
	}
	xmlInt(b, "  ", "stash_count", s.StashCount)

	b.WriteString("  <staged>\n")
	for _, f := range s.Staged {
		xmlFileEntry(b, "    ", f)
	}
	b.WriteString("  </staged>\n")

	b.WriteString("  <unstaged>\n")
	for _, f := range s.Unstaged {
		xmlFileEntry(b, "    ", f)
	}
	b.WriteString("  </unstaged>\n")

	if len(s.Conflicts) > 0 {
		b.WriteString("  <conflicts>\n")
		for _, f := range s.Conflicts {
			xmlFileEntry(b, "    ", f)
		}
		b.WriteString("  </conflicts>\n")
	}

	if len(s.Untracked) > 0 {
		b.WriteString("  <untracked>\n")
		for _, f := range s.Untracked {
			fmt.Fprintf(b, "    <file>%s</file>\n", xe(f.Path))
		}
		b.WriteString("  </untracked>\n")
	}

	b.WriteString("</status>\n")
}

func logXML(b *strings.Builder, entries []LogEntry) {
	b.WriteString("<commits>\n")
	for _, e := range entries {
		b.WriteString("  <commit>\n")
		xmlTag(b, "    ", "hash", e.Hash)
		xmlTag(b, "    ", "subject", e.Subject)
		xmlTag(b, "    ", "author", e.Author)
		xmlTag(b, "    ", "date", e.Date)
		b.WriteString("  </commit>\n")
	}
	b.WriteString("</commits>\n")
}

func diffXML(b *strings.Builder, d *DiffOut) {
	b.WriteString("<diff>\n")

	b.WriteString("  <staged>\n")
	for _, f := range d.Staged {
		xmlFileDiff(b, "    ", f)
	}
	b.WriteString("  </staged>\n")

	b.WriteString("  <unstaged>\n")
	for _, f := range d.Unstaged {
		xmlFileDiff(b, "    ", f)
	}
	b.WriteString("  </unstaged>\n")

	if len(d.Untracked) > 0 {
		b.WriteString("  <untracked>\n")
		for _, u := range d.Untracked {
			fmt.Fprintf(b, "    <file>%s</file>\n", xe(u.Path))
		}
		b.WriteString("  </untracked>\n")
	}

	b.WriteString("</diff>\n")
}

func showXML(b *strings.Builder, s *ShowOut) {
	b.WriteString("<commit>\n")
	xmlTag(b, "  ", "hash", s.Hash)
	xmlTag(b, "  ", "subject", s.Subject)
	xmlTag(b, "  ", "author", s.Author)
	xmlTag(b, "  ", "date", s.Date)
	xmlInt(b, "  ", "additions", s.Additions)
	xmlInt(b, "  ", "deletions", s.Deletions)
	xmlInt(b, "  ", "files_changed", s.FilesChanged)
	b.WriteString("  <diff>\n")
	for _, f := range s.Diff {
		xmlFileDiff(b, "    ", f)
	}
	b.WriteString("  </diff>\n")
	b.WriteString("</commit>\n")
}

func blameXML(b *strings.Builder, entries []BlameEntry) {
	b.WriteString("<blame>\n")
	for _, e := range entries {
		fmt.Fprintf(b, `  <line num="%d" hash="%s" author="%s" date="%s">%s</line>`+"\n",
			e.Line, xe(e.Hash), xe(e.Author), xe(e.Date), xe(e.Content))
	}
	b.WriteString("</blame>\n")
}

func branchesXML(b *strings.Builder, branches []BranchEntry) {
	b.WriteString("<branches>\n")
	for _, br := range branches {
		current := "false"
		if br.Current {
			current = "true"
		}
		fmt.Fprintf(b, `  <branch current="%s"`, current)
		if br.Upstream != "" {
			fmt.Fprintf(b, ` upstream="%s"`, xe(br.Upstream))
		}
		fmt.Fprintf(b, ">%s</branch>\n", xe(br.Name))
	}
	b.WriteString("</branches>\n")
}

func stashXML(b *strings.Builder, entries []StashEntry) {
	b.WriteString("<stash>\n")
	for _, e := range entries {
		fmt.Fprintf(b, `  <entry ref="%s">%s</entry>`+"\n", xe(e.Ref), xe(e.Description))
	}
	b.WriteString("</stash>\n")
}

func contextXML(b *strings.Builder, c *ContextOut) {
	b.WriteString("<context>\n")
	statusXML(b, c.Status)
	diffXML(b, c.Diff)
	logXML(b, c.Log)
	b.WriteString("</context>\n")
}

func reviewXML(b *strings.Builder, r *ReviewOut) {
	b.WriteString("<review>\n")
	if r.Base != "" {
		xmlTag(b, "  ", "base", r.Base)
	}
	xmlTag(b, "  ", "head", r.Head)
	b.WriteString("  <lines>\n")
	xmlInt(b, "    ", "added", r.Lines.Added)
	xmlInt(b, "    ", "removed", r.Lines.Removed)
	xmlInt(b, "    ", "total_changed", r.Lines.TotalChanged)
	b.WriteString("  </lines>\n")
	xmlInt(b, "  ", "files_changed", r.FilesChanged)

	if r.CommitsCount > 0 {
		xmlInt(b, "  ", "commits_count", r.CommitsCount)
		b.WriteString("  <commits>\n")
		for _, c := range r.Commits {
			b.WriteString("    <commit>\n")
			xmlTag(b, "      ", "hash", c.Hash)
			xmlTag(b, "      ", "subject", c.Subject)
			xmlTag(b, "      ", "author", c.Author)
			xmlTag(b, "      ", "date", c.Date)
			b.WriteString("    </commit>\n")
		}
		b.WriteString("  </commits>\n")
	}

	b.WriteString("  <diff>\n")
	for _, f := range r.Diff {
		xmlFileDiff(b, "    ", f)
	}
	b.WriteString("  </diff>\n")

	if r.Status != nil {
		b.WriteString("  <status>\n")
		xmlTag(b, "    ", "branch", r.Status.Branch)
		if r.Status.Upstream != "" {
			xmlTag(b, "    ", "upstream", r.Status.Upstream)
		}
		xmlInt(b, "    ", "ahead", r.Status.Ahead)
		xmlInt(b, "    ", "behind", r.Status.Behind)
		b.WriteString("  </status>\n")
	}

	b.WriteString("</review>\n")
}
