package main

import (
	"fmt"
	"os"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/edouard-claude/redmine-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

var version = "2.0.0"

func main() {
	client, err := redmine.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "redmine client: %v\n", err)
		os.Exit(1)
	}

	s := server.NewMCPServer(
		"redmine-mcp",
		version,
		server.WithInstructions("Redmine access via REST API. Query and manage issues, comments, attachments, and projects."),
	)

	tools.RegisterAll(s, client)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
