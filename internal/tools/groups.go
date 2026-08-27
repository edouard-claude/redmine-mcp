package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListGroups(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("list_groups",
		mcp.WithDescription("List Redmine user groups (requires an admin API key). Adding a user to a group gives them every project membership the group has."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		groups, err := client.GetGroups()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list groups failed: %v", err)), nil
		}
		return mcp.NewToolResultText(FormatGroups(groups)), nil
	})
}

func registerAddGroupUser(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("add_group_user",
		mcp.WithDescription("Add an existing user to a Redmine group (requires an admin API key). The user inherits every project membership and role the group has — prefer this over add_project_member when a project's access is managed through a group."),
		mcp.WithString("group",
			mcp.Description("Group name or numeric ID"),
			mcp.Required(),
		),
		mcp.WithString("user",
			mcp.Description("User login, name or numeric ID"),
			mcp.Required(),
		),
		withChangeSummary(),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		group := req.GetString("group", "")
		user := req.GetString("user", "")
		if group == "" || user == "" {
			return mcp.NewToolResultError("group and user are required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}

		groupID, err := client.ResolveGroupID(group)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid group: %v", err)), nil
		}
		resolved, err := client.ResolveUserID(user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid user: %v", err)), nil
		}
		userID, err := strconv.Atoi(resolved)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("user %q resolved to %q which is not a numeric ID", user, resolved)), nil
		}

		if abort := confirmWrite(ctx, s,
			fmt.Sprintf("Add a user to group %q on %s", group, client.BaseURL()),
			why,
			[]string{
				summarize("User", fmt.Sprintf("%s (id %d)", user, userID)),
				summarize("Group", fmt.Sprintf("%s (id %d)", group, groupID)),
			}); abort != nil {
			return abort, nil
		}

		if err := client.AddGroupUser(groupID, userID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("add to group failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s added to group %s. They now inherit the group's project memberships.", user, group)), nil
	})
}
