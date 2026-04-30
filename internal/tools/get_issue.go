package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGetIssue(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("get_issue",
		mcp.WithDescription("Get the full details of a Redmine issue by its ID. Returns subject, status, tracker, priority, project, version, author, assignee, dates, completion ratio, and description."),
		mcp.WithNumber("issue_id",
			mcp.Description("Redmine issue number (e.g. 5871)"),
			mcp.Required(),
		),
		mcp.WithNumber("max_description_chars",
			mcp.Description("Max characters for description (default: 10000, 0 = no limit)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		issueID := req.GetInt("issue_id", 0)
		if issueID == 0 {
			return mcp.NewToolResultError("issue_id is required"), nil
		}
		maxDesc := req.GetInt("max_description_chars", 10000)

		issue, err := client.GetIssue(issueID, "attachments", "journals", "children", "total_spent_time")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get issue: %v", err)), nil
		}

		return mcp.NewToolResultText(formatIssue(issue, maxDesc)), nil
	})
}
