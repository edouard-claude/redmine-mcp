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

	// cached reference data
	statuses []IssueStatus
	trackers []Tracker
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

// --- Attachments ---

// DownloadAttachment fetches an attachment by its ID and filename.
// Returns the body bytes and content type.
func (c *Client) DownloadAttachment(id int, filename string) ([]byte, string, error) {
	u := fmt.Sprintf("%s/attachments/download/%d/%s", c.baseURL, id, filename)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download attachment: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}

	return body, resp.Header.Get("Content-Type"), nil
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
