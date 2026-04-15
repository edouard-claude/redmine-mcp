package tools

import (
	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all MCP tools on the server.
func RegisterAll(s *server.MCPServer, client *redmine.Client) {
	registerGetIssue(s, client)
	registerSearchIssues(s, client)
	registerGetComments(s, client)
	registerGetSubtasks(s, client)
	registerGetAttachments(s, client)
	registerDownloadAttachment(s, client)
	registerListProjects(s, client)
	registerCreateIssue(s, client)
	registerUpdateIssue(s, client)
	registerUpdateComment(s, client)
}
