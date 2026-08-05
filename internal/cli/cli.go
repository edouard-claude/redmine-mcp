// Package cli exposes the Redmine operations as subcommands of the binary,
// so the same executable can be invoked either as an MCP server (stdio JSON-RPC)
// or as a plain CLI tool composable from a shell or another agent via Bash.
package cli

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/edouard-claude/redmine-mcp/internal/redmine"
	"github.com/edouard-claude/redmine-mcp/internal/tools"
)

// Run dispatches a CLI subcommand. Returns a process exit code.
func Run(args []string, client *redmine.Client) int {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return 0
	case "get-issue":
		return cmdGetIssue(client, args[1:])
	case "search":
		return cmdSearch(client, args[1:])
	case "get-comments":
		return cmdGetComments(client, args[1:])
	case "get-subtasks":
		return cmdGetSubtasks(client, args[1:])
	case "get-attachments":
		return cmdGetAttachments(client, args[1:])
	case "download-attachment":
		return cmdDownloadAttachment(client, args[1:])
	case "list-projects":
		return cmdListProjects(client, args[1:])
	case "create-issue":
		return cmdCreateIssue(client, args[1:])
	case "update-issue":
		return cmdUpdateIssue(client, args[1:])
	case "update-comment":
		return cmdUpdateComment(client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: redmine-mcp <command> [options]

Reads:
  get-issue <id>            Full issue details
  search [filters]          Search issues (--project, --status, --query, ...)
  get-comments <id>         Journal notes for an issue
  get-subtasks <id>         Child issues
  get-attachments <id>      File attachments (metadata + URLs)
  download-attachment <id>  Fetch attachment content (-o writes to a file)
  list-projects             All accessible projects

Writes:
  create-issue              Create issue (--project, --subject required)
  update-issue <id>         Update fields and/or add a comment
  update-comment <jid>      Edit an existing comment

Server:
  mcp                       Run as MCP server over stdio (also the default
                            when invoked with no arguments)

Run 'redmine-mcp <command> --help' for command-specific options.

Environment:
  REDMINE_URL               Redmine base URL  (required)
  REDMINE_API_KEY           Redmine API key   (required)
`)
}

// --- helpers ---

func failf(format string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

// parseWithFlags parses flags first (so --help works), then requires `wantN`
// positional integer args. Returns the parsed positionals or a non-zero exit
// code to propagate. On --help, returns exitCode == 0 to signal a clean exit.
func parseWithFlags(fs *flag.FlagSet, args []string, positional string, wantN int) (ids []int, exitCode int, ok bool) {
	fs.Usage = func() {
		w := fs.Output()
		if positional != "" {
			fmt.Fprintf(w, "Usage: redmine-mcp %s [flags] %s\n", fs.Name(), positional)
		} else {
			fmt.Fprintf(w, "Usage: redmine-mcp %s [flags]\n", fs.Name())
		}
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, 0, false
		}
		return nil, 2, false
	}
	rest := fs.Args()
	if len(rest) < wantN {
		fmt.Fprintf(os.Stderr, "%s requires %d positional argument(s) (%s)\n", fs.Name(), wantN, positional)
		return nil, 2, false
	}
	if !checkNoExtraArgs(fs.Name(), rest[wantN:]) {
		return nil, 2, false
	}
	ids = make([]int, wantN)
	for i := 0; i < wantN; i++ {
		v, err := strconv.Atoi(rest[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %q is not an integer\n", fs.Name(), rest[i])
			return nil, 2, false
		}
		ids[i] = v
	}
	return ids, 0, true
}

// parseFlagsOnly parses flags for a subcommand that takes no positional args.
// Reports unexpected positionals — a common mistake is placing flags after a
// would-be positional, which Go's flag package silently drops.
func parseFlagsOnly(fs *flag.FlagSet, args []string) (exitCode int, ok bool) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: redmine-mcp %s [flags]\n", fs.Name())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	if !checkNoExtraArgs(fs.Name(), fs.Args()) {
		return 2, false
	}
	return 0, true
}

// checkNoExtraArgs reports leftover args. A leftover beginning with '-' is
// almost always a flag placed after a positional — Go's flag package stops at
// the first non-flag, so those flags silently never take effect.
func checkNoExtraArgs(cmd string, extra []string) bool {
	if len(extra) == 0 {
		return true
	}
	for _, a := range extra {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "%s: unexpected argument %q after positional — flags must precede positional args (try 'redmine-mcp %s --help')\n", cmd, a, cmd)
			return false
		}
	}
	fmt.Fprintf(os.Stderr, "%s: unexpected extra argument(s): %v\n", cmd, extra)
	return false
}

// --- reads ---

func cmdGetIssue(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("get-issue", flag.ContinueOnError)
	maxDesc := fs.Int("max-desc", 10000, "Max description characters (0 = no limit)")
	ids, code, ok := parseWithFlags(fs, args, "<id>", 1)
	if !ok {
		return code
	}

	issue, err := client.GetIssue(ids[0], "attachments", "journals", "children", "total_spent_time")
	if err != nil {
		return failf("get issue: %v", err)
	}
	fmt.Print(tools.FormatIssue(issue, *maxDesc))
	return 0
}

func cmdSearch(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	project := fs.String("project", "", "Project identifier")
	status := fs.String("status", "", "Status: 'open', 'closed', '*', or a status name")
	assignee := fs.String("assignee", "", "Assignee name or numeric ID")
	tracker := fs.String("tracker", "", "Tracker name")
	version := fs.String("version", "", "Target version name")
	query := fs.String("query", "", "Free-text search in subject/description")
	sort := fs.String("sort", "updated_on:desc", "Sort field (e.g. priority:desc)")
	limit := fs.Int("limit", 20, "Max results (max 100)")
	offset := fs.Int("offset", 0, "Pagination offset")
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *limit > 100 {
		*limit = 100
	}

	params, err := tools.BuildListParams(client, *project, *status, *assignee, *tracker, *version, *sort, *limit, *offset)
	if err != nil {
		return failf("filter error: %v", err)
	}

	if *query != "" {
		results, _, err := client.SearchText(*query, *project, 100, 0)
		if err != nil {
			return failf("search failed: %v", err)
		}
		if len(results) == 0 {
			fmt.Println("No issues found.")
			return 0
		}
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = strconv.Itoa(r.ID)
		}
		params.IssueIDs = strings.Join(ids, ",")
	}

	issues, total, err := client.ListIssues(params)
	if err != nil {
		return failf("search failed: %v", err)
	}
	fmt.Print(tools.FormatIssueSummaries(issues, *offset))
	fmt.Printf("Total: %d issue(s)\n", total)
	return 0
}

func cmdGetComments(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("get-comments", flag.ContinueOnError)
	ids, code, ok := parseWithFlags(fs, args, "<id>", 1)
	if !ok {
		return code
	}
	issue, err := client.GetIssue(ids[0], "journals")
	if err != nil {
		return failf("get comments: %v", err)
	}
	fmt.Print(tools.FormatComments(ids[0], issue.Journals))
	return 0
}

func cmdGetSubtasks(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("get-subtasks", flag.ContinueOnError)
	ids, code, ok := parseWithFlags(fs, args, "<id>", 1)
	if !ok {
		return code
	}
	issue, err := client.GetIssue(ids[0], "children")
	if err != nil {
		return failf("get subtasks: %v", err)
	}
	fmt.Print(tools.FormatChildren(ids[0], issue.Children))
	return 0
}

func cmdGetAttachments(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("get-attachments", flag.ContinueOnError)
	ids, code, ok := parseWithFlags(fs, args, "<id>", 1)
	if !ok {
		return code
	}
	issue, err := client.GetIssue(ids[0], "attachments")
	if err != nil {
		return failf("get attachments: %v", err)
	}
	fmt.Print(tools.FormatAttachments(ids[0], issue.Attachments))
	return 0
}

func cmdDownloadAttachment(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("download-attachment", flag.ContinueOnError)
	id := fs.Int("id", 0, "Attachment ID (also accepted as a positional arg)")
	out := fs.String("o", "", "Write to this path ('-' for stdout, empty for a temp dir)")
	base64Out := fs.Bool("base64", false, "Print base64 to stdout instead of writing a file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	rest := fs.Args()
	if *id == 0 && len(rest) > 0 {
		v, err := strconv.Atoi(rest[0])
		if err != nil {
			return failf("download-attachment: %q is not an attachment ID", rest[0])
		}
		*id, rest = v, rest[1:]
	}
	if !checkNoExtraArgs("download-attachment", rest) {
		return 2
	}
	if *id == 0 {
		return failf("download-attachment: an attachment ID is required (try 'redmine-mcp download-attachment --help')")
	}

	att, err := client.DownloadAttachment(*id)
	if err != nil {
		return failf("download failed: %v", err)
	}

	if *base64Out {
		fmt.Println(base64.StdEncoding.EncodeToString(att.Data))
		return 0
	}
	if *out == "-" {
		os.Stdout.Write(att.Data)
		return 0
	}
	// Text goes to stdout so it stays pipeable; binaries need a file.
	if *out == "" && (strings.HasPrefix(att.ContentType, "text/") || isTextExt(att.Filename)) {
		os.Stdout.Write(att.Data)
		return 0
	}

	path, err := tools.SaveAttachment(att, *out)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Println(path)
	fmt.Fprintf(os.Stderr, "Wrote %d bytes (%s)\n", len(att.Data), att.ContentType)
	return 0
}

func isTextExt(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range []string{".md", ".txt", ".json", ".csv", ".xml", ".yaml", ".yml", ".html", ".log", ".sql"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func cmdListProjects(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("list-projects", flag.ContinueOnError)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	projects, _, err := client.ListProjects(100, 0)
	if err != nil {
		return failf("list projects: %v", err)
	}
	fmt.Print(tools.FormatProjects(projects))
	return 0
}

// --- writes ---

func cmdCreateIssue(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("create-issue", flag.ContinueOnError)
	project := fs.String("project", "", "Project identifier (required)")
	subject := fs.String("subject", "", "Issue subject (required)")
	description := fs.String("description", "", "Description (Textile)")
	tracker := fs.String("tracker", "", "Tracker name or numeric ID")
	status := fs.String("status", "", "Initial status name or numeric ID")
	priorityID := fs.Int("priority-id", 0, "Priority numeric ID")
	assignee := fs.String("assignee", "", "Assignee name or numeric ID")
	version := fs.String("version", "", "Target version name or numeric ID")
	parentID := fs.Int("parent-id", 0, "Parent issue ID for subtasks")
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *project == "" || *subject == "" {
		return failf("create-issue: --project and --subject are required")
	}

	projects, _, err := client.ListProjects(100, 0)
	if err != nil {
		return failf("list projects: %v", err)
	}
	var projectID int
	for _, p := range projects {
		if p.Identifier == *project {
			projectID = p.ID
			break
		}
	}
	if projectID == 0 {
		return failf("unknown project: %q", *project)
	}

	params := redmine.IssueCreateParams{
		ProjectID:   projectID,
		Subject:     *subject,
		Description: *description,
	}

	if *tracker != "" {
		resolved, err := client.ResolveTrackerID(*tracker)
		if err != nil {
			return failf("invalid tracker: %v", err)
		}
		fmt.Sscanf(resolved, "%d", &params.TrackerID)
	}
	if *status != "" {
		resolved, err := client.ResolveStatusID(*status)
		if err != nil {
			return failf("invalid status: %v", err)
		}
		fmt.Sscanf(resolved, "%d", &params.StatusID)
	}
	if *priorityID > 0 {
		params.PriorityID = *priorityID
	}
	if *assignee != "" {
		resolved, err := client.ResolveUserID(*assignee)
		if err != nil {
			return failf("invalid assignee: %v", err)
		}
		fmt.Sscanf(resolved, "%d", &params.AssignedToID)
	}
	if *version != "" {
		resolved, err := client.ResolveVersionID(*project, *version)
		if err != nil {
			return failf("invalid version: %v", err)
		}
		fmt.Sscanf(resolved, "%d", &params.FixedVersionID)
	}
	if *parentID > 0 {
		params.ParentIssueID = *parentID
	}

	issue, err := client.CreateIssue(params)
	if err != nil {
		return failf("create failed: %v", err)
	}
	fmt.Printf("Issue #%d created: %s\n%s/issues/%d\n", issue.ID, issue.Subject, client.BaseURL(), issue.ID)
	return 0
}

func cmdUpdateIssue(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("update-issue", flag.ContinueOnError)
	notes := fs.String("notes", "", "Add a comment")
	subject := fs.String("subject", "", "New subject")
	description := fs.String("description", "", "New description")
	status := fs.String("status", "", "New status name or numeric ID")
	assignee := fs.String("assignee", "", "New assignee name or numeric ID")
	tracker := fs.String("tracker", "", "New tracker name or numeric ID")
	doneRatio := fs.Int("done-ratio", -1, "Completion percentage (0-100)")
	priorityID := fs.Int("priority-id", 0, "New priority numeric ID")
	ids, code, ok := parseWithFlags(fs, args, "<id>", 1)
	if !ok {
		return code
	}
	id := ids[0]

	if fs.NFlag() == 0 {
		return failf("update-issue: no fields to change — provide at least one flag (try 'redmine-mcp update-issue --help')")
	}

	var params redmine.IssueUpdateParams
	if *notes != "" {
		params.Notes = notes
	}
	if *subject != "" {
		params.Subject = subject
	}
	if *description != "" {
		params.Description = description
	}
	if *status != "" {
		resolved, err := client.ResolveStatusID(*status)
		if err != nil {
			return failf("invalid status: %v", err)
		}
		statuses, err := client.GetStatuses()
		if err != nil {
			return failf("get statuses: %v", err)
		}
		for _, s := range statuses {
			if strconv.Itoa(s.ID) == resolved {
				sid := s.ID
				params.StatusID = &sid
				break
			}
		}
		if params.StatusID == nil {
			return failf("status %q resolved to %q which is not numeric — use a specific status name", *status, resolved)
		}
	}
	if *assignee != "" {
		resolved, err := client.ResolveUserID(*assignee)
		if err != nil {
			return failf("invalid assignee: %v", err)
		}
		var aid int
		fmt.Sscanf(resolved, "%d", &aid)
		if aid > 0 {
			params.AssignedToID = &aid
		}
	}
	if *tracker != "" {
		resolved, err := client.ResolveTrackerID(*tracker)
		if err != nil {
			return failf("invalid tracker: %v", err)
		}
		var tid int
		fmt.Sscanf(resolved, "%d", &tid)
		if tid > 0 {
			params.TrackerID = &tid
		}
	}
	if *doneRatio >= 0 {
		params.DoneRatio = doneRatio
	}
	if *priorityID > 0 {
		params.PriorityID = priorityID
	}

	if err := client.UpdateIssue(id, params); err != nil {
		return failf("update failed: %v", err)
	}
	fmt.Printf("Issue #%d updated.\n", id)
	return 0
}

func cmdUpdateComment(client *redmine.Client, args []string) int {
	fs := flag.NewFlagSet("update-comment", flag.ContinueOnError)
	notes := fs.String("notes", "", "New comment content (required)")
	ids, code, ok := parseWithFlags(fs, args, "<journal-id>", 1)
	if !ok {
		return code
	}
	if *notes == "" {
		return failf("update-comment: --notes is required")
	}
	if err := client.UpdateJournal(ids[0], *notes); err != nil {
		return failf("update failed: %v", err)
	}
	fmt.Printf("Comment #%d updated.\n", ids[0])
	return 0
}
