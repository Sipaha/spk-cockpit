# spk-cockpit Phase 2: Time-tracking + Popover + Quick-add parser

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add time-tracking on todos (start/stop sessions, daily aggregation, single-active-globally invariant), a compact popover layout for the existing window, a tray tooltip that reflects the active timer, and an inline quick-add parser (`Title !urgent #tag due:tomorrow`) wired into the add-todo form.

**Architecture:** Timer is a new domain (`internal/timer`) following the same shape as `internal/todo` — repo interfaces, SQLite + fake implementations, conformance tests, a service that emits domain events. HTTP API exposes start/stop/active/total endpoints. CLI gets a `cockpit timer` subcommand. Web gets a TimerBadge component and start/stop buttons on each todo row, both driven by SSE events. Popover is a new React route `/popover` rendering a compact layout in the same Wails window — a true frameless tray-anchored window is deferred to Phase 3 (requires Wails v3 multi-window). Quick-add parser lives in `web/src/lib/parser.ts` with unit tests, used by `AddTodoForm`. Tray tooltip is updated by a goroutine subscribing to the event bus.

**Tech Stack:** Go 1.22+, modernc.org/sqlite, Wails v2 (existing), fyne.io/systray (existing), React 19, react-router-dom 7+ (NEW), Vitest, Tailwind 4.

**Reference Phase 1 plan:** `docs/superpowers/plans/2026-04-27-spk-cockpit-phase1-foundation.md`. Phase 2 builds directly on top of Phase 1 — assume all Phase 1 packages exist.

**Phase 1 lessons (carried over conventions):**

- **Build tags:** `make build-fast` uses `-tags "webkit2_41 production"`. Phase 2 must not break this.
- **Doc comments:** every exported type/func/const must have a doc comment (`revive`). Group constants share one comment.
- **No `Co-Authored-By`** in commit messages.
- **Don't build into project root.** Use `/tmp/cockpit-build-check` or `make build` (which writes to `build/bin/`).
- **`//nolint:revive`** with explanatory comment is acceptable for `pkg.PkgType` stutter (e.g., `timer.TimerRepo`).
- **`//nolint:gosec`** with comment is acceptable for `LIMIT %d` with controlled int and `os.ReadFile(pidFile)` patterns.
- **All connections** go through the existing `*sql.DB`. SQLite is opened with `MaxOpenConns(1)` + `PRAGMA foreign_keys=ON` etc. — don't touch this.
- **All mutations** go through a domain service that emits an audit event AND publishes a domain event to the EventBus.
- **Tests** use `t.TempDir()` and `clock.NewFake(t0)`.
- **Conformance tests** in `internal/store/conformance_test.go` run identical assertions against fake + sqlite repos.

---

## Out of scope for Phase 2

- Frameless tray-anchored popover window (requires Wails v3 multi-window — Phase 3+).
- Meeting card in popover (no meeting domain yet — Phase 3).
- Pomodoro / focus timer with break enforcement (different feature — not in MVP).
- Daily aggregation report UI (button "show today's hours" appears, but the aggregation page itself is YAGNI for now — basic per-todo total is enough for v1).

---

## File Structure (changes from Phase 1)

```
spk-task-manager/
├── internal/
│   ├── store/
│   │   ├── migrations/
│   │   │   └── 0002_timer_sessions.sql            # NEW
│   │   ├── timer_repo.go                          # NEW
│   │   └── conformance_test.go                    # MODIFIED (timer cases)
│   ├── timer/                                     # NEW package
│   │   ├── service.go
│   │   ├── repo.go                                # TimerRepo interface, errors, types
│   │   ├── service_test.go
│   │   └── fakerepo/
│   │       └── timer_repo.go
│   ├── api/
│   │   ├── dto.go                                 # MODIFIED (add TimerSession DTO)
│   │   └── events.go                              # MODIFIED (timer event types — already declared in Phase 1, just confirm)
│   ├── server/
│   │   ├── server.go                              # MODIFIED (Deps gains Timer *timer.Service)
│   │   ├── routes.go                              # MODIFIED (5 new routes)
│   │   └── timer_handler.go                       # NEW
│   ├── cli/
│   │   ├── start.go                               # MODIFIED (wire timer service + tooltip subscriber)
│   │   └── timer.go                               # NEW (cobra subcommands)
│   └── tray/
│       └── tooltip.go                             # NEW (goroutine subscribing to bus)
└── web/
    ├── package.json                               # MODIFIED (add react-router-dom)
    └── src/
        ├── App.tsx                                # MODIFIED (BrowserRouter + Routes)
        ├── lib/
        │   ├── types.ts                           # MODIFIED (TimerSession, parser-related)
        │   ├── api.ts                             # MODIFIED (timer methods)
        │   ├── store.ts                           # MODIFIED (timer slice + apply timer events)
        │   ├── parser.ts                          # NEW
        │   └── parser.test.ts                     # NEW
        ├── components/
        │   ├── TodoRow.tsx                        # MODIFIED (Start/Stop button)
        │   ├── AddTodoForm.tsx                    # MODIFIED (use parser)
        │   └── TimerBadge.tsx                     # NEW
        └── pages/
            ├── Todos.tsx                          # MODIFIED (subscribe to timer events too)
            └── Popover.tsx                        # NEW
```

---

## Task 1: Migration 0002 — `timer_sessions` table

**Files:**
- Create: `internal/store/migrations/0002_timer_sessions.sql`

- [ ] **Step 1.1: Write the migration**

```sql
CREATE TABLE timer_sessions (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id    TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at   INTEGER,
  source     TEXT NOT NULL DEFAULT 'manual'
);
CREATE INDEX idx_timer_todo ON timer_sessions(todo_id, started_at);
CREATE UNIQUE INDEX uq_timer_active ON timer_sessions(todo_id) WHERE ended_at IS NULL;
```

The partial unique index enforces "one active session per todo" at the database level. The "one globally" rule is enforced in the domain service.

- [ ] **Step 1.2: Verify migrations apply**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
go test ./internal/store/...
```

Expected: `TestMigrate_AppliesOnFreshDB` and `TestMigrate_IsIdempotent` still PASS. Schema now includes `timer_sessions`.

- [ ] **Step 1.3: Update existing migration test to assert the new table**

Edit `internal/store/migrate_test.go`. In `TestMigrate_AppliesOnFreshDB`, replace the line:

```go
for _, table := range []string{"todos", "tags", "todo_tags", "todo_events", "kv"} {
```

with:

```go
for _, table := range []string{"todos", "tags", "todo_tags", "todo_events", "kv", "timer_sessions"} {
```

And replace `require.Equal(t, []int{1}, versions)` with:

```go
require.Equal(t, []int{1, 2}, versions)
```

- [ ] **Step 1.4: Run tests + commit**

```bash
go test ./internal/store/...
golangci-lint run
git add internal/store/migrations/ internal/store/migrate_test.go
git commit -m "feat: add timer_sessions migration"
```

---

## Task 2: Define API DTOs and timer event constants

**Files:**
- Modify: `internal/api/dto.go`
- Modify: `internal/api/events.go`

- [ ] **Step 2.1: Append to `internal/api/dto.go`**

Append at the end of the file (after `ErrorBody`):

```go
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
	TodoID      string `json:"todoId"`
	SinceUnix   int64  `json:"sinceUnix"`
	TotalSec    int64  `json:"totalSec"`
	SessionCnt  int    `json:"sessionCount"`
	HasActive   bool   `json:"hasActive"`
}

// StartTimerRequest is the body of POST /api/timer/start.
type StartTimerRequest struct {
	TodoID string `json:"todoId"`
}
```

- [ ] **Step 2.2: Append to `internal/api/events.go`**

Phase 1 already declared `EventTodoCreated` etc. — add timer events. Find the `const (...)` block of event names and add two new entries inside it:

```go
	EventTimerStarted      = "timer.started"
	EventTimerStopped      = "timer.stopped"
```

Append data structs at the end of the file:

```go
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
```

- [ ] **Step 2.3: Verify and commit**

```bash
go build ./internal/api/...
golangci-lint run
git add internal/api/
git commit -m "feat: add timer DTOs and event types"
```

---

## Task 3: Define `TimerRepo` interface and errors

**Files:**
- Create: `internal/timer/repo.go`

- [ ] **Step 3.1: Write `internal/timer/repo.go`**

```go
// Package timer holds the time-tracking domain (service, repository contract, errors).
package timer

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	// ErrSessionNotFound is returned when a session id does not exist.
	ErrSessionNotFound = errors.New("timer: session not found")
	// ErrNoActiveSession is returned when Stop is called and nothing is running.
	ErrNoActiveSession = errors.New("timer: no active session")
	// ErrAlreadyActiveOnTodo is returned by repo.Start when the partial unique index trips.
	// Service-level Start handles the "one active globally" rule by stopping any current
	// session first, so this error from the repo indicates a bug.
	ErrAlreadyActiveOnTodo = errors.New("timer: another active session exists on this todo")
)

