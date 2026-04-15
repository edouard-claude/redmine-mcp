# redmine-mcp

An [MCP](https://modelcontextprotocol.io/) server that connects AI assistants (Claude Desktop, Claude Code, etc.) to [Redmine](https://www.redmine.org/) via its REST API.

Read and write issues, comments, attachments, and projects — directly from your AI workflow.

## Features

### Read

| Tool | Description |
|------|-------------|
| `get_issue` | Full issue details (attachments, journals, children) |
| `search_issues` | Search with filters (project, status, assignee, tracker, version, text) + pagination |
| `get_comments` | Journal notes for an issue |
| `get_subtasks` | Child issues of a parent |
| `get_attachments` | File attachments with download URLs |
| `download_attachment` | Download attachment content (images as base64, text inline) |
| `list_projects` | All accessible projects |

### Write

| Tool | Description |
|------|-------------|
| `create_issue` | Create a new issue |
| `update_issue` | Update issue fields and/or add a comment |
| `update_comment` | Edit an existing comment |

Tools accept human-readable names for statuses, trackers, assignees, and versions — they are resolved to IDs automatically.

## Prerequisites

- Go 1.25+
- A Redmine instance (tested with Redmine 5.x)
- A Redmine API key

## Getting your Redmine API Key

1. Log in to your Redmine instance
2. Navigate to **My Account** (top-right menu → *My account*, or go to `https://your-redmine.com/my/account`)
3. In the right sidebar, find the **API access key** section
4. Click **Show** to reveal your existing key, or **Reset** to generate a new one
5. Copy the key

> **Note:** If you don't see the API access key section, your Redmine administrator may need to enable the REST API. This is done in **Administration → Settings → API tab → Enable REST web service**.

## Installation

### From source

```bash
git clone https://github.com/edouard-claude/redmine-mcp.git
cd redmine-mcp
make install   # builds and installs to /usr/local/bin/redmine-mcp
```

### Go install

```bash
go install github.com/edouard-claude/redmine-mcp/cmd/redmine-mcp@latest
```

## Configuration

The server requires two environment variables:

| Variable | Description |
|----------|-------------|
| `REDMINE_URL` | Your Redmine base URL (e.g. `https://redmine.example.com`) |
| `REDMINE_API_KEY` | Your API key (see above) |

### Claude Code

```bash
claude mcp add redmine -s user \
  -e REDMINE_URL=https://redmine.example.com \
  -e REDMINE_API_KEY=your_api_key_here \
  -- /usr/local/bin/redmine-mcp
```

Verify with:

```bash
claude mcp get redmine
```

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "redmine": {
      "command": "/usr/local/bin/redmine-mcp",
      "env": {
        "REDMINE_URL": "https://redmine.example.com",
        "REDMINE_API_KEY": "your_api_key_here"
      }
    }
  }
}
```

Config file location:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

## Usage examples

Once connected, you can ask your AI assistant things like:

- *"Show me issue #1234"*
- *"Search for open bugs in the backend project"*
- *"What are the latest comments on issue #5678?"*
- *"Create a bug report for the login page crash"*
- *"Update issue #1234 status to In Progress and assign it to me"*
- *"Download the screenshot attached to issue #5678"*

## Architecture

```
cmd/redmine-mcp/       → Entry point (MCP stdio server)
internal/
  ├── redmine/         → REST API client, types, HTTP helpers
  └── tools/           → MCP tool handlers + formatting
```

Built with:
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go MCP SDK
- Go standard library (`net/http`, `encoding/json`)

No database driver needed — everything goes through the Redmine REST API.

## License

MIT
