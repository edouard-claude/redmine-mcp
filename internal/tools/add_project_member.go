package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAddProjectMember(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("add_project_member",
		mcp.WithDescription("Add an existing user to a Redmine project with one or more roles (requires the 'manage members' permission or an admin API key). Use search_users to find the user and list_roles for the role names."),
		mcp.WithString("project",
			mcp.Description("Project identifier (e.g. 'apnl') or numeric ID"),
			mcp.Required(),
		),
		mcp.WithString("user",
			mcp.Description("User login, name or numeric ID"),
			mcp.Required(),
		),
		mcp.WithString("roles",
			mcp.Description("Comma-separated role names or numeric IDs (e.g. 'Développeur' or 'Manager,Rapporteur')"),
			mcp.Required(),
		),
		withChangeSummary(),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		user := req.GetString("user", "")
		roles := req.GetString("roles", "")
		if project == "" || user == "" || roles == "" {
			return mcp.NewToolResultError("project, user and roles are required"), nil
		}
		why, abort := requireChangeSummary(req)
		if abort != nil {
			return abort, nil
		}

		resolved, err := client.ResolveUserID(user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid user: %v", err)), nil
		}
		userID, err := strconv.Atoi(resolved)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("user %q resolved to %q which is not a numeric ID", user, resolved)), nil
		}

		roleIDs, err := client.ResolveRoleIDs(roles)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid roles: %v", err)), nil
		}

		if abort := confirmWrite(ctx, s,
			fmt.Sprintf("Add a member to project %q on %s", project, client.BaseURL()),
			why,
			[]string{
				summarize("User", fmt.Sprintf("%s (id %d)", user, userID)),
				summarize("Roles", roles),
			}); abort != nil {
			return abort, nil
		}

		m, err := client.AddMembership(project, userID, roleIDs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("add member failed: %v", err)), nil
		}

		names := make([]string, len(m.Roles))
		for i, r := range m.Roles {
			names[i] = r.Name
		}
		who := user
		if m.User != nil && m.User.Name != "" {
			who = m.User.Name
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s added to project %s with role(s): %s", who, project, strings.Join(names, ", "))), nil
	})
}
