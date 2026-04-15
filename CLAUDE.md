# redmine-mcp

MCP server providing read/write access to Redmine via its REST API.

## Build & Deploy

```bash
make build     # builds ./redmine-mcp (local)
make install   # builds directly to /usr/local/bin/redmine-mcp
```

**Always use `make install`** after changes — Claude Desktop loads the binary from `/usr/local/bin/`.

## Environment Variables

- `REDMINE_URL` — Redmine web base URL (required, e.g. `https://projets.example.com`)
- `REDMINE_API_KEY` — Redmine API key for authentication (required, from My Account > API access key)

## Architecture

```
cmd/redmine-mcp/       → Entry point, MCP stdio server
internal/
  ├── redmine/         → REST API client, types, HTTP helpers
  └── tools/           → MCP tool registrations + formatting
```

## MCP Tools

### Read

| Tool | Description |
|------|-------------|
| `get_issue` | Full issue details by ID (includes attachments, journals, children) |
| `search_issues` | Search with filters (project, status, assignee, tracker, version, text) + pagination |
| `get_comments` | Journal notes for an issue |
| `get_subtasks` | Child issues of a parent |
| `get_attachments` | File attachments with download URLs |
| `download_attachment` | Download and return attachment content (images as base64, text inline) |
| `list_projects` | All accessible projects |

### Write

| Tool | Description |
|------|-------------|
| `create_issue` | Create a new issue (project + subject required) |
| `update_issue` | Update issue fields and/or add a comment |

## Conventions

- Go stdlib + mcp-go only (no database driver)
- Name-to-ID resolution: tools accept human-readable names (status, tracker, assignee, version) and resolve them to API IDs internally
- Reference data (statuses, trackers) is cached per client instance
- Errors returned as `mcp.NewToolResultError()`, not Go errors

## Gotchas

- **macOS `cp` binaire** : ne pas `cp` un binaire Go vers `/usr/local/bin/` — builder directement vers la destination avec `go build -o /usr/local/bin/`
- **Test MCP stdio** : `(printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n'; sleep 1) | /usr/local/bin/redmine-mcp` pour vérifier que le serveur répond
- **Claude Code MCP config** : géré via `claude mcp add/remove -s user`, stocké dans `~/.claude.json`
