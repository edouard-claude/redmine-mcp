# redmine-mcp

A single Go binary that exposes [Redmine](https://www.redmine.org/) through two interfaces:

- an **[MCP](https://modelcontextprotocol.io/) server** over stdio (JSON-RPC) — for Claude Desktop, Claude Code, and any MCP-aware client;
- a **plain CLI** with subcommands — composable from a shell or invoked by agents via `Bash` (the [Pi](https://pi.dev/)-style pattern that avoids preloading tool schemas).

Read and write issues, comments, attachments, and projects — pick whichever interface fits the caller.

## Features

### Read

| Tool | Description |
|------|-------------|
| `get_issue` | Full issue details (attachments, journals, children) |
| `search_issues` | Search with filters (project, status, assignee, tracker, version, text) + pagination |
| `get_comments` | Journal notes for an issue |
| `get_subtasks` | Child issues of a parent |
| `get_attachments` | File attachments with download URLs |
| `download_attachment` | Download attachment content by `attachment_id` (images inline, text inline, binaries saved to disk) |
| `list_projects` | All accessible projects |
| `search_users` | Search user accounts by name/login/email (admin API key required) |
| `list_roles` | Roles assignable to project members |
| `list_groups` | User groups (admin API key required) |
| `list_project_members` | Members (users and groups) of a project with their roles |

### Write

| Tool | Description |
|------|-------------|
| `create_issue` | Create a new issue |
| `update_issue` | Update issue fields and/or add a comment |
| `update_comment` | Edit an existing comment |
| `create_user` | Create a user account (admin API key required) |
| `add_project_member` | Add a user to a project with one or more roles |
| `add_group_user` | Add a user to a group (inherits the group's project memberships) |

Tools accept human-readable names for statuses, trackers, assignees, and versions — they are resolved to IDs automatically.

## Two ways to run it

The same binary detects how it was invoked:

```bash
redmine-mcp                          # MCP stdio server (default — Claude Desktop config)
redmine-mcp mcp                      # MCP stdio server (explicit)
redmine-mcp help                     # CLI help
redmine-mcp <command> [flags]        # CLI mode (see below)
```

### CLI mode

Every MCP tool has a kebab-case CLI equivalent. Each subcommand has its own `--help`:

```bash
redmine-mcp get-issue 7415
redmine-mcp get-issue --max-desc 5000 7415
redmine-mcp search --project apnl --status open --limit 10
redmine-mcp search --query "login crash" --project apnl
redmine-mcp get-comments 7415
redmine-mcp get-subtasks 7415
redmine-mcp get-attachments 7415
redmine-mcp download-attachment 4321 -o /tmp/screen.png
redmine-mcp list-projects
redmine-mcp create-issue --project apnl --subject "Bug login" --tracker Anomalie
redmine-mcp update-issue 7415 --status "Résolu" --notes "Fixed in v7.6.2"
redmine-mcp update-comment 98765 --notes "edited content"
redmine-mcp search-users --query dupont
redmine-mcp list-roles
redmine-mcp list-project-members --project apnl
redmine-mcp create-user --login jdupont --firstname Jean --lastname Dupont --mail j.dupont@example.com --send-info
redmine-mcp add-project-member --project apnl --user jdupont --roles "Développeur"
redmine-mcp add-group-user --group "Client X" --user jdupont
```

Conventions:

- Flags must precede positional arguments (stdlib `flag` limitation).
- Errors go to stderr; results go to stdout.
- Exit codes: `0` success, `1` operation error, `2` usage error.
- `download-attachment <id>` needs only the attachment ID — never the filename. Without `-o` it prints text to stdout and writes binaries under `$TMPDIR/redmine-attachments` (override with `REDMINE_DOWNLOAD_DIR`), printing the path. Use `-o -` for raw bytes on stdout, `-base64` for base64.

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
cmd/redmine-mcp/       → Entry point, routes args[0] → MCP server or CLI dispatcher
internal/
  ├── redmine/         → REST API client, types, HTTP helpers
  ├── tools/           → MCP tool handlers + exported formatters (FormatIssue, …)
  └── cli/             → Subcommand dispatcher (stdlib `flag`), reuses tools.Format* and redmine.Client
```

Built with:
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go MCP SDK
- Go standard library only (`net/http`, `encoding/json`, `flag`)

No database driver needed — everything goes through the Redmine REST API.

## License

MIT
