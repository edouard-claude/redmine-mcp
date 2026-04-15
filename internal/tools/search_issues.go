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

func registerSearchIssues(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("search_issues",
		mcp.WithDescription("Search Redmine issues with filters. Returns a paginated list of issue summaries. Use 'status' with 'open' or 'closed' to filter by state, or a specific status name. Combine multiple filters to narrow results."),
		mcp.WithString("project",
			mcp.Description("Filter by project identifier (e.g. 'fidelatoo')"),
		),
		mcp.WithString("status",
			mcp.Description("Filter by status: 'open', 'closed', '*', or a specific status name (e.g. 'En cours')"),
		),
		mcp.WithString("assignee",
			mcp.Description("Filter by assignee name or numeric ID (e.g. 'Edouard Claude' or '42')"),
		),
		mcp.WithString("tracker",
			mcp.Description("Filter by tracker name (e.g. 'Anomalie', 'Evolution')"),
		),
		mcp.WithString("version",
			mcp.Description("Filter by target version name (e.g. 'V7.6')"),
		),
		mcp.WithString("query",
			mcp.Description("Free-text search in subject and description"),
		),
		mcp.WithString("sort",
			mcp.Description("Sort field (e.g. 'updated_on:desc', 'priority:desc', 'created_on:asc')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 20, max: 100)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset for pagination (default: 0)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		status := req.GetString("status", "")
		assignee := req.GetString("assignee", "")
		tracker := req.GetString("tracker", "")
		version := req.GetString("version", "")
		query := req.GetString("query", "")
		sort := req.GetString("sort", "updated_on:desc")
		limit := req.GetInt("limit", 20)
		offset := req.GetInt("offset", 0)

		if limit > 100 {
			limit = 100
		}

		// If text search is provided, use the search endpoint to find matching issue IDs,
		// then fetch them with filters via the issues endpoint.
		if query != "" {
			return searchWithText(client, project, status, assignee, tracker, version, query, sort, limit, offset)
		}

		params, err := buildListParams(client, project, status, assignee, tracker, version, sort, limit, offset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("filter error: %v", err)), nil
		}

		issues, total, err := client.ListIssues(params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		result := formatIssueSummaries(issues, offset)
		result += fmt.Sprintf("Total: %d issue(s)\n", total)
		return mcp.NewToolResultText(result), nil
	})
}

func buildListParams(client *redmine.Client, project, status, assignee, tracker, version, sort string, limit, offset int) (redmine.IssueListParams, error) {
	params := redmine.IssueListParams{
		ProjectID: project,
		Sort:      sort,
		Limit:     limit,
		Offset:    offset,
	}

	if status != "" {
		resolved, err := client.ResolveStatusID(status)
		if err != nil {
			return params, err
		}
		params.StatusID = resolved
	}

	if tracker != "" {
		resolved, err := client.ResolveTrackerID(tracker)
		if err != nil {
			return params, err
		}
		params.TrackerID = resolved
	}

	if assignee != "" {
		resolved, err := client.ResolveUserID(assignee)
		if err != nil {
			return params, err
		}
		params.AssignedToID = resolved
	}

	if version != "" {
		resolved, err := client.ResolveVersionID(project, version)
		if err != nil {
			return params, err
		}
		params.VersionID = resolved
	}

	return params, nil
}

func searchWithText(client *redmine.Client, project, status, assignee, tracker, version, query, sort string, limit, offset int) (*mcp.CallToolResult, error) {
	// Use Redmine search to find matching issue IDs
	results, _, err := client.SearchText(query, project, 100, 0)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No issues found."), nil
	}

	// Collect issue IDs
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = strconv.Itoa(r.ID)
	}

	// Fetch full issue data with any additional filters
	params, err := buildListParams(client, project, status, assignee, tracker, version, sort, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("filter error: %v", err)), nil
	}
	params.IssueIDs = strings.Join(ids, ",")

	issues, total, err := client.ListIssues(params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	result := formatIssueSummaries(issues, offset)
	result += fmt.Sprintf("Total: %d issue(s) (text search: %d matches)\n", total, len(results))
	return mcp.NewToolResultText(result), nil
}
