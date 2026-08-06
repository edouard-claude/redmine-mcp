package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// maxPreview caps how much free-form text (description, notes) is echoed back
// in a confirmation prompt, in runes.
const maxPreview = 1500

// confirmTimeout bounds how long a write waits for the user's answer.
const confirmTimeout = 10 * time.Minute

const elicitationUnsupportedMsg = "write aborted: this client does not support MCP elicitation, so the change could not be confirmed by a human. Show the change to the user and let them run the equivalent CLI command, or set REDMINE_AUTO_WRITE=1 in the server environment if the client already asks for approval before every tool call."

// clientSupportsElicitation reports whether the client advertised the
// elicitation capability at initialization. mcp-go does not check this itself
// and would block until the context expires.
func clientSupportsElicitation(ctx context.Context) bool {
	session, ok := server.ClientSessionFromContext(ctx).(server.SessionWithClientInfo)
	if !ok {
		return false
	}
	return session.GetClientCapabilities().Elicitation != nil
}

// autoWriteEnabled reports whether the confirmation step is bypassed. Set
// REDMINE_AUTO_WRITE=1 for clients that already gate every tool call
// themselves (e.g. Claude Code's permission prompt).
func autoWriteEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("REDMINE_AUTO_WRITE"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// confirmWrite asks the user to approve a write through the MCP elicitation
// flow before it reaches Redmine. It returns nil when the write may proceed,
// or the result the tool must return when it has to be aborted. Anything that
// is not an explicit approval aborts the write.
func confirmWrite(ctx context.Context, s *server.MCPServer, title string, fields []string) *mcp.CallToolResult {
	if autoWriteEnabled() {
		return nil
	}

	if !clientSupportsElicitation(ctx) {
		return mcp.NewToolResultError(elicitationUnsupportedMsg)
	}

	// mcp-go waits for the client's answer until the context is cancelled, so
	// cap the wait: a client that advertises elicitation but never answers
	// would otherwise hang the tool call forever.
	ctx, cancel := context.WithTimeout(ctx, confirmTimeout)
	defer cancel()

	var b strings.Builder
	b.WriteString(title)
	for _, f := range fields {
		if f == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(f)
	}
	b.WriteString("\n\nApply this to Redmine?")

	result, err := s.RequestElicitation(ctx, mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: b.String(),
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"confirm": map[string]any{
						"type":        "boolean",
						"title":       "Confirm",
						"description": "Yes, write this to Redmine",
					},
				},
				"required": []string{"confirm"},
			},
		},
	})
	if err != nil {
		if errors.Is(err, server.ErrElicitationNotSupported) || errors.Is(err, server.ErrNoActiveSession) {
			return mcp.NewToolResultError(elicitationUnsupportedMsg)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return mcp.NewToolResultError("write aborted: no answer from the user within 10 minutes. Nothing was written.")
		}
		return mcp.NewToolResultError(fmt.Sprintf("write aborted: confirmation request failed: %v", err))
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
	case mcp.ElicitationResponseActionDecline:
		return mcp.NewToolResultError("write aborted: the user declined the change. Do not retry without new instructions.")
	default:
		return mcp.NewToolResultError("write aborted: the user cancelled the confirmation. Do not retry without new instructions.")
	}

	content, ok := result.Content.(map[string]any)
	if !ok || !truthy(content["confirm"]) {
		return mcp.NewToolResultError("write aborted: the user did not confirm the change. Do not retry without new instructions.")
	}
	return nil
}

// intSummary renders an optional numeric field, skipping it when unset.
func intSummary(label string, value *int, format string) string {
	if value == nil {
		return ""
	}
	return label + ": " + fmt.Sprintf(format, *value)
}

// parentSummary renders an optional parent issue reference.
func parentSummary(id int) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("Parent issue: #%d", id)
}

// truthy interprets the confirmation flag leniently: clients are not all
// consistent in how they serialize a boolean form field.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1", "on":
			return true
		}
	case float64:
		return t != 0
	}
	return false
}

// summarize renders a labelled value for a confirmation prompt, truncating
// long free-form text on a rune boundary.
func summarize(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxPreview {
		return fmt.Sprintf("%s: %s… (truncated, %d characters total)", label, string(runes[:maxPreview]), len(runes))
	}
	return label + ": " + value
}
