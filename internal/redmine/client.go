package redmine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is an authenticated Redmine REST API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	// dlClient has a longer timeout: attachments can be tens of megabytes.
	dlClient *http.Client

	// cached reference data
	statuses []IssueStatus
	trackers []Tracker
	roles    []IDName
}

// NewClient creates a Redmine client from REDMINE_URL and REDMINE_API_KEY.
func NewClient() (*Client, error) {
	baseURL := os.Getenv("REDMINE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("REDMINE_URL is not set")
	}
	apiKey := os.Getenv("REDMINE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("REDMINE_API_KEY is not set")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dlClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// BaseURL returns the Redmine base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// --- HTTP helpers ---

func (c *Client) get(path string, params url.Values, out interface{}) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) sendJSON(method, path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// --- Issues ---

// GetIssue fetches a single issue by ID with optional includes.
func (c *Client) GetIssue(id int, includes ...string) (*Issue, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", strings.Join(includes, ","))
	}
	var resp issueResponse
	if err := c.get(fmt.Sprintf("/issues/%d.json", id), params, &resp); err != nil {
		return nil, fmt.Errorf("get issue #%d: %w", id, err)
	}
	return &resp.Issue, nil
}

// ListIssues fetches issues with filters and pagination.
func (c *Client) ListIssues(p IssueListParams) ([]Issue, int, error) {
	params := url.Values{}
	if p.ProjectID != "" {
		params.Set("project_id", p.ProjectID)
	}
	if p.StatusID != "" {
		params.Set("status_id", p.StatusID)
	}
	if p.AssignedToID != "" {
		params.Set("assigned_to_id", p.AssignedToID)
	}
	if p.TrackerID != "" {
		params.Set("tracker_id", p.TrackerID)
	}
	if p.VersionID != "" {
		params.Set("fixed_version_id", p.VersionID)
	}
	if p.ParentID != "" {
		params.Set("parent_id", p.ParentID)
	}
	if p.IssueIDs != "" {
		params.Set("issue_id", p.IssueIDs)
	}
	if p.Sort != "" {
		params.Set("sort", p.Sort)
	}
	if p.Limit > 0 {
		params.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		params.Set("offset", strconv.Itoa(p.Offset))
	}

	var resp issuesResponse
	if err := c.get("/issues.json", params, &resp); err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	return resp.Issues, resp.TotalCount, nil
}

// SearchText searches Redmine for issues matching a text query.
func (c *Client) SearchText(query, projectID string, limit, offset int) ([]SearchResult, int, error) {
	params := url.Values{
		"q":           {query},
		"issues":      {"1"},
		"attachments": {"0"},
	}
	if projectID != "" {
		params.Set("scope", "subprojects")
		// search within project context
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/search.json"
	if projectID != "" {
		path = fmt.Sprintf("/projects/%s/search.json", projectID)
	}

	var resp searchResponse
	if err := c.get(path, params, &resp); err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}

	// filter to issues only
	var issues []SearchResult
	for _, r := range resp.Results {
		if r.Type == "issue" || r.Type == "issue-closed" {
			issues = append(issues, r)
		}
	}
	return issues, resp.TotalCount, nil
}

// CreateIssue creates a new issue and returns it.
func (c *Client) CreateIssue(p IssueCreateParams) (*Issue, error) {
	payload := map[string]interface{}{"issue": p}
	resp, err := c.sendJSON("POST", "/issues.json", payload)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create issue: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode created issue: %w", err)
	}
	return &result.Issue, nil
}

// UpdateIssue updates an existing issue (fields + optional comment via Notes).
func (c *Client) UpdateIssue(id int, p IssueUpdateParams) error {
	payload := map[string]interface{}{"issue": p}
	resp, err := c.sendJSON("PUT", fmt.Sprintf("/issues/%d.json", id), payload)
	if err != nil {
		return fmt.Errorf("update issue #%d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update issue #%d: HTTP %d: %s", id, resp.StatusCode, string(body))
	}
	return nil
}

