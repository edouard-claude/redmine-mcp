package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListProjects(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("list_projects",
		mcp.WithDescription("List all active Redmine projects with their identifiers. Use the identifier when filtering issues by project."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projects, _, err := client.ListProjects(100, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
		}

		return mcp.NewToolResultText(formatProjects(projects)), nil
	})
}