// TimerRepo persists time-tracking sessions. //nolint:revive // domain naming intentional
type TimerRepo interface {
	// Start inserts a new session with EndedAt nil and returns its id. If the
	// partial unique index trips, ErrAlreadyActiveOnTodo is returned.
	Start(ctx context.Context, todoID string, startedAt int64, source string) (int64, error)

	// Stop sets EndedAt on the active session of todoID. Returns the updated row.
	// If no active session exists, ErrNoActiveSession.
	Stop(ctx context.Context, todoID string, endedAt int64) (api.TimerSession, error)

	// Active returns the currently-active session across all todos (one expected),
	// or (nil, nil) if no timer is active.
	Active(ctx context.Context) (*api.TimerSession, error)

	// ListByTodo returns all sessions for a todo, newest first.
	ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error)

	// TotalForTodo returns aggregated seconds for completed sessions of todoID
	// with started_at >= sinceUnix. Active (unfinished) session is NOT included.
	TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (int64, int, error)
}
```

- [ ] **Step 3.2: Verify and commit**

```bash
go build ./internal/timer/...
golangci-lint run
git add internal/timer/repo.go
git commit -m "feat: define timer repository interface"
```

---

## Task 4: SQLite TimerRepo

**Files:**
- Create: `internal/store/timer_repo.go`

- [ ] **Step 4.1: Write `internal/store/timer_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

// TimerRepo is the SQLite-backed implementation of timer.TimerRepo. //nolint:revive // domain naming intentional
type TimerRepo struct {
	db *sql.DB
}

// NewTimerRepo constructs a TimerRepo over db.
func NewTimerRepo(db *sql.DB) *TimerRepo { return &TimerRepo{db: db} }

// Start inserts an active session.
func (r *TimerRepo) Start(ctx context.Context, todoID string, startedAt int64, source string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO timer_sessions(todo_id, started_at, source) VALUES (?, ?, ?)`,
		todoID, startedAt, source,
	)
	if err != nil {
		// Detect partial unique index violation: SQLITE_CONSTRAINT_UNIQUE.
		var serr *sqlite.Error
		if errors.As(err, &serr) && strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, timer.ErrAlreadyActiveOnTodo
		}
		return 0, fmt.Errorf("insert timer_session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// Stop sets ended_at on the single active row for todoID.
func (r *TimerRepo) Stop(ctx context.Context, todoID string, endedAt int64) (api.TimerSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.TimerSession{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`SELECT id, todo_id, started_at, ended_at, source
		 FROM timer_sessions WHERE todo_id = ? AND ended_at IS NULL`, todoID)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.TimerSession{}, timer.ErrNoActiveSession
	}
	if err != nil {
		return api.TimerSession{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE timer_sessions SET ended_at = ? WHERE id = ?`, endedAt, s.ID); err != nil {
		return api.TimerSession{}, fmt.Errorf("update timer_session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.TimerSession{}, fmt.Errorf("commit tx: %w", err)
	}
	s.EndedAt = &endedAt
	return s, nil
}

