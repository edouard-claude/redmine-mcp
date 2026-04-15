package tools

import (
	"fmt"
	"strings"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
)

func formatIssue(issue *redmine.Issue, maxDesc int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Issue #%d — %s\n", issue.ID, issue.Subject)
	fmt.Fprintf(&b, "Project: %s | Tracker: %s | Priority: %s\n", issue.Project.Name, issue.Tracker.Name, issue.Priority.Name)

	statusLabel := "open"
	if issue.Status.IsClosed {
		statusLabel = "closed"
	}
	fmt.Fprintf(&b, "Status: %s (%s) | Done: %d%%\n", issue.Status.Name, statusLabel, issue.DoneRatio)

	fmt.Fprintf(&b, "Author: %s", issue.Author.Name)
	if issue.AssignedTo != nil {
		fmt.Fprintf(&b, " | Assignee: %s", issue.AssignedTo.Name)
	}
	b.WriteString("\n")

	if issue.FixedVersion != nil {
		fmt.Fprintf(&b, "Version: %s\n", issue.FixedVersion.Name)
	}

	fmt.Fprintf(&b, "Created: %s | Updated: %s\n", formatDateTime(issue.CreatedOn), formatDateTime(issue.UpdatedOn))

	if issue.StartDate != nil || issue.DueDate != nil {
		if issue.StartDate != nil {
			fmt.Fprintf(&b, "Start: %s", *issue.StartDate)
		}
		if issue.DueDate != nil {
			if issue.StartDate != nil {
				b.WriteString(" | ")
			}
			fmt.Fprintf(&b, "Due: %s", *issue.DueDate)
		}
		b.WriteString("\n")
	}

	if issue.Parent != nil {
		fmt.Fprintf(&b, "Parent: #%d\n", issue.Parent.ID)
	}

	if issue.Description != "" {
		desc := issue.Description
		if maxDesc > 0 && len(desc) > maxDesc {
			desc = desc[:maxDesc] + "\n[truncated]"
		}
		fmt.Fprintf(&b, "\n## Description\n%s\n", desc)
	}

	return b.String()
}

func formatIssueSummaries(issues []redmine.Issue, offset int) string {
	if len(issues) == 0 {
		return "No issues found."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d issue(s) (offset %d):\n\n", len(issues), offset)

	for _, issue := range issues {
		fmt.Fprintf(&b, "#%d | %s | %s | %s\n", issue.ID, issue.Status.Name, issue.Tracker.Name, issue.Subject)
		fmt.Fprintf(&b, "  Project: %s", issue.Project.Name)
		if issue.AssignedTo != nil {
			fmt.Fprintf(&b, " | Assignee: %s", issue.AssignedTo.Name)
		}
		fmt.Fprintf(&b, " | %d%% | Updated: %s\n\n", issue.DoneRatio, formatDateTime(issue.UpdatedOn))
	}

	return b.String()
}

func formatComments(issueID int, journals []redmine.Journal) string {
	// filter to journals with notes only
	var comments []redmine.Journal
	for _, j := range journals {
		if j.Notes != "" {
			comments = append(comments, j)
		}
	}

	if len(comments) == 0 {
		return fmt.Sprintf("No comments for issue #%d.", issueID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Comments for #%d (%d comment(s))\n\n", issueID, len(comments))

	for i, c := range comments {
		fmt.Fprintf(&b, "### Comment #%d (journal_id: %d) by %s — %s\n%s\n\n", i+1, c.ID, c.User.Name, formatDateTime(c.CreatedOn), c.Notes)
	}

	return b.String()
}

func formatProjects(projects []redmine.Project) string {
	if len(projects) == 0 {
		return "No projects found."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Projects (%d)\n\n", len(projects))

	for _, p := range projects {
		fmt.Fprintf(&b, "- **%s** (`%s`)", p.Name, p.Identifier)
		if p.Description != "" {
			desc := p.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			fmt.Fprintf(&b, " — %s", desc)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatAttachments(issueID int, attachments []redmine.Attachment) string {
	if len(attachments) == 0 {
		return fmt.Sprintf("No attachments for issue #%d.", issueID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Attachments for #%d (%d file(s))\n\n", issueID, len(attachments))

	for i, a := range attachments {
		fmt.Fprintf(&b, "%d. **%s** (%s, %s)\n", i+1, a.Filename, formatSize(a.Filesize), a.ContentType)
		fmt.Fprintf(&b, "   By %s — %s\n", a.Author.Name, formatDateTime(a.CreatedOn))
		if a.Description != "" {
			fmt.Fprintf(&b, "   Description: %s\n", a.Description)
		}
		if a.ContentURL != "" {
			fmt.Fprintf(&b, "   URL: %s\n", a.ContentURL)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatChildren(issueID int, children []redmine.ChildIssue) string {
	if len(children) == 0 {
		return fmt.Sprintf("No subtasks for issue #%d.", issueID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Subtasks of #%d (%d issue(s))\n\n", issueID, len(children))

	for _, c := range children {
		fmt.Fprintf(&b, "#%d | %s | %s\n", c.ID, c.Tracker.Name, c.Subject)
	}

	return b.String()
}

// formatDateTime trims the Redmine ISO timestamp to a readable format.
func formatDateTime(s string) string {
	if len(s) >= 16 {
		return s[:10] + " " + s[11:16]
	}
	return s
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
