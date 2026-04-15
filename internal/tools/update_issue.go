package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerUpdateIssue(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("update_issue",
		mcp.WithDescription("Update a Redmine issue. Can change fields (status, assignee, done_ratio, etc.) and/or add a comment via the 'notes' parameter."),
		mcp.WithNumber("issue_id",
			mcp.Description("Redmine issue number"),
			mcp.Required(),
		),
		mcp.WithString("notes",
			mcp.Description("Comment to add to the issue"),
		),
		mcp.WithString("subject",
			mcp.Description("New subject/title"),
		),
		mcp.WithString("description",
			mcp.Description("New description"),
		),
		mcp.WithString("status",
			mcp.Description("New status name (e.g. 'En cours', 'Résolu') or numeric ID"),
		),
		mcp.WithString("assignee",
			mcp.Description("New assignee name or numeric ID"),
		),
		mcp.WithString("tracker",
			mcp.Description("New tracker name or numeric ID"),
		),
		mcp.WithNumber("done_ratio",
			mcp.Description("Completion percentage (0-100)"),
		),
		mcp.WithNumber("priority_id",
			mcp.Description("New priority numeric ID"),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		issueID := req.GetInt("issue_id", 0)
		if issueID == 0 {
			return mcp.NewToolResultError("issue_id is required"), nil
		}

		var params redmine.IssueUpdateParams

		if notes := req.GetString("notes", ""); notes != "" {
			params.Notes = &notes
		}
		if subject := req.GetString("subject", ""); subject != "" {
			params.Subject = &subject
		}
		if desc := req.GetString("description", ""); desc != "" {
			params.Description = &desc
		}

		if status := req.GetString("status", ""); status != "" {
			resolved, err := client.ResolveStatusID(status)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid status: %v", err)), nil
			}
			// ResolveStatusID may return "open"/"closed" which aren't numeric
			// For update, we need the actual ID
			statuses, err := client.GetStatuses()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get statuses: %v", err)), nil
			}
			for _, s := range statuses {
				if fmt.Sprintf("%d", s.ID) == resolved {
					params.StatusID = &s.ID
					break
				}
			}
			if params.StatusID == nil {
				return mcp.NewToolResultError(fmt.Sprintf("status %q resolved to %q which is not a numeric ID — use a specific status name", status, resolved)), nil
			}
		}

		if assignee := req.GetString("assignee", ""); assignee != "" {
			resolved, err := client.ResolveUserID(assignee)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid assignee: %v", err)), nil
			}
			var id int
			fmt.Sscanf(resolved, "%d", &id)
			if id > 0 {
				params.AssignedToID = &id
			}
		}

		if tracker := req.GetString("tracker", ""); tracker != "" {
			resolved, err := client.ResolveTrackerID(tracker)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid tracker: %v", err)), nil
			}
			var id int
			fmt.Sscanf(resolved, "%d", &id)
			if id > 0 {
				params.TrackerID = &id
			}
		}

		if doneRatio := req.GetInt("done_ratio", -1); doneRatio >= 0 {
			params.DoneRatio = &doneRatio
		}

		if priorityID := req.GetInt("priority_id", 0); priorityID > 0 {
			params.PriorityID = &priorityID
		}

		if err := client.UpdateIssue(issueID, params); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Issue #%d updated successfully.", issueID)), nil
	})
}