// Active returns the current active session, or (nil, nil) if none.
func (r *TimerRepo) Active(ctx context.Context) (*api.TimerSession, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, todo_id, started_at, ended_at, source
		 FROM timer_sessions WHERE ended_at IS NULL LIMIT 1`)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListByTodo returns sessions newest first.
func (r *TimerRepo) ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	q := `SELECT id, todo_id, started_at, ended_at, source
		FROM timer_sessions WHERE todo_id = ? ORDER BY started_at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit) //nolint:gosec // limit is a controlled int
	}
	rows, err := r.db.QueryContext(ctx, q, todoID)
	if err != nil {
		return nil, fmt.Errorf("query timer_sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.TimerSession
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TotalForTodo aggregates completed sessions only (active session excluded).
func (r *TimerRepo) TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (int64, int, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(ended_at - started_at), 0), COUNT(*)
		 FROM timer_sessions
		 WHERE todo_id = ? AND ended_at IS NOT NULL AND started_at >= ?`,
		todoID, sinceUnix,
	)
	var total int64
	var count int
	if err := row.Scan(&total, &count); err != nil {
		return 0, 0, fmt.Errorf("aggregate sessions: %w", err)
	}
	return total, count, nil
}

type sessionScanner interface {
	Scan(...any) error
}

func scanSession(s sessionScanner) (api.TimerSession, error) {
	var x api.TimerSession
	var ended sql.NullInt64
	if err := s.Scan(&x.ID, &x.TodoID, &x.StartedAt, &ended, &x.Source); err != nil {
		return api.TimerSession{}, err
	}
	if ended.Valid {
		v := ended.Int64
		x.EndedAt = &v
	}
	return x, nil
}
```

- [ ] **Step 4.2: Verify and commit**

```bash
go build ./internal/store/...
golangci-lint run
git add internal/store/timer_repo.go
git commit -m "feat: implement SQLite TimerRepo"
```

---

## Task 5: Fake TimerRepo + conformance tests

**Files:**
- Create: `internal/timer/fakerepo/timer_repo.go`
- Modify: `internal/store/conformance_test.go`

- [ ] **Step 5.1: Write `internal/timer/fakerepo/timer_repo.go`**

```go
// Package fakerepo provides an in-memory timer.TimerRepo for unit tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

// Timer is an in-memory timer.TimerRepo.
type Timer struct {
	mu     sync.Mutex
	nextID int64
	rows   map[int64]api.TimerSession
}

// NewTimer creates an empty in-memory timer repo.
func NewTimer() *Timer { return &Timer{rows: map[int64]api.TimerSession{}} }

// Start inserts a session.
func (r *Timer) Start(_ context.Context, todoID string, startedAt int64, source string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.rows {
		if s.TodoID == todoID && s.EndedAt == nil {
			return 0, timer.ErrAlreadyActiveOnTodo
		}
	}
	r.nextID++
	r.rows[r.nextID] = api.TimerSession{
		ID:        r.nextID,
		TodoID:    todoID,
		StartedAt: startedAt,
		Source:    source,
	}
	return r.nextID, nil
}

// Stop closes the active session for todoID.
func (r *Timer) Stop(_ context.Context, todoID string, endedAt int64) (api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.rows {
		if s.TodoID == todoID && s.EndedAt == nil {
			s.EndedAt = &endedAt
			r.rows[id] = s
			return s, nil
		}
	}
	return api.TimerSession{}, timer.ErrNoActiveSession
}

// Active returns the single active session (one expected by domain invariant).
func (r *Timer) Active(_ context.Context) (*api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.rows {
		if s.EndedAt == nil {
			s := s
			return &s, nil
		}
	}
	return nil, nil
}

// ListByTodo returns sessions newest first.
func (r *Timer) ListByTodo(_ context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TimerSession
	for _, s := range r.rows {
		if s.TodoID == todoID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt != out[j].StartedAt {
			return out[i].StartedAt > out[j].StartedAt
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TotalForTodo aggregates completed sessions.
func (r *Timer) TotalForTodo(_ context.Context, todoID string, sinceUnix int64) (int64, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var total int64
	var count int
	for _, s := range r.rows {
		if s.TodoID != todoID || s.EndedAt == nil || s.StartedAt < sinceUnix {
			continue
		}
		total += *s.EndedAt - s.StartedAt
		count++
	}
	return total, count, nil
}
```

- [ ] **Step 5.2: Append timer conformance to `internal/store/conformance_test.go`**

Append at the end of the file (keep existing tests intact):

```go
type timerRepoCase struct {
	name string
	new  func(t *testing.T) timer.TimerRepo
}

func timerRepoCases() []timerRepoCase {
	return []timerRepoCase{
		{
			name: "fake",
			new:  func(_ *testing.T) timer.TimerRepo { return timerfake.NewTimer() },
		},
		{
			name: "sqlite",
			new: func(t *testing.T) timer.TimerRepo {
				dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
				s, err := Open(dsn)
				require.NoError(t, err)
				t.Cleanup(func() { _ = s.Close() })
				require.NoError(t, Migrate(s.DB))
				return NewTimerRepo(s.DB)
			},
		},
	}
}

func TestTimerRepo_Conformance(t *testing.T) {
	for _, c := range timerRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			active, err := r.Active(ctx)
			require.NoError(t, err)
			require.Nil(t, active)

			id, err := r.Start(ctx, "todo-1", 100, "manual")
			require.NoError(t, err)
			require.NotZero(t, id)

			active, err = r.Active(ctx)
			require.NoError(t, err)
			require.NotNil(t, active)
			require.Equal(t, "todo-1", active.TodoID)
			require.Nil(t, active.EndedAt)

			// Cannot start twice on same todo without stopping.
			_, err = r.Start(ctx, "todo-1", 110, "manual")
			require.ErrorIs(t, err, timer.ErrAlreadyActiveOnTodo)

			s, err := r.Stop(ctx, "todo-1", 160)
			require.NoError(t, err)
			require.NotNil(t, s.EndedAt)
			require.Equal(t, int64(160), *s.EndedAt)

			active, err = r.Active(ctx)
			require.NoError(t, err)
			require.Nil(t, active)

			total, cnt, err := r.TotalForTodo(ctx, "todo-1", 0)
			require.NoError(t, err)
			require.Equal(t, int64(60), total)
			require.Equal(t, 1, cnt)

			// Stop with nothing running -> error.
			_, err = r.Stop(ctx, "todo-1", 200)
			require.ErrorIs(t, err, timer.ErrNoActiveSession)
		})
	}
}
```

Add to the existing `import` block at the top of `conformance_test.go`:

```go
"github.com/spk/spk-cockpit/internal/timer"
timerfake "github.com/spk/spk-cockpit/internal/timer/fakerepo"
```

(`timerfake` alias avoids clashing with `fakerepo` already imported for todos.)

- [ ] **Step 5.3: Run conformance and commit**

```bash
go test ./internal/store/... -v -run TestTimerRepo_Conformance
go test ./internal/...
golangci-lint run
git add internal/timer/fakerepo/ internal/store/conformance_test.go
git commit -m "feat: add fake TimerRepo and conformance tests"
```

Expected output: both `fake` and `sqlite` subtests PASS.

---

## Task 6: Timer domain service (TDD)

**Files:**
- Create: `internal/timer/service.go`
- Create: `internal/timer/service_test.go`

The service enforces "one active timer globally" — `Start(todoID)` first stops any existing active session before starting a new one. It also publishes `EventTimerStarted` and `EventTimerStopped` to the bus.

- [ ] **Step 6.1: Write `internal/timer/service_test.go`**

```go
package timer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/timer/fakerepo"
)

func newTimerSvc(t *testing.T, t0 time.Time) (*timer.Service, *fakerepo.Timer) {
	r := fakerepo.NewTimer()
	c := clock.NewFake(t0)
	return timer.NewService(r, c, nil), r
}

func TestService_Start_AssignsAndReturnsSession(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()

	got, err := s.Start(ctx, "todo-1")
	require.NoError(t, err)
	require.Equal(t, "todo-1", got.TodoID)
	require.Equal(t, t0.Unix(), got.StartedAt)
	require.Nil(t, got.EndedAt)
}

func TestService_Start_StopsExistingFirst(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newTimerSvc(t, t0)
	ctx := context.Background()

	first, err := s.Start(ctx, "todo-A")
	require.NoError(t, err)

	// Advance fake clock by 5 minutes.
	c := s.Clock().(*clock.Fake)
	c.Advance(5 * time.Minute)

	second, err := s.Start(ctx, "todo-B")
	require.NoError(t, err)
	require.Equal(t, "todo-B", second.TodoID)

	// First session should now be closed in the repo.
	sessions, err := r.ListByTodo(ctx, "todo-A", 10)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.NotNil(t, sessions[0].EndedAt)
	require.Equal(t, first.ID, sessions[0].ID)

	active, err := r.Active(ctx)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, "todo-B", active.TodoID)
}

func TestService_Stop_ReturnsClosedSessionAndDuration(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()

	_, err := s.Start(ctx, "todo-1")
	require.NoError(t, err)

	c := s.Clock().(*clock.Fake)
	c.Advance(90 * time.Second)

	stopped, dur, err := s.Stop(ctx)
	require.NoError(t, err)
	require.NotNil(t, stopped.EndedAt)
	require.Equal(t, int64(90), dur)
	require.Equal(t, "todo-1", stopped.TodoID)
}

func TestService_Stop_NoActiveReturnsErrNoActive(t *testing.T) {
	s, _ := newTimerSvc(t, time.Now())
	_, _, err := s.Stop(context.Background())
	require.ErrorIs(t, err, timer.ErrNoActiveSession)
}

func TestService_Active_NilWhenNothingRunning(t *testing.T) {
	s, _ := newTimerSvc(t, time.Now())
	got, err := s.Active(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestService_TotalForTodo_AggregatesCompleted(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()
	c := s.Clock().(*clock.Fake)

	_, _ = s.Start(ctx, "todo-1")
	c.Advance(60 * time.Second)
	_, _, _ = s.Stop(ctx)

	c.Advance(10 * time.Second)
	_, _ = s.Start(ctx, "todo-1")
	c.Advance(30 * time.Second)
	_, _, _ = s.Stop(ctx)

	total, err := s.TotalForTodo(ctx, "todo-1", 0)
	require.NoError(t, err)
	require.Equal(t, int64(90), total.TotalSec)
	require.Equal(t, 2, total.SessionCnt)
	require.False(t, total.HasActive)
}
```

- [ ] **Step 6.2: Run tests to confirm they fail**

```bash
go test ./internal/timer/...
```

Expected: compile error — `timer.NewService`, `Service.Start`, etc. undefined.

- [ ] **Step 6.3: Write `internal/timer/service.go`**

```go
package timer

import (
	"context"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. May be nil — service is nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the time-tracking domain service.
type Service struct {
	repo  TimerRepo
	clock clock.Clock
	bus   EventPublisher
}

// NewService constructs the service.
func NewService(r TimerRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{repo: r, clock: c, bus: bus}
}

// Clock exposes the injected clock (used by tests).
func (s *Service) Clock() clock.Clock { return s.clock }

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// Start begins tracking time on todoID. If a different timer is already running,
// it is stopped first (one-active-globally rule).
func (s *Service) Start(ctx context.Context, todoID string) (api.TimerSession, error) {
	if cur, err := s.repo.Active(ctx); err != nil {
		return api.TimerSession{}, fmt.Errorf("active: %w", err)
	} else if cur != nil {
		if cur.TodoID == todoID {
			// Already running on this todo — return as-is, no-op.
			return *cur, nil
		}
		// Stop the other.
		if _, _, err := s.stopActive(ctx, *cur); err != nil {
			return api.TimerSession{}, err
		}
	}

	now := s.clock.Now().Unix()
	id, err := s.repo.Start(ctx, todoID, now, "manual")
	if err != nil {
		return api.TimerSession{}, fmt.Errorf("start: %w", err)
	}
	session := api.TimerSession{ID: id, TodoID: todoID, StartedAt: now, Source: "manual"}
	s.publish(api.EventTimerStarted, api.TimerStartedData{
		TodoID: todoID, SessionID: id, StartedAt: now,
	})
	return session, nil
}

// Stop ends the currently-active session, returning it and its duration in seconds.
func (s *Service) Stop(ctx context.Context) (api.TimerSession, int64, error) {
	cur, err := s.repo.Active(ctx)
	if err != nil {
		return api.TimerSession{}, 0, fmt.Errorf("active: %w", err)
	}
	if cur == nil {
		return api.TimerSession{}, 0, ErrNoActiveSession
	}
	return s.stopActive(ctx, *cur)
}

func (s *Service) stopActive(ctx context.Context, cur api.TimerSession) (api.TimerSession, int64, error) {
	now := s.clock.Now().Unix()
	stopped, err := s.repo.Stop(ctx, cur.TodoID, now)
	if err != nil {
		return api.TimerSession{}, 0, fmt.Errorf("stop: %w", err)
	}
	dur := *stopped.EndedAt - stopped.StartedAt
	s.publish(api.EventTimerStopped, api.TimerStoppedData{
		TodoID: stopped.TodoID, SessionID: stopped.ID, EndedAt: *stopped.EndedAt, DurationSec: dur,
	})
	return stopped, dur, nil
}

// Active returns the currently-running session or nil.
func (s *Service) Active(ctx context.Context) (*api.TimerSession, error) {
	return s.repo.Active(ctx)
}

// TotalForTodo aggregates completed sessions and reports active state.
func (s *Service) TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (api.TodoTimeTotal, error) {
	total, cnt, err := s.repo.TotalForTodo(ctx, todoID, sinceUnix)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	active, err := s.repo.Active(ctx)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	hasActive := active != nil && active.TodoID == todoID
	return api.TodoTimeTotal{
		TodoID:     todoID,
		SinceUnix:  sinceUnix,
		TotalSec:   total,
		SessionCnt: cnt,
		HasActive:  hasActive,
	}, nil
}

// ListSessions returns recent sessions for a todo.
func (s *Service) ListSessions(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	return s.repo.ListByTodo(ctx, todoID, limit)
}
```

- [ ] **Step 6.4: Run tests + commit**

```bash
go test ./internal/timer/... -v
golangci-lint run
git add internal/timer/service.go internal/timer/service_test.go
git commit -m "feat: implement Timer domain service"
```

Expected: all 6 tests PASS.

---

## Task 7: HTTP timer handlers + route wiring

**Files:**
- Modify: `internal/server/server.go` (Deps gains `Timer *timer.Service`)
- Modify: `internal/server/routes.go` (5 new routes)
- Create: `internal/server/timer_handler.go`

- [ ] **Step 7.1: Extend `Deps` in `internal/server/server.go`**

Find the `Deps` struct and add `Timer *timer.Service`. The full struct becomes:

```go
type Deps struct {
	Todos *todo.Service
	Tags  todo.TagRepo
	Bus   *eventbus.Bus
	Timer *timer.Service
}
```

Add the import in the same file:

```go
"github.com/spk/spk-cockpit/internal/timer"
```

- [ ] **Step 7.2: Create `internal/server/timer_handler.go`**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

func handleTimerStart(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.StartTimerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if req.TodoID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "todoId is required")
			return
		}
		s, err := d.Timer.Start(r.Context(), req.TodoID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "timer.start_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func handleTimerStop(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _, err := d.Timer.Stop(r.Context())
		if errors.Is(err, timer.ErrNoActiveSession) {
			writeError(w, http.StatusConflict, "timer.no_active", "no active session")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.stop_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func handleTimerActive(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		active, err := d.Timer.Active(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.active_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, active)
	}
}

func handleTodoTime(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var since int64
		if v := r.URL.Query().Get("since"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				since = n
			}
		}
		total, err := d.Timer.TotalForTodo(r.Context(), id, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.total_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, total)
	}
}

