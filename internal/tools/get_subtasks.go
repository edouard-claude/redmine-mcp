package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGetSubtasks(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("get_subtasks",
		mcp.WithDescription("Get all subtasks (child issues) of a Redmine issue."),
		mcp.WithNumber("issue_id",
			mcp.Description("Parent issue number"),
			mcp.Required(),
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

		issue, err := client.GetIssue(issueID, "children")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get subtasks: %v", err)), nil
		}

		return mcp.NewToolResultText(formatChildren(issueID, issue.Children)), nil
	})
}
