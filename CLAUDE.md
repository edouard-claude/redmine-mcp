# redmine-mcp

Single binary exposing Redmine via two interfaces:
- **MCP stdio server** (JSON-RPC over stdin/stdout) — default mode, compatible with Claude Desktop and any MCP client.
- **CLI subcommands** (Pi-style, composable via Bash) — same operations, invoked as `redmine-mcp <command> [flags]`.

## Build & Deploy

```bash
make build     # builds ./redmine-mcp (local)
make install   # builds directly to /usr/local/bin/redmine-mcp
```

**Always use `make install`** after changes — Claude Desktop loads the binary from `/usr/local/bin/`.

## Environment Variables

- `REDMINE_URL` — Redmine web base URL (required, e.g. `https://projets.example.com`)
- `REDMINE_API_KEY` — Redmine API key for authentication (required, from My Account > API access key)
- `REDMINE_AUTO_WRITE` — set to `1` to bypass the human confirmation on write tools (see below)

## Architecture

```
cmd/redmine-mcp/       → Entry point, routes args[0] → MCP server or CLI
internal/
  ├── redmine/         → REST API client, types, HTTP helpers
  ├── tools/           → MCP tool registrations + exported formatters (FormatIssue, FormatComments, …)
  └── cli/             → CLI dispatcher (flag-based subcommands), reuses tools.Format* and redmine.Client
```

Routing in `cmd/redmine-mcp/main.go`:
- no args, `mcp`, or `serve` → `server.ServeStdio` (MCP mode, Claude Desktop)
- any other first arg → `cli.Run` (e.g. `redmine-mcp get-issue 1234`)

## CLI subcommands

Each MCP tool has a CLI equivalent (kebab-case). Help is per-command:

```bash
redmine-mcp                          # → MCP stdio server
redmine-mcp mcp                      # → MCP stdio (explicit)
redmine-mcp help                     # → top-level help
redmine-mcp get-issue --help         # → flags for a subcommand
redmine-mcp get-issue 7415
redmine-mcp search --project apnl --status open --limit 5
redmine-mcp create-issue --project apnl --subject "..."
redmine-mcp update-issue 7415 --notes "comment" --status "Résolu"
```

Flags must precede positional args (stdlib `flag` limitation).

## MCP Tools

### Read

| Tool | Description |
|------|-------------|
| `get_issue` | Full issue details by ID (includes attachments, journals, children) |
| `search_issues` | Search with filters (project, status, assignee, tracker, version, text) + pagination |
| `get_comments` | Journal notes for an issue |
| `get_subtasks` | Child issues of a parent |
| `get_attachments` | File attachments with download URLs |
| `download_attachment` | Download attachment content by `attachment_id` alone (images inline, text inline, binaries saved to disk) |
| `list_projects` | All accessible projects |

### Write

| Tool | Description |
|------|-------------|
| `create_issue` | Create a new issue (project + subject required) |
| `update_issue` | Update issue fields and/or add a comment |
| `update_comment` | Edit an existing journal note |

## Human-in-the-loop (write tools)

`internal/tools/confirm.go` gates the three write tools: before anything reaches
Redmine, `confirmWrite` sends an `elicitation/create` request showing exactly
what will be written (subject, changed fields, full note/description text) and
waits for an explicit `confirm: true`.

- **Fail-closed** — decline, cancel, a missing `confirm` field, a client that
  never advertised the `elicitation` capability, or no answer within 10 minutes
  all abort the write.
- The client capability is checked *before* sending: mcp-go does not check it
  and would block the tool call until the context expires.
- `REDMINE_AUTO_WRITE=1` skips the whole flow — for clients that already prompt
  before every tool call.
- The CLI path (`internal/cli`) is deliberately not gated: the human is the one
  typing the command.

## Conventions

- Go stdlib + mcp-go only (no database driver)
- Name-to-ID resolution: tools accept human-readable names (status, tracker, assignee, version) and resolve them to API IDs internally
- Reference data (statuses, trackers) is cached per client instance
- Errors returned as `mcp.NewToolResultError()`, not Go errors

## Gotchas

- **macOS `cp` binaire** : ne pas `cp` un binaire Go vers `/usr/local/bin/` — builder directement vers la destination avec `go build -o /usr/local/bin/`
- **Test MCP stdio** : `(printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n'; sleep 1) | /usr/local/bin/redmine-mcp` pour vérifier que le serveur répond
- **Claude Code MCP config** : géré via `claude mcp add/remove -s user`, stocké dans `~/.claude.json`
- **Téléchargement de pièces jointes** : ne jamais mettre le nom de fichier dans l'URL. `/attachments/download/:id/:filename` exige une correspondance octet pour octet, or Redmine stocke les noms accentués en Unicode NFD (`e` + U+0301) alors que les clients renvoient du NFC → 404 systématique. Utiliser `/attachments/download/:id` (sans nom) et `/attachments/:id.json` pour les métadonnées.
- **Login HTML en HTTP 200** : sur les routes non-`.json`, un Redmine qui refuse la clé API sert la page de login avec un code 200. Comparer la taille reçue au `filesize` des métadonnées pour détecter le cas.