func handleTodoTimerSessions(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		sessions, err := d.Timer.ListSessions(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.list_failed", err.Error())
			return
		}
		if sessions == nil {
			sessions = []api.TimerSession{}
		}
		writeJSON(w, http.StatusOK, sessions)
	}
}
```

- [ ] **Step 7.3: Register routes in `internal/server/routes.go`**

Inside `registerRoutes`, after the existing `mux.HandleFunc("GET /api/tags", ...)` line and before the SPA fallback, add:

```go
	mux.HandleFunc("POST /api/timer/start", handleTimerStart(d))
	mux.HandleFunc("POST /api/timer/stop", handleTimerStop(d))
	mux.HandleFunc("GET /api/timer/active", handleTimerActive(d))
	mux.HandleFunc("GET /api/todos/{id}/time", handleTodoTime(d))
	mux.HandleFunc("GET /api/todos/{id}/sessions", handleTodoTimerSessions(d))
```

- [ ] **Step 7.4: Update server integration test**

In `internal/server/server_test.go`, the helper `newTestServer` constructs the Deps. Add the timer service:

Find the block:

```go
srv.Deps().Todos = todo.NewService(tr, gr, er, clock.NewFake(time.Unix(1700000000, 0)), bus)
srv.Deps().Tags = gr
srv.Deps().Bus = bus
```

After it, before `go func()`, add:

```go
timerRepo := timerfake.NewTimer()
srv.Deps().Timer = timer.NewService(timerRepo, clock.NewFake(time.Unix(1700000000, 0)), bus)
```

Add to the imports:

```go
"github.com/spk/spk-cockpit/internal/timer"
timerfake "github.com/spk/spk-cockpit/internal/timer/fakerepo"
```

Add a basic timer-flow test at the end of the file:

```go
func TestServer_TimerStartActiveStop(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()
	c := udsClient(sock)

	// Create a todo first.
	body, _ := json.Marshal(api.CreateTodoRequest{Title: "T", Priority: api.PriorityNormal})
	resp, err := c.Post("http://unix/api/todos", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	var td api.Todo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&td))

	// Start.
	startBody, _ := json.Marshal(api.StartTimerRequest{TodoID: td.ID})
	resp2, err := c.Post("http://unix/api/timer/start", "application/json", bytes.NewReader(startBody))
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)

	// Active should report the same todo.
	resp3, err := c.Get("http://unix/api/timer/active")
	require.NoError(t, err)
	defer resp3.Body.Close()
	var active *api.TimerSession
	require.NoError(t, json.NewDecoder(resp3.Body).Decode(&active))
	require.NotNil(t, active)
	require.Equal(t, td.ID, active.TodoID)

	// Stop.
	resp4, err := c.Post("http://unix/api/timer/stop", "application/json", nil)
	require.NoError(t, err)
	defer resp4.Body.Close()
	require.Equal(t, 200, resp4.StatusCode)

	// Active should now be nil (JSON null).
	resp5, err := c.Get("http://unix/api/timer/active")
	require.NoError(t, err)
	defer resp5.Body.Close()
	var afterStop *api.TimerSession
	require.NoError(t, json.NewDecoder(resp5.Body).Decode(&afterStop))
	require.Nil(t, afterStop)
}
```

- [ ] **Step 7.5: Run tests + commit**

```bash
go test ./internal/server/... -v
go test ./internal/...
golangci-lint run
git add internal/server/
git commit -m "feat: wire Timer service into UDS server with REST handlers"
```

---

## Task 8: Wire Timer service into the daemon

**Files:**
- Modify: `internal/cli/start.go`

The daemon currently constructs `todoSvc` and wires it; do the same for the timer service.

- [ ] **Step 8.1: Add timer construction to `runStart`**

In `internal/cli/start.go`, after the line `todoSvc := todo.NewService(todoRepo, tagRepo, eventRepo, clock.Real(), bus)`, add:

```go
	timerRepo := store.NewTimerRepo(st.DB)
	timerSvc := timer.NewService(timerRepo, clock.Real(), bus)
```

After `srv.Deps().Bus = bus`, add:

```go
	srv.Deps().Timer = timerSvc
```

Add the import:

```go
"github.com/spk/spk-cockpit/internal/timer"
```

- [ ] **Step 8.2: Build verification**

```bash
make web-build
go build -tags "webkit2_41 production" -o /tmp/cockpit-build-check ./cmd/cockpit
ls -lh /tmp/cockpit-build-check && rm /tmp/cockpit-build-check
```

- [ ] **Step 8.3: Smoke-test end-to-end**

```bash
export SPK_COCKPIT_DATA_DIR=/tmp/spk-p2-data
export SPK_COCKPIT_STATE_DIR=/tmp/spk-p2-state
export SPK_COCKPIT_CONFIG_DIR=/tmp/spk-p2-config
rm -rf "$SPK_COCKPIT_DATA_DIR" "$SPK_COCKPIT_STATE_DIR" "$SPK_COCKPIT_CONFIG_DIR"

./build/bin/spk-cockpit start &
DAEMON_PID=$!
sleep 2

# Create a todo, start timer, query active, stop timer, query total.
SOCK="$SPK_COCKPIT_STATE_DIR/cockpit.sock"
TODO_ID=$(curl -sS --unix-socket "$SOCK" -X POST http://unix/api/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Test timer","priority":1}' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "TODO_ID=$TODO_ID"

curl -sS --unix-socket "$SOCK" -X POST http://unix/api/timer/start \
  -H "Content-Type: application/json" \
  -d "{\"todoId\":\"$TODO_ID\"}"
echo

sleep 2

curl -sS --unix-socket "$SOCK" http://unix/api/timer/active
echo

curl -sS --unix-socket "$SOCK" -X POST http://unix/api/timer/stop
echo

curl -sS --unix-socket "$SOCK" "http://unix/api/todos/$TODO_ID/time"
echo

kill $DAEMON_PID 2>/dev/null; wait $DAEMON_PID 2>/dev/null

unset SPK_COCKPIT_DATA_DIR SPK_COCKPIT_STATE_DIR SPK_COCKPIT_CONFIG_DIR
```

Expected: `start` returns a session JSON; `active` returns same session; `stop` returns the closed session; `time` returns `{"totalSec": 2 (±1), ...}`.

- [ ] **Step 8.4: Commit**

```bash
git add internal/cli/start.go
git commit -m "feat: wire Timer service into the daemon"
```

---

## Task 9: CLI `cockpit timer` subcommand

**Files:**
- Create: `internal/cli/timer.go`
- Modify: `internal/cli/client.go` (add timer methods)

- [ ] **Step 9.1: Add timer methods to `Client` in `internal/cli/client.go`**

Append to the file (before the trailing `var ErrDaemonNotRunning`):

```go
// StartTimer starts a timer on todoID.
func (c *Client) StartTimer(ctx context.Context, todoID string) (api.TimerSession, error) {
	var out api.TimerSession
	if err := c.postJSON(ctx, "/api/timer/start", api.StartTimerRequest{TodoID: todoID}, &out); err != nil {
		return api.TimerSession{}, err
	}
	return out, nil
}

// StopTimer stops the active timer.
func (c *Client) StopTimer(ctx context.Context) (api.TimerSession, error) {
	resp, err := c.do(ctx, "POST", "/api/timer/stop", nil)
	if err != nil {
		return api.TimerSession{}, err
	}
	defer resp.Body.Close()
	var out api.TimerSession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.TimerSession{}, err
	}
	return out, nil
}

