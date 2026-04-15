package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGetComments(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("get_comments",
		mcp.WithDescription("Get all comments (journal notes) for a Redmine issue. Returns author, date, and content of each comment."),
		mcp.WithNumber("issue_id",
			mcp.Description("Redmine issue number"),
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

		issue, err := client.GetIssue(issueID, "journals")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get comments: %v", err)), nil
		}

		return mcp.NewToolResultText(formatComments(issueID, issue.Journals)), nil
	})
}
