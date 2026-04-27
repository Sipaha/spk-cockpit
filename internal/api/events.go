package api

// Event type names used over SSE / event bus.
const (
	EventTodoCreated       = "todo.created"
	EventTodoUpdated       = "todo.updated"
	EventTodoStatusChanged = "todo.status_changed"
	EventTodoDeleted       = "todo.deleted"
	EventTimerStarted      = "timer.started"
	EventTimerStopped      = "timer.stopped"
)

// Event is the envelope sent over SSE and the in-process bus.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// TodoCreatedData is the payload of EventTodoCreated.
type TodoCreatedData struct {
	Todo Todo `json:"todo"`
}

// TodoUpdatedData is the payload of EventTodoUpdated.
type TodoUpdatedData struct {
	Todo          Todo     `json:"todo"`
	ChangedFields []string `json:"changedFields"`
}

// TodoStatusChangedData is the payload of EventTodoStatusChanged.
type TodoStatusChangedData struct {
	TodoID string     `json:"todoId"`
	From   TodoStatus `json:"from"`
	To     TodoStatus `json:"to"`
}

// TodoDeletedData is the payload of EventTodoDeleted.
type TodoDeletedData struct {
	TodoID string `json:"todoId"`
}

// TimerStartedData is the payload of EventTimerStarted.
type TimerStartedData struct {
	TodoID    string `json:"todoId"`
	SessionID int64  `json:"sessionId"`
	StartedAt int64  `json:"startedAt"`
}

// TimerStoppedData is the payload of EventTimerStopped.
type TimerStoppedData struct {
	TodoID      string `json:"todoId"`
	SessionID   int64  `json:"sessionId"`
	EndedAt     int64  `json:"endedAt"`
	DurationSec int64  `json:"durationSec"`
}
