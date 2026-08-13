package tools

import (
	"context"
	"fmt"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSearchUsers(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("search_users",
		mcp.WithDescription("Search Redmine user accounts by name, login or email (requires an admin API key). Without a query, lists all users. Use this to find the user_id before add_project_member."),
		mcp.WithString("query",
			mcp.Description("Filter matching login, firstname, lastname or email (empty = all users)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 25, max 100)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Pagination offset"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := req.GetInt("limit", 25)
		if limit > 100 {
			limit = 100
		}
		users, total, err := client.ListUsers(req.GetString("query", ""), limit, req.GetInt("offset", 0))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search users failed: %v", err)), nil
		}
		return mcp.NewToolResultText(FormatUsers(users, total)), nil
	})
}

func registerListRoles(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("list_roles",
		mcp.WithDescription("List the Redmine roles assignable to project members (use the names with add_project_member)."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		roles, err := client.GetRoles()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list roles failed: %v", err)), nil
		}
		return mcp.NewToolResultText(FormatRoles(roles)), nil
	})
}

func registerListProjectMembers(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("list_project_members",
		mcp.WithDescription("List the members (users and groups) of a Redmine project with their roles."),
		mcp.WithString("project",
			mcp.Description("Project identifier (e.g. 'apnl') or numeric ID"),
			mcp.Required(),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		if project == "" {
			return mcp.NewToolResultError("project is required"), nil
		}
		members, total, err := client.ListMemberships(project, 100, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list members failed: %v", err)), nil
		}
		return mcp.NewToolResultText(FormatMemberships(project, members, total)), nil
	})
}