// ActiveTimer returns the active session, or nil if none.
func (c *Client) ActiveTimer(ctx context.Context) (*api.TimerSession, error) {
	resp, err := c.do(ctx, "GET", "/api/timer/active", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out *api.TimerSession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 9.2: Create `internal/cli/timer.go`**

```go
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var timerCmd = &cobra.Command{
	Use:   "timer",
	Short: "Time-tracking on todos",
}

var timerStartCmd = &cobra.Command{
	Use:   "start <id-suffix>",
	Short: "Start a timer on a todo",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveID(c, args[0])
		if err != nil {
			return err
		}
		s, err := c.StartTimer(context.Background(), id)
		if err != nil {
			return err
		}
		fmt.Printf("started %s on %s at %s\n", shortID(s.TodoID), s.TodoID, time.Unix(s.StartedAt, 0).Format(time.RFC3339))
		return nil
	},
}

var timerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active timer",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		s, err := c.StopTimer(context.Background())
		if err != nil {
			return err
		}
		dur := time.Duration(0)
		if s.EndedAt != nil {
			dur = time.Duration(*s.EndedAt-s.StartedAt) * time.Second
		}
		fmt.Printf("stopped %s after %s\n", shortID(s.TodoID), dur)
		return nil
	},
}

var timerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active timer (if any)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		s, err := c.ActiveTimer(context.Background())
		if err != nil {
			return err
		}
		if s == nil {
			fmt.Println("(no active timer)")
			return nil
		}
		started := time.Unix(s.StartedAt, 0)
		dur := time.Since(started).Round(time.Second)
		fmt.Printf("active: %s on %s (running for %s)\n", shortID(s.TodoID), s.TodoID, dur)
		return nil
	},
}

func init() {
	timerCmd.AddCommand(timerStartCmd, timerStopCmd, timerStatusCmd)
	rootCmd.AddCommand(timerCmd)
}

func shortID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}
```

- [ ] **Step 9.3: Smoke-test**

Same smoke-test pattern as Task 8.3, but using CLI:

```bash
./build/bin/spk-cockpit start &
sleep 2
TODO_ID=$(./build/bin/spk-cockpit todo add "T" -p normal | awk '{print $2}' | tr -d ':')
./build/bin/spk-cockpit timer start ${TODO_ID: -6}
sleep 2
./build/bin/spk-cockpit timer status
./build/bin/spk-cockpit timer stop
./build/bin/spk-cockpit timer status
./build/bin/spk-cockpit stop
```

Expected output sequence:
- `started ...`
- `active: ... (running for 2s)`
- `stopped ... after 2s`
- `(no active timer)`

(The exact `started`/`stopped` formats use `shortID` for the leading suffix.)

- [ ] **Step 9.4: Build + commit**

```bash
make build
golangci-lint run
git add internal/cli/timer.go internal/cli/client.go
git commit -m "feat: add cockpit timer start/stop/status subcommands"
```

---

## Task 10: Tray tooltip subscriber

**Files:**
- Create: `internal/tray/tooltip.go`
- Modify: `internal/cli/start.go`

The tooltip subscriber is a goroutine that listens to the bus and updates the tray tooltip whenever a timer starts or stops.

- [ ] **Step 10.1: Create `internal/tray/tooltip.go`**

```go
package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

// Subscriber polls the event bus and updates the tray tooltip on timer state changes.
type Subscriber struct {
	bus     EventSource
	tray    Backend
	current activeTimer
}

// EventSource is the slice of the bus that Subscriber needs.
type EventSource interface {
	Subscribe(buf int) chan api.Event
	Unsubscribe(ch chan api.Event)
}

type activeTimer struct {
	todoID    string
	startedAt int64
}

// NewSubscriber wires the bus and tray.
func NewSubscriber(bus EventSource, t Backend) *Subscriber {
	return &Subscriber{bus: bus, tray: t}
}

// Run subscribes and updates the tooltip until ctx is done.
// A 30s ticker keeps the elapsed time fresh while a timer is running.
func (s *Subscriber) Run(ctx context.Context) {
	ch := s.bus.Subscribe(32)
	defer s.bus.Unsubscribe(ch)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			s.handleEvent(e)
		case <-tick.C:
			s.refresh()
		}
	}
}

func (s *Subscriber) handleEvent(e api.Event) {
	switch e.Type {
	case api.EventTimerStarted:
		// Data was JSON-encoded by the bus producer; in-process it's the typed struct.
		if d, ok := e.Data.(api.TimerStartedData); ok {
			s.current = activeTimer{todoID: d.TodoID, startedAt: d.StartedAt}
			s.refresh()
		}
	case api.EventTimerStopped:
		s.current = activeTimer{}
		s.tray.SetTooltip("spk-cockpit")
	}
}

func (s *Subscriber) refresh() {
	if s.current.todoID == "" {
		s.tray.SetTooltip("spk-cockpit")
		return
	}
	elapsed := time.Since(time.Unix(s.current.startedAt, 0)).Round(time.Second)
	s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • %s on %s", elapsed, shortTodoID(s.current.todoID)))
}

func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}
```

- [ ] **Step 10.2: Wire subscriber into `runStart`**

In `internal/cli/start.go`, after the existing `go func()` that runs the tray (the line `t := tray.New(...)`), append a tooltip subscriber goroutine. Find the section that looks like:

```go
go func() {
    t := tray.New(
        func() { if winApp != nil { winApp.Show() } },
        func() { cancel() },
    )
    t.Run(nil, nil)
}()
```

Refactor to capture the tray instance so the subscriber can call `SetTooltip`:

```go
trayBackend := tray.New(
    func() { if winApp != nil { winApp.Show() } },
    func() { cancel() },
)
go func() {
    trayBackend.Run(nil, nil)
}()
go tray.NewSubscriber(bus, trayBackend).Run(ctx)
```

(`bus` here is the `*eventbus.Bus` already constructed in `runStart`. Its `Subscribe`/`Unsubscribe` methods satisfy `tray.EventSource` structurally.)

- [ ] **Step 10.3: Build + smoke-test**

```bash
make build
```

Expected: builds cleanly. Tooltip will only be visible when running the binary on a real desktop — a unit test for the goroutine logic could be added later but the lint+build pass is enough for Phase 2.

- [ ] **Step 10.4: Commit**

```bash
golangci-lint run
git add internal/tray/tooltip.go internal/cli/start.go
git commit -m "feat: tray tooltip reflects active timer"
```

---

## Task 11: Web — types, API, and store extensions

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/store.ts`

- [ ] **Step 11.1: Append timer types to `web/src/lib/types.ts`**

Add at the end of the file:

```ts
export interface TimerSession {
  id: number;
  todoId: string;
  startedAt: number;
  endedAt?: number;
  source: string;
}

export interface TodoTimeTotal {
  todoId: string;
  sinceUnix: number;
  totalSec: number;
  sessionCount: number;
  hasActive: boolean;
}

export interface StartTimerRequest {
  todoId: string;
}
```

- [ ] **Step 11.2: Add timer methods to `web/src/lib/api.ts`**

Inside the `export const api = { ... }` object, add the following methods (preserving existing ones):

```ts
  startTimer: (todoId: string) =>
    request<TimerSession>("/api/timer/start", {
      method: "POST",
      body: JSON.stringify({ todoId }),
    }),
  stopTimer: () =>
    request<TimerSession>("/api/timer/stop", { method: "POST" }),
  activeTimer: () =>
    request<TimerSession | null>("/api/timer/active"),
  todoTime: (id: string, sinceUnix = 0) =>
    request<TodoTimeTotal>(`/api/todos/${id}/time?since=${sinceUnix}`),
```

Update the `import` line at the top:

```ts
import type { Todo, Tag, CreateTodoRequest, UpdateTodoRequest, TimerSession, TodoTimeTotal } from "./types";
```

- [ ] **Step 11.3: Extend Zustand store in `web/src/lib/store.ts`**

Add a timer slice. Update the `TodoState` interface and `useTodoStore` factory:

```ts
import { create } from "zustand";
import { api } from "./api";
import type { Todo, ApiEvent, TimerSession } from "./types";

interface TodoState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;

  activeTimer: TimerSession | null;

  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  applyEvent: (e: ApiEvent) => void;
  loadActiveTimer: () => Promise<void>;
}

export const useTodoStore = create<TodoState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  activeTimer: null,

  async load() {
    set({ loading: true, error: null });
    try {
      const todos = await api.listTodos(get().includeDone);
      set({ todos, loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  setIncludeDone(v) {
    set({ includeDone: v });
    void get().load();
  },
  async loadActiveTimer() {
    try {
      const t = await api.activeTimer();
      set({ activeTimer: t });
    } catch {
      set({ activeTimer: null });
    }
  },
  applyEvent(e) {
    if (e.type === "todo.created") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: [todo, ...get().todos] });
    } else if (e.type === "todo.updated") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: get().todos.map((t) => (t.id === todo.id ? todo : t)) });
    } else if (e.type === "todo.deleted") {
      const { todoId } = e.data as { todoId: string };
      set({ todos: get().todos.filter((t) => t.id !== todoId) });
    } else if (e.type === "timer.started") {
      const d = e.data as { todoId: string; sessionId: number; startedAt: number };
      set({
        activeTimer: {
          id: d.sessionId,
          todoId: d.todoId,
          startedAt: d.startedAt,
          source: "manual",
        },
      });
    } else if (e.type === "timer.stopped") {
      set({ activeTimer: null });
    }
  },
}));
```

- [ ] **Step 11.4: Verify build**

```bash
cd /home/spk/IdeaProjects/spk-task-manager/web
pnpm build
pnpm lint
```

- [ ] **Step 11.5: Commit**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add web/src/lib/
git commit -m "feat: extend web types/api/store with timer support"
```

