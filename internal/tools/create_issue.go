package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerCreateIssue(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("create_issue",
		mcp.WithDescription("Create a new Redmine issue. Requires project and subject at minimum."),
		mcp.WithString("project",
			mcp.Description("Project identifier (e.g. 'fidelatoo')"),
			mcp.Required(),
		),
		mcp.WithString("subject",
			mcp.Description("Issue subject/title"),
			mcp.Required(),
		),
		mcp.WithString("description",
			mcp.Description("Issue description (supports Textile markup)"),
		),
		mcp.WithString("tracker",
			mcp.Description("Tracker name (e.g. 'Anomalie', 'Evolution') or numeric ID"),
		),
		mcp.WithString("status",
			mcp.Description("Initial status name or numeric ID"),
		),
		mcp.WithNumber("priority_id",
			mcp.Description("Priority numeric ID"),
		),
		mcp.WithString("assignee",
			mcp.Description("Assignee name or numeric ID"),
		),
		mcp.WithString("version",
			mcp.Description("Target version name or numeric ID"),
		),
		mcp.WithNumber("parent_issue_id",
			mcp.Description("Parent issue ID for subtasks"),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		if project == "" {
			return mcp.NewToolResultError("project is required"), nil
		}
		subject := req.GetString("subject", "")
		if subject == "" {
			return mcp.NewToolResultError("subject is required"), nil
		}

		// Resolve project identifier to numeric ID via the projects endpoint
		projects, _, err := client.ListProjects(100, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
		}
		var projectID int
		for _, p := range projects {
			if p.Identifier == project {
				projectID = p.ID
				break
			}
		}
		if projectID == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("unknown project: %q", project)), nil
		}

		params := redmine.IssueCreateParams{
			ProjectID:   projectID,
			Subject:     subject,
			Description: req.GetString("description", ""),
		}

		if tracker := req.GetString("tracker", ""); tracker != "" {
			resolved, err := client.ResolveTrackerID(tracker)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid tracker: %v", err)), nil
			}
			fmt.Sscanf(resolved, "%d", &params.TrackerID)
		}

		if status := req.GetString("status", ""); status != "" {
			resolved, err := client.ResolveStatusID(status)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid status: %v", err)), nil
			}
			fmt.Sscanf(resolved, "%d", &params.StatusID)
		}

		if priorityID := req.GetInt("priority_id", 0); priorityID > 0 {
			params.PriorityID = priorityID
		}

		if assignee := req.GetString("assignee", ""); assignee != "" {
			resolved, err := client.ResolveUserID(assignee)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid assignee: %v", err)), nil
			}
			fmt.Sscanf(resolved, "%d", &params.AssignedToID)
		}

		if version := req.GetString("version", ""); version != "" {
			resolved, err := client.ResolveVersionID(project, version)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid version: %v", err)), nil
			}
			fmt.Sscanf(resolved, "%d", &params.FixedVersionID)
		}

		if parentID := req.GetInt("parent_issue_id", 0); parentID > 0 {
			params.ParentIssueID = parentID
		}

		issue, err := client.CreateIssue(params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("create failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Issue #%d created: %s\n%s/issues/%d", issue.ID, issue.Subject, client.BaseURL(), issue.ID)), nil
	})
}
