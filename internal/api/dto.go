// Package api defines DTOs and event types shared across spk-cockpit's HTTP, CLI and UI layers.
package api

// Priority encodes a todo's importance (low to urgent).
type Priority int

// Priority levels.
const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

// TodoStatus is the lifecycle state of a todo.
type TodoStatus string

// Todo statuses.
const (
	StatusOpen       TodoStatus = "open"
	StatusInProgress TodoStatus = "in_progress"
	StatusDone       TodoStatus = "done"
	StatusCancelled  TodoStatus = "cancelled"
)

// Todo is the canonical todo DTO.
type Todo struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Notes     string     `json:"notes"`
	Priority  Priority   `json:"priority"`
	Status    TodoStatus `json:"status"`
	DueAt     *int64     `json:"dueAt,omitempty"`
	Tags      []string   `json:"tags"`
	CreatedAt int64      `json:"createdAt"`
	UpdatedAt int64      `json:"updatedAt"`
	DoneAt    *int64     `json:"doneAt,omitempty"`
	// SortOrder is the manual within-column position on the kanban board.
	// Higher values render higher. The UI computes new orders as the average
	// of neighbors when a card is dropped between two existing cards, which
	// avoids touching unrelated rows on every reorder.
	SortOrder float64 `json:"sortOrder"`
}

// Tag is a label that can be attached to multiple todos.
type Tag struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt int64  `json:"createdAt"`
}

// TodoEvent is an audit-log entry for a todo state transition.
type TodoEvent struct {
	ID        int64  `json:"id"`
	TodoID    string `json:"todoId"`
	Kind      string `json:"kind"`
	FromValue string `json:"fromValue,omitempty"`
	ToValue   string `json:"toValue,omitempty"`
	Payload   string `json:"payload,omitempty"`
	At        int64  `json:"at"`
}

// CreateTodoRequest is the body of POST /api/todos.
type CreateTodoRequest struct {
	Title    string   `json:"title"`
	Notes    string   `json:"notes,omitempty"`
	Priority Priority `json:"priority"`
	DueAt    *int64   `json:"dueAt,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// UpdateTodoRequest is the body of PATCH /api/todos/{id}; nil pointers mean "leave unchanged".
type UpdateTodoRequest struct {
	Title     *string     `json:"title,omitempty"`
	Notes     *string     `json:"notes,omitempty"`
	Priority  *Priority   `json:"priority,omitempty"`
	Status    *TodoStatus `json:"status,omitempty"`
	DueAt     *int64      `json:"dueAt,omitempty"`
	Tags      *[]string   `json:"tags,omitempty"`
	SortOrder *float64    `json:"sortOrder,omitempty"`
}

// ErrorResponse wraps an ErrorBody for parse-friendly client error reporting.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the payload of an error response.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// TimerSession is one tracking interval on a todo. EndedAt nil = currently running.
type TimerSession struct {
	ID        int64  `json:"id"`
	TodoID    string `json:"todoId"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   *int64 `json:"endedAt,omitempty"`
	Source    string `json:"source"`
}

// TodoTimeTotal returns aggregated tracked time for a todo since SinceUnix.
type TodoTimeTotal struct {
	TodoID     string `json:"todoId"`
	SinceUnix  int64  `json:"sinceUnix"`
	TotalSec   int64  `json:"totalSec"`
	SessionCnt int    `json:"sessionCount"`
	HasActive  bool   `json:"hasActive"`
}

// StartTimerRequest is the body of POST /api/timer/start.
type StartTimerRequest struct {
	TodoID string `json:"todoId"`
}

// MeetingSource indicates where a meeting came from.
type MeetingSource string

// Meeting sources.
const (
	MeetingSourceManual MeetingSource = "manual"
	MeetingSourceCalDAV MeetingSource = "caldav"
)