---

## Task 12: TimerBadge component + Start/Stop button on TodoRow

**Files:**
- Create: `web/src/components/TimerBadge.tsx`
- Modify: `web/src/components/TodoRow.tsx`

- [ ] **Step 12.1: Create `web/src/components/TimerBadge.tsx`**

```tsx
import { useEffect, useState } from "react";
import { Timer as TimerIcon } from "lucide-react";

export interface TimerBadgeProps {
  startedAt: number;
  label?: string;
}

function formatElapsed(sec: number): string {
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return `${m}:${String(s).padStart(2, "0")}`;
  const h = Math.floor(m / 60);
  const mm = m % 60;
  return `${h}:${String(mm).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function TimerBadge({ startedAt, label }: TimerBadgeProps) {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));

  useEffect(() => {
    const id = window.setInterval(() => {
      setNow(Math.floor(Date.now() / 1000));
    }, 1000);
    return () => window.clearInterval(id);
  }, []);

  const elapsed = Math.max(0, now - startedAt);
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-accent/20 text-accent text-xs font-mono">
      <TimerIcon size={12} />
      {formatElapsed(elapsed)}
      {label ? <span className="text-fgmute ml-1">{label}</span> : null}
    </span>
  );
}
```

- [ ] **Step 12.2: Update `web/src/components/TodoRow.tsx`**

Replace the existing file contents:

```tsx
import { Check, Trash2, Play, Square } from "lucide-react";
import type { Todo } from "../lib/types";
import { Priority } from "../lib/types";
import { TagPill } from "./TagPill";
import { TimerBadge } from "./TimerBadge";

const priorityClass: Record<number, string> = {
  [Priority.Urgent]: "text-urgent",
  [Priority.High]: "text-high",
  [Priority.Normal]: "text-normal",
  [Priority.Low]: "text-low",
};

const priorityGlyph: Record<number, string> = {
  [Priority.Urgent]: "🔥",
  [Priority.High]: "⚡",
  [Priority.Normal]: "•",
  [Priority.Low]: "▫",
};

export interface TodoRowProps {
  todo: Todo;
  activeTimerStartedAt: number | null; // not null = active on this todo
  onToggleDone: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
  onStartTimer: (todo: Todo) => void;
  onStopTimer: (todo: Todo) => void;
}