// UpdateJournal edits the notes of an existing journal entry (comment).
func (c *Client) UpdateJournal(journalID int, notes string) error {
	payload := map[string]any{"journal": map[string]any{"notes": notes}}
	resp, err := c.sendJSON("PUT", fmt.Sprintf("/journals/%d.json", journalID), payload)
	if err != nil {
		return fmt.Errorf("update journal #%d: %w", journalID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update journal #%d: HTTP %d: %s", journalID, resp.StatusCode, string(body))
	}
	return nil
}

// --- Projects ---

// ListProjects fetches all accessible projects with pagination.
func (c *Client) ListProjects(limit, offset int) ([]Project, int, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var resp projectsResponse
	if err := c.get("/projects.json", params, &resp); err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	return resp.Projects, resp.TotalCount, nil
}

// --- Users & memberships ---

// ListUsers fetches user accounts, optionally filtered by name (matches login,
// firstname, lastname, mail). Requires an admin API key.
func (c *Client) ListUsers(name string, limit, offset int) ([]User, int, error) {
	params := url.Values{}
	if name != "" {
		params.Set("name", name)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	var resp usersResponse
	if err := c.get("/users.json", params, &resp); err != nil {
		return nil, 0, fmt.Errorf("list users (admin API key required): %w", err)
	}
	return resp.Users, resp.TotalCount, nil
}

// CreateUser creates a user account. Requires an admin API key. sendInfo asks
// Redmine to email the credentials to the new user; in the API payload it is a
// sibling of the user hash, not part of it.
func (c *Client) CreateUser(p UserCreateParams, sendInfo bool) (*User, error) {
	payload := map[string]interface{}{"user": p}
	if sendInfo {
		payload["send_information"] = true
	}
	resp, err := c.sendJSON("POST", "/users.json", payload)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create user: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result userResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode created user: %w", err)
	}
	return &result.User, nil
}

// ListMemberships fetches the members (users and groups) of a project.
func (c *Client) ListMemberships(projectID string, limit, offset int) ([]Membership, int, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	var resp membershipsResponse
	if err := c.get(fmt.Sprintf("/projects/%s/memberships.json", projectID), params, &resp); err != nil {
		return nil, 0, fmt.Errorf("list memberships for %s: %w", projectID, err)
	}
	return resp.Memberships, resp.TotalCount, nil
}

// AddMembership adds a user to a project with the given roles. Requires the
// "manage members" permission on the project (or an admin API key).
func (c *Client) AddMembership(projectID string, userID int, roleIDs []int) (*Membership, error) {
	payload := map[string]interface{}{
		"membership": map[string]interface{}{
			"user_id":  userID,
			"role_ids": roleIDs,
		},
	}
	resp, err := c.sendJSON("POST", fmt.Sprintf("/projects/%s/memberships.json", projectID), payload)
	if err != nil {
		return nil, fmt.Errorf("add member to %s: %w", projectID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("add member to %s: HTTP %d: %s", projectID, resp.StatusCode, string(body))
	}

	var result membershipResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode created membership: %w", err)
	}
	return &result.Membership, nil
}

// GetGroups fetches all user groups. Requires an admin API key.
func (c *Client) GetGroups() ([]IDName, error) {
	var resp groupsResponse
	if err := c.get("/groups.json", nil, &resp); err != nil {
		return nil, fmt.Errorf("get groups (admin API key required): %w", err)
	}
	return resp.Groups, nil
}

// ResolveGroupID resolves a group name or numeric ID.
func (c *Client) ResolveGroupID(name string) (int, error) {
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	groups, err := c.GetGroups()
	if err != nil {
		return 0, err
	}
	for _, g := range groups {
		if strings.EqualFold(g.Name, name) {
			return g.ID, nil
		}
	}
	return 0, fmt.Errorf("unknown group: %q (list_groups shows the valid names)", name)
}

// AddGroupUser adds a user to a group; the user inherits every project
// membership the group has. Requires an admin API key.
func (c *Client) AddGroupUser(groupID, userID int) error {
	payload := map[string]interface{}{"user_id": userID}
	resp, err := c.sendJSON("POST", fmt.Sprintf("/groups/%d/users.json", groupID), payload)
	if err != nil {
		return fmt.Errorf("add user to group #%d: %w", groupID, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("add user to group #%d: HTTP %d: %s", groupID, resp.StatusCode, string(body))
}

// GetRoles fetches and caches all roles assignable to project members.
func (c *Client) GetRoles() ([]IDName, error) {
	if c.roles != nil {
		return c.roles, nil
	}
	var resp rolesResponse
	if err := c.get("/roles.json", nil, &resp); err != nil {
		return nil, fmt.Errorf("get roles: %w", err)
	}
	c.roles = resp.Roles
	return c.roles, nil
}

// ResolveRoleIDs resolves a comma-separated list of role names or numeric IDs.
func (c *Client) ResolveRoleIDs(names string) ([]int, error) {
	var ids []int
	for _, part := range strings.Split(names, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
			continue
		}
		roles, err := c.GetRoles()
		if err != nil {
			return nil, err
		}
		found := 0
		for _, r := range roles {
			if strings.EqualFold(r.Name, part) {
				found = r.ID
				break
			}
		}
		if found == 0 {
			return nil, fmt.Errorf("unknown role: %q (list_roles shows the valid names)", part)
		}
		ids = append(ids, found)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	return ids, nil
}

// --- Attachments ---

// AttachmentContent is a downloaded attachment: its metadata plus the bytes.
type AttachmentContent struct {
	Attachment
	Data []byte
}

// GetAttachment fetches attachment metadata by ID (filename, content type, size,
// content_url). This is the authoritative source for the filename — the one
// echoed back by a caller is often Unicode-normalized differently and would 404.
func (c *Client) GetAttachment(id int) (*Attachment, error) {
	var resp attachmentResponse
	if err := c.get(fmt.Sprintf("/attachments/%d.json", id), nil, &resp); err != nil {
		return nil, fmt.Errorf("get attachment #%d: %w", id, err)
	}
	return &resp.Attachment, nil
}

// DownloadAttachment fetches an attachment's content by ID alone.
//
// Redmine's download route is /attachments/download/:id[/:filename]; the
// filename segment is cosmetic but must match the stored name byte-for-byte
// when present. Filenames with accents are stored NFD-decomposed while clients
// typically send NFC, so any caller-supplied name is a 404 waiting to happen —
// we omit the segment entirely and fall back to the server's own content_url.
func (c *Client) DownloadAttachment(id int) (*AttachmentContent, error) {
	meta, err := c.GetAttachment(id)
	if err != nil {
		return nil, err
	}

	candidates := []string{fmt.Sprintf("%s/attachments/download/%d", c.baseURL, id)}
	if meta.ContentURL != "" {
		candidates = append(candidates, meta.ContentURL)
	}

	var lastErr error
	for _, u := range candidates {
		body, ctype, err := c.fetchRaw(u)
		if err != nil {
			lastErr = err
			continue
		}
		// A Redmine that refuses the key on non-.json routes serves the login
		// page with HTTP 200; the declared filesize is the reliable tell.
		if meta.Filesize > 0 && int64(len(body)) != meta.Filesize {
			lastErr = fmt.Errorf("got %d bytes for a %d-byte attachment (content-type %s) — check API key permissions", len(body), meta.Filesize, ctype)
			continue
		}
		if ctype == "" {
			ctype = meta.ContentType
		}
		out := &AttachmentContent{Attachment: *meta, Data: body}
		out.ContentType = ctype
		return out, nil
	}
	return nil, fmt.Errorf("download attachment #%d: %w", id, lastErr)
}

// fetchRaw performs an authenticated GET and returns the raw body.
func (c *Client) fetchRaw(u string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)

	resp, err := c.dlClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d on %s", resp.StatusCode, u)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}

	ctype := resp.Header.Get("Content-Type")
	if i := strings.Index(ctype, ";"); i != -1 {
		ctype = strings.TrimSpace(ctype[:i])
	}
	return body, ctype, nil
}

// --- Reference data (cached) ---

// GetStatuses fetches and caches all issue statuses.
func (c *Client) GetStatuses() ([]IssueStatus, error) {
	if c.statuses != nil {
		return c.statuses, nil
	}
	var resp statusesResponse
	if err := c.get("/issue_statuses.json", nil, &resp); err != nil {
		return nil, fmt.Errorf("get statuses: %w", err)
	}
	c.statuses = resp.IssueStatuses
	return c.statuses, nil
}

// GetTrackers fetches and caches all trackers.
func (c *Client) GetTrackers() ([]Tracker, error) {
	if c.trackers != nil {
		return c.trackers, nil
	}
	var resp trackersResponse
	if err := c.get("/trackers.json", nil, &resp); err != nil {
		return nil, fmt.Errorf("get trackers: %w", err)
	}
	c.trackers = resp.Trackers
	return c.trackers, nil
}

// GetVersions fetches versions for a project.
func (c *Client) GetVersions(projectID string) ([]Version, error) {
	var resp versionsResponse
	if err := c.get(fmt.Sprintf("/projects/%s/versions.json", projectID), nil, &resp); err != nil {
		return nil, fmt.Errorf("get versions for %s: %w", projectID, err)
	}
	return resp.Versions, nil
}

// ResolveStatusID resolves a status name to its ID.
// Accepts "open", "closed", "*" as-is, or matches by name.
func (c *Client) ResolveStatusID(name string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "open" || lower == "closed" || lower == "*" {
		return lower, nil
	}
	statuses, err := c.GetStatuses()
	if err != nil {
		return "", err
	}
	for _, s := range statuses {
		if strings.EqualFold(s.Name, name) {
			return strconv.Itoa(s.ID), nil
		}
	}
	// try as numeric ID
	if _, err := strconv.Atoi(name); err == nil {
		return name, nil
	}
	return "", fmt.Errorf("unknown status: %q", name)
}

// ResolveTrackerID resolves a tracker name to its numeric ID string.
func (c *Client) ResolveTrackerID(name string) (string, error) {
	// try as numeric ID first
	if _, err := strconv.Atoi(name); err == nil {
		return name, nil
	}
	trackers, err := c.GetTrackers()
	if err != nil {
		return "", err
	}
	for _, t := range trackers {
		if strings.EqualFold(t.Name, name) {
			return strconv.Itoa(t.ID), nil
		}
	}
	return "", fmt.Errorf("unknown tracker: %q", name)
}

// ResolveVersionID resolves a version name to its numeric ID string.
func (c *Client) ResolveVersionID(projectID, name string) (string, error) {
	if _, err := strconv.Atoi(name); err == nil {
		return name, nil
	}
	if projectID == "" {
		return "", fmt.Errorf("project is required to resolve version name %q", name)
	}
	versions, err := c.GetVersions(projectID)
	if err != nil {
		return "", err
	}
	for _, v := range versions {
		if strings.EqualFold(v.Name, name) {
			return strconv.Itoa(v.ID), nil
		}
	}
	return "", fmt.Errorf("unknown version %q in project %s", name, projectID)
}

// ResolveUserID tries to resolve a username/login to a numeric ID string.
// Falls through to treating the input as a numeric ID if user search fails.
func (c *Client) ResolveUserID(name string) (string, error) {
	if _, err := strconv.Atoi(name); err == nil {
		return name, nil
	}
	if strings.EqualFold(name, "me") {
		return "me", nil
	}
	// try the users endpoint (may require admin)
	params := url.Values{"name": {name}, "limit": {"1"}}
	var resp usersResponse
	if err := c.get("/users.json", params, &resp); err == nil && len(resp.Users) > 0 {
		return strconv.Itoa(resp.Users[0].ID), nil
	}
	return "", fmt.Errorf("cannot resolve user %q (try numeric ID)", name)
}
