package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// maxInlineImage caps raw bytes returned as inline image content. Base64
	// inflates by ~4/3 and the model API rejects images above 5 MB encoded.
	maxInlineImage = 3_500_000
	// maxInlineText caps characters returned inline for text attachments.
	maxInlineText = 100_000
)

func registerDownloadAttachment(s *server.MCPServer, client *redmine.Client) {
	tool := mcp.NewTool("download_attachment",
		mcp.WithDescription("Download an attachment from Redmine by ID. Images are returned inline as viewable content; text is returned inline; PDFs and other binaries are saved to disk and the local path is returned (open it with the Read tool)."),
		mcp.WithNumber("attachment_id",
			mcp.Description("Attachment ID from get_attachments results"),
			mcp.Required(),
		),
		mcp.WithString("save_path",
			mcp.Description("Optional absolute path to write the file to. Defaults to a temp directory. Any binary attachment is always written to disk."),
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

		att, err := client.DownloadAttachment(attachmentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download failed: %v", err)), nil
		}

		savePath := req.GetString("save_path", "")

		switch {
		case isImage(att.ContentType) && len(att.Data) <= maxInlineImage:
			return mcp.NewToolResultImage(
				fmt.Sprintf("%s (%s, %d bytes)", att.Filename, att.ContentType, len(att.Data)),
				base64.StdEncoding.EncodeToString(att.Data),
				att.ContentType,
			), nil

		case isText(att.ContentType, att.Filename):
			text := string(att.Data)
			if len(text) > maxInlineText {
				path, werr := SaveAttachment(att, savePath)
				if werr != nil {
					return mcp.NewToolResultError(werr.Error()), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf(
					"%s is %d bytes — showing the first %d characters. Full file saved to %s\n\n%s",
					att.Filename, len(att.Data), maxInlineText, path, text[:maxInlineText])), nil
			}
			return mcp.NewToolResultText(text), nil
		}

		path, err := SaveAttachment(att, savePath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hint := "Read it with the Read tool."
		if isImage(att.ContentType) {
			hint = "Too large to inline; read it with the Read tool."
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Saved %s (%s, %d bytes) to:\n%s\n\n%s", att.Filename, att.ContentType, len(att.Data), path, hint)), nil
	})
}

// SaveAttachment writes the attachment to dest, or to a temp directory when
// dest is empty, and returns the absolute path written.
func SaveAttachment(att *redmine.AttachmentContent, dest string) (string, error) {
	// Issue attachments are frequently confidential and the default directory
	// sits in a world-readable, predictably-named /tmp — keep both the
	// directory and the file readable only by the invoking user.
	if dest == "" {
		dir := os.Getenv("REDMINE_DOWNLOAD_DIR")
		if dir == "" {
			dir = filepath.Join(os.TempDir(), "redmine-attachments")
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create %s: %w", dir, err)
		}
		dest = filepath.Join(dir, fmt.Sprintf("%d-%s", att.ID, safeFilename(att.Filename)))
	} else if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(dest, att.Data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", dest, err)
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return dest, nil
	}
	return abs, nil
}

// safeFilename strips path separators and control characters so a Redmine-side
// filename can never escape the download directory.
func safeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == os.PathSeparator || r == '/' || r == '\\' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimLeft(name, ".")
	if name == "" {
		return "attachment"
	}
	return name
}

func isImage(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

func isText(contentType, filename string) bool {
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	switch contentType {
	case "application/json", "application/xml", "application/x-yaml", "application/yaml":
		return true
	}
	lower := strings.ToLower(filename)
	for _, ext := range []string{".md", ".txt", ".json", ".csv", ".xml", ".yaml", ".yml", ".html", ".log", ".sql", ".go", ".js", ".ts", ".py", ".sh"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