export function TodoRow({
  todo,
  activeTimerStartedAt,
  onToggleDone,
  onDelete,
  onStartTimer,
  onStopTimer,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;
  return (
    <div className="flex items-center gap-3 p-3 rounded hover:bg-bgsub group">
      <button
        onClick={() => onToggleDone(todo)}
        className={`w-5 h-5 rounded border ${isDone ? "bg-success border-success" : "border-fgmute"} flex items-center justify-center`}
        aria-label={isDone ? "Mark as open" : "Mark as done"}
      >
        {isDone && <Check size={14} className="text-bg" />}
      </button>
      <span className={priorityClass[todo.priority]}>{priorityGlyph[todo.priority]}</span>
      <span className={`flex-1 ${isDone ? "line-through text-fgmute" : ""}`}>
        {todo.title}
      </span>
      {hasTimer && <TimerBadge startedAt={activeTimerStartedAt!} />}
      <div className="flex gap-1">
        {todo.tags.map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      {hasTimer ? (
        <button
          onClick={() => onStopTimer(todo)}
          className="text-urgent hover:text-fg"
          aria-label="Stop timer"
        >
          <Square size={16} />
        </button>
      ) : (
        <button
          onClick={() => onStartTimer(todo)}
          className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-accent"
          aria-label="Start timer"
          disabled={isDone}
        >
          <Play size={16} />
        </button>
      )}
      <button
        onClick={() => onDelete(todo)}
        className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-urgent"
        aria-label="Delete"
      >
        <Trash2 size={16} />
      </button>
    </div>
  );
}
```

- [ ] **Step 12.3: Verify build**

```bash
cd web && pnpm build && pnpm lint
```

(Note: TodoList in Task 13 will be updated to pass the new props — until then `pnpm build` will fail because TodoList still passes the old prop signature. That's OK; commit Task 12 only after Task 13 lands. Skip the commit step for now — combine commits 12+13.)

---

## Task 13: TodoList — wire timer actions and active-timer prop

**Files:**
- Modify: `web/src/components/TodoList.tsx`
- Modify: `web/src/pages/Todos.tsx`

- [ ] **Step 13.1: Replace `web/src/components/TodoList.tsx`**

```tsx
import { useEffect } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TodoRow } from "./TodoRow";
import type { Todo } from "../lib/types";

export function TodoList() {
  const {
    todos,
    loading,
    error,
    load,
    includeDone,
    setIncludeDone,
    activeTimer,
    loadActiveTimer,
  } = useTodoStore();

  useEffect(() => {
    void load();
    void loadActiveTimer();
  }, [load, loadActiveTimer]);

  async function toggleDone(t: Todo) {
    const next = t.status === "done" ? "open" : "done";
    await api.updateTodo(t.id, { status: next });
  }

  async function remove(t: Todo) {
    if (!confirm(`Delete "${t.title}"?`)) return;
    await api.deleteTodo(t.id);
  }

  async function startTimer(t: Todo) {
    await api.startTimer(t.id);
  }

  async function stopTimer(_t: Todo) {
    await api.stopTimer();
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold">Todos</h2>
        <label className="flex items-center gap-2 text-fgmute text-sm">
          <input
            type="checkbox"
            checked={includeDone}
            onChange={(e) => setIncludeDone(e.target.checked)}
          />
          show done
        </label>
      </div>
      {loading && <div className="text-fgmute">loading…</div>}
      {error && <div className="text-urgent">error: {error}</div>}
      <div className="flex flex-col">
        {todos.map((t) => (
          <TodoRow
            key={t.id}
            todo={t}
            activeTimerStartedAt={
              activeTimer && activeTimer.todoId === t.id ? activeTimer.startedAt : null
            }
            onToggleDone={toggleDone}
            onDelete={remove}
            onStartTimer={startTimer}
            onStopTimer={stopTimer}
          />
        ))}
        {!loading && todos.length === 0 && (
          <div className="text-fgmute py-8 text-center">no todos yet</div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 13.2: Build + lint**

```bash
cd web && pnpm build && pnpm lint
```

Expected: clean.

- [ ] **Step 13.3: Commit Tasks 12 + 13 together**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add web/src/components/ web/src/pages/
git commit -m "feat: timer Start/Stop button on TodoRow with live elapsed badge"
```

---

## Task 14: Quick-add inline parser + tests

**Files:**
- Create: `web/src/lib/parser.ts`
- Create: `web/src/lib/parser.test.ts`

The parser accepts strings like:

```
Fix login bug !urgent #backend #review due:2026-05-01
Buy milk
Quick task !low #shopping
Review MR due:tomorrow
```

It extracts:
- A bare title (everything that's not a token).
- Priority tokens `!low`, `!normal`, `!high`, `!urgent`.
- Tags `#tag` (one per token, alphanumeric + dash + underscore).
- Due `due:YYYY-MM-DD` or `due:tomorrow` or `due:today` (resolved to local 18:00 unix seconds).

Unknown tokens are left in the title.

- [ ] **Step 14.1: Write `web/src/lib/parser.test.ts`**

```ts
import { describe, it, expect } from "vitest";
import { parseQuickAdd } from "./parser";
import { Priority } from "./types";

describe("parseQuickAdd", () => {
  it("returns plain title when there are no tokens", () => {
    const r = parseQuickAdd("Buy milk");
    expect(r.title).toBe("Buy milk");
    expect(r.priority).toBe(Priority.Normal);
    expect(r.tags).toEqual([]);
    expect(r.dueAt).toBeUndefined();
  });

  it("parses priority", () => {
    const r = parseQuickAdd("Fix bug !urgent");
    expect(r.title).toBe("Fix bug");
    expect(r.priority).toBe(Priority.Urgent);
  });

  it("parses multiple tags", () => {
    const r = parseQuickAdd("Review MR #backend #review");
    expect(r.title).toBe("Review MR");
    expect(r.tags).toEqual(["backend", "review"]);
  });

  it("parses due:YYYY-MM-DD as 18:00 local that day", () => {
    const r = parseQuickAdd("Ship release due:2026-05-01");
    expect(r.title).toBe("Ship release");
    expect(r.dueAt).toBeDefined();
    const d = new Date(r.dueAt! * 1000);
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(4); // May (0-indexed)
    expect(d.getDate()).toBe(1);
    expect(d.getHours()).toBe(18);
  });

  it("parses due:today/tomorrow", () => {
    const r1 = parseQuickAdd("X due:today");
    const r2 = parseQuickAdd("Y due:tomorrow");
    expect(r1.dueAt).toBeDefined();
    expect(r2.dueAt).toBeDefined();
    expect(r2.dueAt!).toBeGreaterThan(r1.dueAt!);
  });

  it("handles all tokens together", () => {
    const r = parseQuickAdd("Fix login bug !urgent #backend #review due:2026-05-01");
    expect(r.title).toBe("Fix login bug");
    expect(r.priority).toBe(Priority.Urgent);
    expect(r.tags).toEqual(["backend", "review"]);
    expect(r.dueAt).toBeDefined();
  });

  it("ignores unknown ! / # / due: tokens (passes them through as title)", () => {
    const r = parseQuickAdd("Try !nope #with-dash due:badformat");
    expect(r.title).toBe("Try !nope due:badformat");
    expect(r.tags).toEqual(["with-dash"]);
  });

  it("trims and collapses spaces in remaining title", () => {
    const r = parseQuickAdd("  Fix    !high   #x   bug  ");
    expect(r.title).toBe("Fix bug");
    expect(r.priority).toBe(Priority.High);
  });

  it("returns empty title if input is only tokens", () => {
    const r = parseQuickAdd("!high #only");
    expect(r.title).toBe("");
    expect(r.priority).toBe(Priority.High);
    expect(r.tags).toEqual(["only"]);
  });
});
```

- [ ] **Step 14.2: Run tests to confirm they fail**

```bash
cd web && pnpm test --run lib/parser.test.ts
```

Expected: FAIL — module missing.

- [ ] **Step 14.3: Write `web/src/lib/parser.ts`**

```ts
import { Priority } from "./types";
import type { Priority as P } from "./types";

export interface QuickAddResult {
  title: string;
  priority: P;
  tags: string[];
  dueAt?: number;
}

const TAG_RE = /^#([a-z0-9][a-z0-9_\-]*)$/i;
const PRIO_MAP: Record<string, P> = {
  "!low": Priority.Low,
  "!normal": Priority.Normal,
  "!high": Priority.High,
  "!urgent": Priority.Urgent,
};

export function parseQuickAdd(input: string): QuickAddResult {
  const tokens = input.split(/\s+/).filter(Boolean);
  let priority: P = Priority.Normal;
  const tags: string[] = [];
  let dueAt: number | undefined;
  const titleTokens: string[] = [];

  for (const tok of tokens) {
    if (PRIO_MAP[tok.toLowerCase()] !== undefined) {
      priority = PRIO_MAP[tok.toLowerCase()];
      continue;
    }
    const tagMatch = tok.match(TAG_RE);
    if (tagMatch) {
      tags.push(tagMatch[1]);
      continue;
    }
    if (tok.toLowerCase().startsWith("due:")) {
      const v = tok.slice(4);
      const ts = parseDueValue(v);
      if (ts !== null) {
        dueAt = ts;
        continue;
      }
      // unknown due value — fall through and keep token in title
    }
    titleTokens.push(tok);
  }

  return {
    title: titleTokens.join(" ").trim(),
    priority,
    tags,
    dueAt,
  };
}

function parseDueValue(v: string): number | null {
  const lower = v.toLowerCase();
  const today = atSixPM(new Date());

  if (lower === "today") {
    return Math.floor(today.getTime() / 1000);
  }
  if (lower === "tomorrow") {
    const t = new Date(today.getTime());
    t.setDate(t.getDate() + 1);
    return Math.floor(t.getTime() / 1000);
  }
  // YYYY-MM-DD
  const m = v.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (m) {
    const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 18, 0, 0, 0);
    if (!isNaN(d.getTime())) {
      return Math.floor(d.getTime() / 1000);
    }
  }
  return null;
}

function atSixPM(base: Date): Date {
  const d = new Date(base);
  d.setHours(18, 0, 0, 0);
  return d;
}
```

- [ ] **Step 14.4: Run tests to confirm pass**

```bash
cd web && pnpm test --run lib/parser.test.ts
```

Expected: 9/9 tests PASS.

- [ ] **Step 14.5: Commit**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add web/src/lib/parser.ts web/src/lib/parser.test.ts
git commit -m "feat: quick-add inline parser (priority/tags/due)"
```

---

## Task 15: Wire parser into AddTodoForm

**Files:**
- Modify: `web/src/components/AddTodoForm.tsx`

- [ ] **Step 15.1: Replace `web/src/components/AddTodoForm.tsx`**

```tsx
import { useState } from "react";
import { Priority } from "../lib/types";
import { api } from "../lib/api";
import { parseQuickAdd } from "../lib/parser";

export function AddTodoForm() {
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);

  const preview = input ? parseQuickAdd(input) : null;
  const isValid = preview !== null && preview.title.length > 0;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!isValid) return;
    setBusy(true);
    try {
      await api.createTodo({
        title: preview!.title,
        priority: preview!.priority ?? Priority.Normal,
        tags: preview!.tags.length > 0 ? preview!.tags : undefined,
        dueAt: preview!.dueAt,
      });
      setInput("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-1">
      <input
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder='+ Add todo (e.g. "Fix bug !urgent #backend due:tomorrow")'
        className="flex-1 bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        disabled={busy}
      />
      {preview && input.length > 0 && (
        <div className="text-xs text-fgmute pl-2 flex gap-3 flex-wrap">
          <span>title: <span className="text-fg">{preview.title || "(empty)"}</span></span>
          {preview.priority !== Priority.Normal && (
            <span>!{labelFor(preview.priority)}</span>
          )}
          {preview.tags.length > 0 && <span>tags: {preview.tags.map((t) => `#${t}`).join(" ")}</span>}
          {preview.dueAt && <span>due: {new Date(preview.dueAt * 1000).toLocaleString()}</span>}
        </div>
      )}
    </form>
  );
}

function labelFor(p: number): string {
  switch (p) {
    case Priority.Low:
      return "low";
    case Priority.High:
      return "high";
    case Priority.Urgent:
      return "urgent";
    default:
      return "normal";
  }
}
```

- [ ] **Step 15.2: Build + lint + commit**

```bash
cd web && pnpm build && pnpm lint
cd /home/spk/IdeaProjects/spk-task-manager
git add web/src/components/AddTodoForm.tsx
git commit -m "feat: AddTodoForm uses quick-add parser with live preview"
```

---

## Task 16: Popover route + page

**Files:**
- Modify: `web/package.json` (add react-router-dom)
- Modify: `web/src/App.tsx`
- Create: `web/src/pages/Popover.tsx`

- [ ] **Step 16.1: Install router**

```bash
cd /home/spk/IdeaProjects/spk-task-manager/web
pnpm add react-router-dom
```

- [ ] **Step 16.2: Replace `web/src/App.tsx`**

```tsx
import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import { Todos } from "./pages/Todos";
import { Popover } from "./pages/Popover";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/popover" element={<Popover />} />
        <Route path="*" element={<MainShell />} />
      </Routes>
    </BrowserRouter>
  );
}

function MainShell() {
  const loc = useLocation();
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1 text-fgmute">
          <Link to="/" className={loc.pathname === "/" ? "text-fg" : ""}>
            Todos
          </Link>
          <Link to="/popover" className="text-fgmute">
            Compact view
          </Link>
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Todos />
      </main>
    </div>
  );
}
```

- [ ] **Step 16.3: Create `web/src/pages/Popover.tsx`**

```tsx
import { useEffect } from "react";
import { Link } from "react-router-dom";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TimerBadge } from "../components/TimerBadge";
import { AddTodoForm } from "../components/AddTodoForm";
import type { Todo } from "../lib/types";
import { EventStream } from "../lib/events";

