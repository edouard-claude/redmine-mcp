package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDownloadAttachment(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("download_attachment",
		mcp.WithDescription("Download an attachment from Redmine by ID. Returns images as embedded content, text/markdown as text."),
		mcp.WithNumber("attachment_id",
			mcp.Description("Attachment ID from get_attachments results"),
			mcp.Required(),
		),
		mcp.WithString("filename",
			mcp.Description("Filename of the attachment"),
			mcp.Required(),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		attachmentID := req.GetInt("attachment_id", 0)
		if attachmentID == 0 {
			return mcp.NewToolResultError("attachment_id is required"), nil
		}
		filename := req.GetString("filename", "")
		if filename == "" {
			return mcp.NewToolResultError("filename is required"), nil
		}

		body, contentType, err := client.DownloadAttachment(attachmentID, filename)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
		}

		if isImage(contentType) {
			mimeType := contentType
			if i := strings.Index(mimeType, ";"); i != -1 {
				mimeType = mimeType[:i]
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.ImageContent{
						Type:     "image",
						Data:     base64.StdEncoding.EncodeToString(body),
						MIMEType: mimeType,
					},
				},
			}, nil
		}

		if isText(contentType, filename) {
			return mcp.NewToolResultText(string(body)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Downloaded %s (%s, %d bytes). Binary content not displayed.", filename, contentType, len(body))), nil
	})
}

func isImage(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

func isText(contentType, filename string) bool {
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	lower := strings.ToLower(filename)
	for _, ext := range []string{".md", ".txt", ".json", ".csv", ".xml", ".yaml", ".yml", ".html"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
