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
	Title    *string     `json:"title,omitempty"`
	Notes    *string     `json:"notes,omitempty"`
	Priority *Priority   `json:"priority,omitempty"`
	Status   *TodoStatus `json:"status,omitempty"`
	DueAt    *int64      `json:"dueAt,omitempty"`
	Tags     *[]string   `json:"tags,omitempty"`
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