const stream = new EventStream();

export function Popover() {
  const {
    todos,
    activeTimer,
    load,
    loadActiveTimer,
    applyEvent,
  } = useTodoStore();

  useEffect(() => {
    void load();
    void loadActiveTimer();
    stream.start();
    const off = stream.on(applyEvent);
    return () => {
      off();
      stream.stop();
    };
  }, [load, loadActiveTimer, applyEvent]);

  const open = todos.filter((t) => t.status !== "done" && t.status !== "cancelled");
  const top = open.slice(0, 5);
  const activeTodo = activeTimer ? todos.find((t) => t.id === activeTimer.todoId) : null;

  async function startOn(t: Todo) {
    await api.startTimer(t.id);
  }
  async function stopActive() {
    await api.stopTimer();
  }

  return (
    <div className="bg-bg text-fg p-3 flex flex-col gap-3 max-w-sm">
      {activeTimer && (
        <div className="flex items-center justify-between bg-bgsub rounded p-2">
          <div className="flex flex-col">
            <span className="text-xs text-fgmute">Active timer</span>
            <span className="text-sm">{activeTodo ? activeTodo.title : "(unknown todo)"}</span>
          </div>
          <div className="flex items-center gap-2">
            <TimerBadge startedAt={activeTimer.startedAt} />
            <button onClick={stopActive} className="text-urgent hover:text-fg text-sm">
              stop
            </button>
          </div>
        </div>
      )}

      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between">
          <span className="text-fgmute text-xs uppercase">Today</span>
          <span className="text-fgmute text-xs">
            {open.length} open
          </span>
        </div>
        {top.length === 0 && <div className="text-fgmute text-sm py-2">all clear</div>}
        {top.map((t) => (
          <button
            key={t.id}
            onClick={() => startOn(t)}
            className="flex items-center justify-between text-left p-2 rounded hover:bg-bgsub"
            disabled={!!activeTimer && activeTimer.todoId === t.id}
          >
            <span className="truncate">{t.title}</span>
            {!!activeTimer && activeTimer.todoId === t.id ? (
              <span className="text-accent text-xs">running</span>
            ) : (
              <span className="text-fgmute text-xs">▶ start</span>
            )}
          </button>
        ))}
      </div>

      <AddTodoForm />

      <Link to="/" className="text-fgmute text-xs underline self-start">
        Open full window →
      </Link>
    </div>
  );
}
```

- [ ] **Step 16.4: Verify build and routing works**

```bash
cd web
pnpm build
pnpm lint
```

You can also smoke-test in dev mode (run separately, not part of the commit flow):

```bash
# In one terminal:
./build/bin/spk-cockpit start
# In another:
xdg-open http://localhost:5173/popover  # if pnpm dev is running
```

But for the plan, verifying `pnpm build` succeeds is sufficient.

- [ ] **Step 16.5: Rebuild Go binary so embedded UI includes popover**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
make build
```

- [ ] **Step 16.6: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/src/
git commit -m "feat: add /popover route with compact today/timer layout"
```

---

## Task 17: Visual smoke test (Wails window)

**Files:** none

The implementer cannot interactively click; instead, take screenshots via OS tools.

- [ ] **Step 17.1: Launch and screenshot main view**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
rm -rf /tmp/spk-p2-data /tmp/spk-p2-state /tmp/spk-p2-config
SPK_COCKPIT_DATA_DIR=/tmp/spk-p2-data \
SPK_COCKPIT_STATE_DIR=/tmp/spk-p2-state \
SPK_COCKPIT_CONFIG_DIR=/tmp/spk-p2-config \
DISPLAY=:0 \
./build/bin/spk-cockpit start > /tmp/cockpit-p2.log 2>&1 &
DAEMON_PID=$!

sleep 4
if ! kill -0 $DAEMON_PID 2>/dev/null; then
    echo "BLOCKED: daemon exited; log follows:"
    cat /tmp/cockpit-p2.log
    exit 1
fi

# Add a few todos through the API; start a timer.
SOCK=/tmp/spk-p2-state/cockpit.sock
TODO_ID=$(curl -sS --unix-socket "$SOCK" -X POST http://unix/api/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Phase 2 timer demo","priority":2}' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
curl -sS --unix-socket "$SOCK" -X POST http://unix/api/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Quick add !urgent #demo due:tomorrow","priority":3}' > /dev/null
curl -sS --unix-socket "$SOCK" -X POST http://unix/api/timer/start \
  -H "Content-Type: application/json" \
  -d "{\"todoId\":\"$TODO_ID\"}" > /dev/null

sleep 2
DISPLAY=:0 import -window root /tmp/cockpit-p2-main.png
ls -lh /tmp/cockpit-p2-main.png

kill $DAEMON_PID 2>/dev/null
wait $DAEMON_PID 2>/dev/null
```

Then read `/tmp/cockpit-p2-main.png` with the Read tool and confirm visually:
- The main window shows the two todos.
- The "Phase 2 timer demo" todo has the TimerBadge rendered next to it (mm:ss elapsed).
- A red Stop button is visible on the active timer row.

If the screenshot does not show the expected state, REPORT BLOCKED with the screenshot for the controller.

- [ ] **Step 17.2: Document the visual-smoke results in the report**

(No commit — this is a verification step only.)

---

## Task 18: README update

**Files:**
- Modify: `README.md`

- [ ] **Step 18.1: Replace Phase 1 status section**

In `README.md`, find the section starting with `## Phase 1 status` and replace with:

```markdown
## Status

### Phase 1 ✅
- Todo CRUD (priority, status, due, tags, audit history)
- Tray icon with menu (Open window / Quit)
- Wails main window
- CLI: `cockpit start | stop | todo add/list/done/rm`
- SQLite storage with migrations
- HTTP/UDS server with SSE for realtime UI updates

### Phase 2 ✅
- Time-tracking on todos: `/api/timer/start`, `/api/timer/stop`, `/api/timer/active`, `/api/todos/{id}/time`
- One-active-timer-globally invariant
- Tray tooltip reflects active timer
- CLI: `cockpit timer start <id> | stop | status`
- TimerBadge component (live elapsed counter)
- Quick-add inline syntax (`!priority #tag due:tomorrow`) with live preview
- Compact `/popover` route showing today / active timer / quick add

Phases 3–4 (meetings + CalDAV + notifications, standup helper + integrations + autostart + releases) are planned separately.
```

- [ ] **Step 18.2: Update CLI examples**

Find the existing CLI examples block and append:

```bash
cockpit timer start abc123             # timer on the todo whose id ends with abc123
cockpit timer status                   # see what's running
cockpit timer stop                     # stop the active timer
```

- [ ] **Step 18.3: Commit + tag**

```bash
git add README.md
git commit -m "docs: update README for phase 2 completion"
git tag v0.2.0-phase2
```

---

## Phase 2 Done — Definition of Done

- [ ] Migration 0002 applied; `timer_sessions` table exists.
- [ ] `TimerRepo` (SQLite + fake) passes conformance tests.
- [ ] `timer.Service` enforces one-active-globally and emits TimerStarted/TimerStopped events.
- [ ] HTTP endpoints `/api/timer/{start,stop,active}` and `/api/todos/{id}/{time,sessions}` work.
- [ ] CLI `cockpit timer start|stop|status` works end-to-end against the daemon.
- [ ] Tray tooltip changes when a timer starts/stops (and refreshes elapsed every 30s while running).
- [ ] Web UI: TodoRow has Start/Stop button; live TimerBadge counts seconds.
- [ ] Quick-add parser: 9 unit tests pass; AddTodoForm shows live preview.
- [ ] `/popover` route renders compact layout with active timer banner + top todos + quick-add.
- [ ] `make build` succeeds; `go test ./internal/...` PASS; `golangci-lint run` 0 issues; `pnpm test --run` PASS; `pnpm lint` clean.
- [ ] Visual screenshot smoke confirms the timer badge renders in the Wails window.
- [ ] README updated; `v0.2.0-phase2` tag created.