// Meeting is the canonical meeting DTO.
type Meeting struct {
	ID           string        `json:"id"`
	Source       MeetingSource `json:"source"`
	ExternalUID  string        `json:"externalUid,omitempty"`
	ExternalETag string        `json:"externalEtag,omitempty"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Location     string        `json:"location"`
	StartAt      int64         `json:"startAt"`
	EndAt        int64         `json:"endAt"`
	NotifyMin    *int          `json:"notifyMin,omitempty"`
	NotifiedAt   *int64        `json:"notifiedAt,omitempty"`
	PopupMin     *int          `json:"popupMin,omitempty"`
	PopupFiredAt *int64        `json:"popupFiredAt,omitempty"`
	Cancelled    bool          `json:"cancelled"`
	CreatedAt    int64         `json:"createdAt"`
	UpdatedAt    int64         `json:"updatedAt"`
}

// CreateMeetingRequest is the body of POST /api/meetings (manual only).
type CreateMeetingRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	StartAt     int64  `json:"startAt"`
	EndAt       int64  `json:"endAt"`
	NotifyMin   *int   `json:"notifyMin,omitempty"`
	PopupMin    *int   `json:"popupMin,omitempty"`
}

// UpdateMeetingRequest is the body of PATCH /api/meetings/{id}.
// Only manual meetings may be updated; nil pointers leave fields unchanged.
type UpdateMeetingRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Location    *string `json:"location,omitempty"`
	StartAt     *int64  `json:"startAt,omitempty"`
	EndAt       *int64  `json:"endAt,omitempty"`
	NotifyMin   *int    `json:"notifyMin,omitempty"`
	PopupMin    *int    `json:"popupMin,omitempty"`
}

// Note is a markdown note attached to a meeting OR a todo.
type Note struct {
	ID        string `json:"id"`
	MeetingID string `json:"meetingId,omitempty"`
	TodoID    string `json:"todoId,omitempty"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// UpsertNoteRequest is the body of PUT /api/notes (creates or updates by attachment).
// Exactly one of MeetingID / TodoID must be non-empty.
type UpsertNoteRequest struct {
	MeetingID string `json:"meetingId,omitempty"`
	TodoID    string `json:"todoId,omitempty"`
	Body      string `json:"body"`
}

// Secret describes an encrypted secret without exposing its value.
type Secret struct {
	Name      string `json:"name"`
	UpdatedAt int64  `json:"updatedAt"`
}

// SetSecretRequest is the body of PUT /api/secrets/{name}.
type SetSecretRequest struct {
	Value string `json:"value"`
}

// SyncStateEntry reports per-source sync status.
type SyncStateEntry struct {
	Source   string `json:"source"`
	Cursor   string `json:"cursor"`
	LastOkAt *int64 `json:"lastOkAt,omitempty"`
	LastErr  string `json:"lastErr,omitempty"`
}

// StandupSection categorizes a standup item.
type StandupSection string

// Standup section labels.
const (
	StandupSectionYesterday StandupSection = "yesterday"
	StandupSectionToday     StandupSection = "today"
	StandupSectionBlockers  StandupSection = "blockers"
)

// StandupItemSource identifies where an item originated.
type StandupItemSource string

// Standup item sources.
const (
	StandupSourceTodo    StandupItemSource = "todo"
	StandupSourceGitLab  StandupItemSource = "gitlab"
	StandupSourceTracker StandupItemSource = "tracker"
)

// StandupItem is one row in a standup report (todo, commit, or tracker activity).
type StandupItem struct {
	Source StandupItemSource `json:"source"`
	Title  string            `json:"title"`
	Detail string            `json:"detail,omitempty"`
	URL    string            `json:"url,omitempty"`
	RefID  string            `json:"refId,omitempty"`
	At     int64             `json:"at"`
}

// StandupReport is the full standup payload returned by GET /api/standup.
type StandupReport struct {
	Day       string        `json:"day"` // YYYY-MM-DD (local TZ of generation)
	Yesterday []StandupItem `json:"yesterday"`
	Today     []StandupItem `json:"today"`
	Blockers  []StandupItem `json:"blockers"`
	Errors    []string      `json:"errors,omitempty"` // per-source warnings, e.g. "gitlab: 401"
}
