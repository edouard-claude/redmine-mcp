package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerUpdateComment(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("update_comment",
		mcp.WithDescription("Edit an existing comment (journal entry) on a Redmine issue. Use get_comments first to find the journal ID."),
		mcp.WithNumber("journal_id",
			mcp.Description("Journal ID (from get_comments results)"),
			mcp.Required(),
		),
		mcp.WithString("notes",
			mcp.Description("New content for the comment"),
			mcp.Required(),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalID := req.GetInt("journal_id", 0)
		if journalID == 0 {
			return mcp.NewToolResultError("journal_id is required"), nil
		}
		notes := req.GetString("notes", "")
		if notes == "" {
			return mcp.NewToolResultError("notes is required"), nil
		}

		if err := client.UpdateJournal(journalID, notes); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Comment #%d updated successfully.", journalID)), nil
	})
}
