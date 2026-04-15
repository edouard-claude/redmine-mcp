package redmine

// IDName is a common Redmine reference object (project, tracker, user, etc.).
type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Issue represents a full Redmine issue from the REST API.
type Issue struct {
	ID             int           `json:"id"`
	Project        IDName        `json:"project"`
	Tracker        IDName        `json:"tracker"`
	Status         IssueStatus   `json:"status"`
	Priority       IDName        `json:"priority"`
	Author         IDName        `json:"author"`
	AssignedTo     *IDName       `json:"assigned_to"`
	Category       *IDName       `json:"category"`
	FixedVersion   *IDName       `json:"fixed_version"`
	Parent         *IDRef        `json:"parent"`
	Subject        string        `json:"subject"`
	Description    string        `json:"description"`
	StartDate      *string       `json:"start_date"`
	DueDate        *string       `json:"due_date"`
	DoneRatio      int           `json:"done_ratio"`
	IsPrivate      bool          `json:"is_private"`
	EstimatedHours *float64      `json:"estimated_hours"`
	SpentHours     *float64      `json:"spent_hours"`
	CustomFields   []CustomField `json:"custom_fields"`
	CreatedOn      string        `json:"created_on"`
	UpdatedOn      string        `json:"updated_on"`
	ClosedOn       *string       `json:"closed_on"`
	Attachments    []Attachment  `json:"attachments"`
	Journals       []Journal     `json:"journals"`
	Children       []ChildIssue  `json:"children"`
}

// IssueStatus includes the is_closed flag.
type IssueStatus struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsClosed bool   `json:"is_closed"`
}

// IDRef is a reference with only an ID (used for parent issues).
type IDRef struct {
	ID int `json:"id"`
}

// CustomField represents a custom field value on an issue.
type CustomField struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// Attachment represents a file attached to an issue.
type Attachment struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
	ContentURL  string `json:"content_url"`
	Author      IDName `json:"author"`
	CreatedOn   string `json:"created_on"`
}

// Journal represents a comment or change entry on an issue.
type Journal struct {
	ID           int             `json:"id"`
	User         IDName          `json:"user"`
	Notes        string          `json:"notes"`
	CreatedOn    string          `json:"created_on"`
	PrivateNotes bool            `json:"private_notes"`
	Details      []JournalDetail `json:"details"`
}

// JournalDetail is a single field change within a journal entry.
type JournalDetail struct {
	Property string `json:"property"`
	Name     string `json:"name"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// ChildIssue is a lightweight child issue reference.
type ChildIssue struct {
	ID      int    `json:"id"`
	Tracker IDName `json:"tracker"`
	Subject string `json:"subject"`
}

// Project represents a Redmine project.
type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	IsPublic    bool   `json:"is_public"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

// Tracker represents a Redmine tracker (Bug, Feature, etc.).
type Tracker struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SearchResult is a single result from the /search.json endpoint.
type SearchResult struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
	DateTime    string `json:"datetime"`
}

// Version represents a project version/milestone.
type Version struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Project IDName `json:"project"`
	Status  string `json:"status"`
}

// IssueListParams holds filters for listing issues.
type IssueListParams struct {
	ProjectID    string
	StatusID     string // "open", "closed", "*", or numeric
	AssignedToID string // numeric ID or "me"
	TrackerID    string // numeric ID
	VersionID    string // numeric ID
	ParentID     string // numeric ID
	IssueIDs     string // comma-separated IDs
	Sort         string
	Limit        int
	Offset       int
}

// IssueCreateParams holds fields for creating an issue.
type IssueCreateParams struct {
	ProjectID      int    `json:"project_id"`
	TrackerID      int    `json:"tracker_id,omitempty"`
	StatusID       int    `json:"status_id,omitempty"`
	PriorityID     int    `json:"priority_id,omitempty"`
	Subject        string `json:"subject"`
	Description    string `json:"description,omitempty"`
	AssignedToID   int    `json:"assigned_to_id,omitempty"`
	ParentIssueID  int    `json:"parent_issue_id,omitempty"`
	FixedVersionID int    `json:"fixed_version_id,omitempty"`
	EstimatedHours *float64 `json:"estimated_hours,omitempty"`
}

// IssueUpdateParams holds fields for updating an issue.
type IssueUpdateParams struct {
	TrackerID      *int     `json:"tracker_id,omitempty"`
	StatusID       *int     `json:"status_id,omitempty"`
	PriorityID     *int     `json:"priority_id,omitempty"`
	Subject        *string  `json:"subject,omitempty"`
	Description    *string  `json:"description,omitempty"`
	AssignedToID   *int     `json:"assigned_to_id,omitempty"`
	ParentIssueID  *int     `json:"parent_issue_id,omitempty"`
	FixedVersionID *int     `json:"fixed_version_id,omitempty"`
	DoneRatio      *int     `json:"done_ratio,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
	PrivateNotes   *bool    `json:"private_notes,omitempty"`
	EstimatedHours *float64 `json:"estimated_hours,omitempty"`
}

// --- JSON response wrappers ---

type issueResponse struct {
	Issue Issue `json:"issue"`
}

type issuesResponse struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
}

type projectsResponse struct {
	Projects   []Project `json:"projects"`
	TotalCount int       `json:"total_count"`
}

type statusesResponse struct {
	IssueStatuses []IssueStatus `json:"issue_statuses"`
}

type trackersResponse struct {
	Trackers []Tracker `json:"trackers"`
}

type versionsResponse struct {
	Versions []Version `json:"versions"`
}

type searchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
}

type usersResponse struct {
	Users []IDName `json:"users"`
}
