# spk-cockpit Phase 3: Meetings + CalDAV (Yandex) + Notifications

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Read-only sync of upcoming meetings from Yandex Calendar via CalDAV, system notifications fired N minutes before each meeting (default 5, per-meeting override), markdown notes attached to meetings, encrypted storage of CalDAV credentials, and a Calendar/Settings UI to drive it all.

**Architecture:** Three new domains (`meeting`, `note`, `secret`) following the established pattern (repo interfaces + SQLite + in-memory fake + conformance + service-emits-events). CalDAV sync is a background worker using `emersion/go-webdav` (transport) + `emersion/go-ical` (parsing). Notifications use DBus (`godbus/dbus`). NotificationScheduler is a `time.Ticker`-driven goroutine that scans `meetings WHERE start_at - notify_min*60 ≤ now AND notified_at IS NULL` and fires DBus + sets `notified_at`. Master encryption key sourced from OS keyring (`zalando/go-keyring`) with an `SPK_COCKPIT_MASTER_KEY` env-var override for tests.

**Tech Stack:** Go 1.22+ (existing), `emersion/go-webdav` (NEW — CalDAV transport), `emersion/go-ical` (NEW — iCal parsing), `godbus/dbus/v5` (NEW — DBus IPC), `zalando/go-keyring` (NEW — OS keyring abstraction), React 19 + Vite 8 + Tailwind 4 (existing), `react-router-dom` 7 (existing).

**Reference plans:** Phase 1 `docs/superpowers/plans/2026-04-27-spk-cockpit-phase1-foundation.md`, Phase 2 `docs/superpowers/plans/2026-04-27-spk-cockpit-phase2-timer-popover.md`. Phase 3 builds directly on Phase 2 (commit `955894f`, tag `v0.2.0-phase2`).

**Phase 1+2 conventions still apply:**

- Build tags `webkit2_41 production` for Wails build.
- Doc comments on every exported entity (revive). `//nolint:revive` for `pkg.PkgType` stutter is acceptable with explanation. `//nolint:gosec` for `LIMIT %d` with controlled int.
- No `Co-Authored-By` in commit messages. **Always create new commits, never amend.**
- Don't create binaries in project root (root `.gitignore` ignores `/cockpit` and `/spk-cockpit` but it's still a habit-error to avoid).
- All mutations go through a domain service that emits an audit event (where applicable) AND publishes a domain event to the bus.
- All repos have a SQLite implementation AND an in-memory fake; conformance tests run identical assertions against both.

---

## Out of Scope for Phase 3

- Two-way CalDAV sync (writing meetings to Yandex). Phase 3 is **read-only** from CalDAV; manual meetings stay local.
- iCal recurrence rule (`RRULE`) expansion beyond what `go-ical` provides out of the box. Yandex events that recur expand via `go-ical`'s iterator; if it doesn't expand correctly, those events show as the master event only (acceptable for v1).
- Password-prompt fallback when OS keyring is unavailable. v1: keyring is required for production, env-var for tests. Document the limitation.
- Daily / week-grid Calendar UI with drag-drop. v1 Calendar shows a chronological list grouped by **Today / Tomorrow / Later** — sufficient and YAGNI.
- Standalone Notes browse page with full-text search. Notes are accessed through their parent meeting. Standalone notes browse is Phase 4 polish.
- Tray badge "N meetings today". Tooltip is updated; badge icon is Phase 4.

---

## File Structure (new + modified)

```
spk-task-manager/
├── internal/
│   ├── store/
│   │   ├── migrations/0003_meetings_notes_secrets.sql      # NEW
│   │   ├── meeting_repo.go                                 # NEW
│   │   ├── note_repo.go                                    # NEW
│   │   ├── secret_repo.go                                  # NEW
│   │   ├── sync_state_repo.go                              # NEW
│   │   ├── conformance_test.go                             # MODIFIED (3 new conformance tests)
│   │   └── migrate_test.go                                 # MODIFIED (version 3, new tables)
│   ├── api/
│   │   ├── dto.go                                          # MODIFIED (Meeting, Note, Secret DTOs)
│   │   └── events.go                                       # MODIFIED (meeting/note event constants)
│   ├── meeting/                                            # NEW
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── service_test.go
│   │   └── fakerepo/meeting_repo.go
│   ├── note/                                               # NEW
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── service_test.go
│   │   └── fakerepo/note_repo.go
│   ├── secret/                                             # NEW
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── keyring.go                                      # KeyResolver interface + Env+Keyring impls
│   │   ├── service_test.go
│   │   └── fakerepo/secret_repo.go
│   ├── sync/
│   │   └── caldav/                                         # NEW
│   │       ├── client.go                                   # webdav+ical wrapper (interface + real impl)
│   │       ├── syncer.go                                   # background worker
│   │       └── syncer_test.go                              # uses httptest.Server with iCal fixtures
│   ├── notify/                                             # NEW
│   │   ├── notify.go                                       # Notifier interface
│   │   ├── notify_dbus.go                                  # DBus impl (build-tagged)
│   │   ├── notify_noop.go                                  # no-op fallback
│   │   └── scheduler.go                                    # MeetingScheduler worker
│   ├── server/
│   │   ├── server.go                                       # MODIFIED (Deps gains Meetings, Notes, Secrets, Sync)
│   │   ├── routes.go                                       # MODIFIED (meeting/note/secret/sync routes)
│   │   ├── meeting_handler.go                              # NEW
│   │   ├── note_handler.go                                 # NEW
│   │   ├── secret_handler.go                               # NEW
│   │   └── server_test.go                                  # MODIFIED (test fixtures + new handler tests)
│   ├── cli/
│   │   ├── start.go                                        # MODIFIED (wire new services + workers)
│   │   ├── client.go                                       # MODIFIED (meeting/note/secret/sync methods)
│   │   ├── meeting.go                                      # NEW (cobra subcommand)
│   │   └── secret.go                                       # NEW (cobra subcommand)
│   ├── tray/
│   │   └── tooltip.go                                      # MODIFIED (handle MeetingNotificationFired event)
│   └── testdata/
│       └── caldav/                                         # NEW (synthetic iCal fixtures)
│           ├── single_event.ics
│           ├── recurring.ics
│           └── multistatus_response.xml
└── web/
    ├── package.json                                        # (no new deps)
    └── src/
        ├── App.tsx                                         # MODIFIED (add /calendar, /settings routes)
        ├── lib/
        │   ├── types.ts                                    # MODIFIED (Meeting, Note, Secret types)
        │   ├── api.ts                                      # MODIFIED (meeting/note/secret/sync methods)
        │   └── store.ts                                    # MODIFIED (meetings, syncState slices)
        ├── components/
        │   ├── MeetingCard.tsx                             # NEW
        │   ├── MeetingDetail.tsx                           # NEW (notes editor)
        │   └── SyncStatusBadge.tsx                         # NEW
        └── pages/
            ├── Calendar.tsx                                # NEW
            └── Settings.tsx                                # NEW
```

After Phase 3, `make build` still produces a single Linux binary `build/bin/spk-cockpit` with `webkit2_41 production` tags. New runtime deps: an OS keyring service (libsecret on Linux) for production; tests use the env-var override.

---

## Task 1: Migration 0003 — meetings, notes, secrets, sync_state

**Files:**
- Create: `internal/store/migrations/0003_meetings_notes_secrets.sql`
- Modify: `internal/store/migrate_test.go`

- [ ] **Step 1.1: Write the migration**

```sql
CREATE TABLE meetings (
  id            TEXT PRIMARY KEY,
  source        TEXT NOT NULL,           -- 'manual' | 'caldav'
  external_uid  TEXT,
  external_etag TEXT,
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  location      TEXT NOT NULL DEFAULT '',
  start_at      INTEGER NOT NULL,
  end_at        INTEGER NOT NULL,
  notify_min    INTEGER,
  notified_at   INTEGER,
  cancelled     INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  deleted_at    INTEGER
);
CREATE UNIQUE INDEX uq_meetings_external ON meetings(source, external_uid) WHERE external_uid IS NOT NULL;
CREATE INDEX idx_meetings_start ON meetings(start_at);

CREATE TABLE notes (
  id          TEXT PRIMARY KEY,
  meeting_id  TEXT,
  todo_id     TEXT,
  body        TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  deleted_at  INTEGER
);
CREATE INDEX idx_notes_meeting ON notes(meeting_id) WHERE meeting_id IS NOT NULL;
CREATE INDEX idx_notes_todo    ON notes(todo_id)    WHERE todo_id    IS NOT NULL;

CREATE TABLE secrets (
  name       TEXT PRIMARY KEY,
  ciphertext BLOB NOT NULL,
  nonce      BLOB NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE sync_state (
  source     TEXT PRIMARY KEY,
  cursor     TEXT NOT NULL DEFAULT '',
  last_ok_at INTEGER,
  last_err   TEXT NOT NULL DEFAULT ''
);
```

- [ ] **Step 1.2: Update `internal/store/migrate_test.go`**

In `TestMigrate_AppliesOnFreshDB`, change the table list to:

```go
for _, table := range []string{"todos", "tags", "todo_tags", "todo_events", "kv", "timer_sessions", "meetings", "notes", "secrets", "sync_state"} {
```

Change `require.Equal(t, []int{1, 2}, versions)` to:

```go
require.Equal(t, []int{1, 2, 3}, versions)
```

- [ ] **Step 1.3: Run + commit**

```bash
go test ./internal/store/...
golangci-lint run
git add internal/store/migrations/0003_meetings_notes_secrets.sql internal/store/migrate_test.go
git commit -m "feat: add meetings/notes/secrets/sync_state migration"
```

Both `TestMigrate_AppliesOnFreshDB` and `TestMigrate_IsIdempotent` must PASS.

---

## Task 2: API DTOs + meeting/note event constants

**Files:**
- Modify: `internal/api/dto.go`
- Modify: `internal/api/events.go`

- [ ] **Step 2.1: Append to `internal/api/dto.go`**

```go
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
}

// Note is a markdown note attached to a meeting OR a todo (or neither — standalone).
type Note struct {
	ID        string `json:"id"`
	MeetingID string `json:"meetingId,omitempty"`
	TodoID    string `json:"todoId,omitempty"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// UpsertNoteRequest is the body of PUT /api/notes (creates or updates by attachment).
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
```

- [ ] **Step 2.2: Append to `internal/api/events.go`**

Inside the existing `const` block of event names, add:

```go
	EventMeetingUpserted          = "meeting.upserted"
	EventMeetingDeleted           = "meeting.deleted"
	EventMeetingNotificationFired = "meeting.notification_fired"
	EventNoteUpserted             = "note.upserted"
	EventSyncStateChanged         = "sync.state_changed"
```

Append at the end of the file:

```go
// MeetingUpsertedData is the payload of EventMeetingUpserted.
type MeetingUpsertedData struct {
	Meeting Meeting `json:"meeting"`
}

// MeetingDeletedData is the payload of EventMeetingDeleted.
type MeetingDeletedData struct {
	MeetingID string `json:"meetingId"`
}

// MeetingNotificationFiredData is the payload of EventMeetingNotificationFired.
type MeetingNotificationFiredData struct {
	MeetingID string `json:"meetingId"`
	FiredAt   int64  `json:"firedAt"`
}

// NoteUpsertedData is the payload of EventNoteUpserted.
type NoteUpsertedData struct {
	NoteID    string `json:"noteId"`
	MeetingID string `json:"meetingId,omitempty"`
	TodoID    string `json:"todoId,omitempty"`
}

// SyncStateChangedData is the payload of EventSyncStateChanged.
type SyncStateChangedData struct {
	Source   string `json:"source"`
	Status   string `json:"status"` // 'ok' | 'failed'
	LastErr  string `json:"lastErr,omitempty"`
}
```

- [ ] **Step 2.3: Verify and commit**

```bash
go build ./internal/api/...
golangci-lint run
git add internal/api/
git commit -m "feat: add meeting/note/secret DTOs and event types"
```

---

## Task 3: Repository interfaces

**Files:**
- Create: `internal/meeting/repo.go`
- Create: `internal/note/repo.go`
- Create: `internal/secret/repo.go`

- [ ] **Step 3.1: Write `internal/meeting/repo.go`**

```go
// Package meeting holds the meeting domain (service, repository contract, errors).
package meeting

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	ErrNotFound       = errors.New("meeting: not found")
	ErrManualOnly     = errors.New("meeting: only manual meetings may be edited")
	ErrInvalidRange   = errors.New("meeting: end_at must be > start_at")
)

// MeetingFilter narrows MeetingRepo.List.
type MeetingFilter struct {
	FromUnix     int64 // start_at >= FromUnix
	ToUnix       int64 // start_at <= ToUnix; 0 = no upper bound
	IncludeDone  bool  // include cancelled
	Limit        int   // 0 = no limit
}

// MeetingRepo persists meetings. //nolint:revive // domain naming intentional
type MeetingRepo interface {
	Create(ctx context.Context, m api.Meeting) error
	Get(ctx context.Context, id string) (api.Meeting, error)
	Update(ctx context.Context, id string, mutate func(*api.Meeting) error) (api.Meeting, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f MeetingFilter) ([]api.Meeting, error)

	// UpsertExternal performs INSERT-or-UPDATE keyed by (source, external_uid).
	// Returns the resulting meeting and a boolean indicating whether a row was inserted (true)
	// or an existing row was updated (false).
	UpsertExternal(ctx context.Context, m api.Meeting) (api.Meeting, bool, error)

	// MarkCancelled sets cancelled=1 for the given (source, external_uid). Used when
	// the CalDAV server no longer reports a previously-synced event.
	MarkCancelled(ctx context.Context, source api.MeetingSource, externalUID string) error

	// PendingNotification returns meetings that satisfy
	// start_at - notify_min*60 ≤ now AND notified_at IS NULL AND NOT cancelled AND deleted_at IS NULL.
	// defaultNotifyMin is used when the row's notify_min is NULL.
	PendingNotification(ctx context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error)

	// MarkNotified sets notified_at on a single meeting.
	MarkNotified(ctx context.Context, id string, at int64) error
}

// SyncStateRepo tracks per-source sync cursors and last-error strings.
type SyncStateRepo interface {
	Get(ctx context.Context, source string) (api.SyncStateEntry, error)
	Save(ctx context.Context, entry api.SyncStateEntry) error
	List(ctx context.Context) ([]api.SyncStateEntry, error)
}
```

- [ ] **Step 3.2: Write `internal/note/repo.go`**

```go
// Package note holds short-form attached notes (markdown body, attached to meeting or todo).
package note

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// ErrNotFound is returned when a note id does not exist.
var ErrNotFound = errors.New("note: not found")

// NoteFilter narrows List.
type NoteFilter struct {
	MeetingID string
	TodoID    string
	Limit     int
}

// NoteRepo persists notes. //nolint:revive // domain naming intentional
type NoteRepo interface {
	Upsert(ctx context.Context, n api.Note) error
	Get(ctx context.Context, id string) (api.Note, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f NoteFilter) ([]api.Note, error)

	// FindByAttachment returns the note attached to the given (meetingID, todoID) pair,
	// or ErrNotFound if none. Exactly one of meetingID / todoID must be non-empty.
	FindByAttachment(ctx context.Context, meetingID, todoID string) (api.Note, error)
}
```

- [ ] **Step 3.3: Write `internal/secret/repo.go`**

```go
// Package secret stores AES-256-GCM-encrypted credentials with a master key from
// the OS keyring (production) or an env var (tests).
package secret

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a secret name is unknown.
var ErrNotFound = errors.New("secret: not found")

// EncryptedSecret is the persisted form: ciphertext + nonce, never the plaintext.
type EncryptedSecret struct {
	Name       string
	Ciphertext []byte
	Nonce      []byte
	UpdatedAt  int64
}

// SecretRepo persists encrypted secrets. //nolint:revive // domain naming intentional
type SecretRepo interface {
	Get(ctx context.Context, name string) (EncryptedSecret, error)
	Set(ctx context.Context, s EncryptedSecret) error
	Delete(ctx context.Context, name string) error
	ListNames(ctx context.Context) ([]string, error)
}
```

- [ ] **Step 3.4: Verify + commit**

```bash
go build ./internal/meeting/... ./internal/note/... ./internal/secret/...
golangci-lint run
git add internal/meeting/repo.go internal/note/repo.go internal/secret/repo.go
git commit -m "feat: define meeting/note/secret repository interfaces"
```

---

## Task 4: SQLite repositories (Meeting, Note, Secret, SyncState)

**Files:**
- Create: `internal/store/meeting_repo.go`
- Create: `internal/store/note_repo.go`
- Create: `internal/store/secret_repo.go`
- Create: `internal/store/sync_state_repo.go`

- [ ] **Step 4.1: Write `internal/store/meeting_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// MeetingRepo is the SQLite-backed implementation of meeting.MeetingRepo. //nolint:revive // domain naming intentional
type MeetingRepo struct {
	db *sql.DB
}

// NewMeetingRepo constructs a MeetingRepo over db.
func NewMeetingRepo(db *sql.DB) *MeetingRepo { return &MeetingRepo{db: db} }

// Create inserts a meeting. ID and timestamps must be set by caller.
func (r *MeetingRepo) Create(ctx context.Context, m api.Meeting) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO meetings(id, source, external_uid, external_etag, title, description, location,
		                     start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, string(m.Source), nullStr(m.ExternalUID), nullStr(m.ExternalETag),
		m.Title, m.Description, m.Location,
		m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, boolToInt(m.Cancelled),
		m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert meeting: %w", err)
	}
	return nil
}

// Get returns a non-deleted meeting by id.
func (r *MeetingRepo) Get(ctx context.Context, id string) (api.Meeting, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, source, external_uid, external_etag, title, description, location,
		       start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at
		FROM meetings WHERE id = ? AND deleted_at IS NULL`, id)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Meeting{}, meeting.ErrNotFound
	}
	return m, err
}

// Update loads, mutates, saves atomically.
func (r *MeetingRepo) Update(ctx context.Context, id string, mutate func(*api.Meeting) error) (api.Meeting, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Meeting{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, source, external_uid, external_etag, title, description, location,
		       start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at
		FROM meetings WHERE id = ? AND deleted_at IS NULL`, id)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Meeting{}, meeting.ErrNotFound
	}
	if err != nil {
		return api.Meeting{}, err
	}
	if err := mutate(&m); err != nil {
		return api.Meeting{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE meetings SET source=?, external_uid=?, external_etag=?, title=?, description=?, location=?,
		                    start_at=?, end_at=?, notify_min=?, notified_at=?, cancelled=?, updated_at=?
		WHERE id=?`,
		string(m.Source), nullStr(m.ExternalUID), nullStr(m.ExternalETag),
		m.Title, m.Description, m.Location,
		m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, boolToInt(m.Cancelled),
		m.UpdatedAt, m.ID)
	if err != nil {
		return api.Meeting{}, fmt.Errorf("update meeting: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.Meeting{}, fmt.Errorf("commit tx: %w", err)
	}
	return m, nil
}

// Delete soft-deletes by setting deleted_at.
func (r *MeetingRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE meetings SET deleted_at=strftime('%s','now') WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete meeting: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return meeting.ErrNotFound
	}
	return nil
}

// List filters meetings by start range and cancellation state.
func (r *MeetingRepo) List(ctx context.Context, f meeting.MeetingFilter) ([]api.Meeting, error) {
	conds := []string{"deleted_at IS NULL"}
	var args []any
	if !f.IncludeDone {
		conds = append(conds, "cancelled = 0")
	}
	if f.FromUnix > 0 {
		conds = append(conds, "start_at >= ?")
		args = append(args, f.FromUnix)
	}
	if f.ToUnix > 0 {
		conds = append(conds, "start_at <= ?")
		args = append(args, f.ToUnix)
	}
	q := `SELECT id, source, external_uid, external_etag, title, description, location,
	             start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at
	      FROM meetings WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY start_at ASC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit) //nolint:gosec // limit is a controlled int
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Meeting
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertExternal inserts or updates by (source, external_uid).
func (r *MeetingRepo) UpsertExternal(ctx context.Context, m api.Meeting) (api.Meeting, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Meeting{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, source, external_uid, external_etag, title, description, location,
		       start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at
		FROM meetings WHERE source = ? AND external_uid = ? AND deleted_at IS NULL`,
		string(m.Source), m.ExternalUID)
	existing, err := scanMeeting(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Insert new
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO meetings(id, source, external_uid, external_etag, title, description, location,
			                     start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, string(m.Source), m.ExternalUID, nullStr(m.ExternalETag),
			m.Title, m.Description, m.Location,
			m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, boolToInt(m.Cancelled),
			m.CreatedAt, m.UpdatedAt,
		); err != nil {
			return api.Meeting{}, false, fmt.Errorf("insert external: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return api.Meeting{}, false, fmt.Errorf("commit: %w", err)
		}
		return m, true, nil
	case err != nil:
		return api.Meeting{}, false, err
	default:
		// Update existing — preserve ID, created_at, notify_min override (don't clobber user-set value),
		// and notified_at unless start_at is changing (NotificationScheduler handles that decision).
		preserved := existing
		preserved.ExternalETag = m.ExternalETag
		preserved.Title = m.Title
		preserved.Description = m.Description
		preserved.Location = m.Location
		preserved.StartAt = m.StartAt
		preserved.EndAt = m.EndAt
		preserved.Cancelled = m.Cancelled
		preserved.UpdatedAt = m.UpdatedAt
		if m.StartAt != existing.StartAt {
			// Reset notification when the meeting was rescheduled.
			preserved.NotifiedAt = nil
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE meetings SET external_etag=?, title=?, description=?, location=?,
			                    start_at=?, end_at=?, notified_at=?, cancelled=?, updated_at=?
			WHERE id=?`,
			nullStr(preserved.ExternalETag), preserved.Title, preserved.Description, preserved.Location,
			preserved.StartAt, preserved.EndAt, preserved.NotifiedAt, boolToInt(preserved.Cancelled),
			preserved.UpdatedAt, preserved.ID); err != nil {
			return api.Meeting{}, false, fmt.Errorf("update external: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return api.Meeting{}, false, fmt.Errorf("commit: %w", err)
		}
		return preserved, false, nil
	}
}

// MarkCancelled flips cancelled=1 for the given (source, external_uid).
func (r *MeetingRepo) MarkCancelled(ctx context.Context, source api.MeetingSource, externalUID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE meetings SET cancelled=1, updated_at=strftime('%s','now')
		 WHERE source=? AND external_uid=? AND deleted_at IS NULL`,
		string(source), externalUID)
	if err != nil {
		return fmt.Errorf("mark cancelled: %w", err)
	}
	return nil
}

// PendingNotification returns meetings ready to be notified.
func (r *MeetingRepo) PendingNotification(ctx context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source, external_uid, external_etag, title, description, location,
		       start_at, end_at, notify_min, notified_at, cancelled, created_at, updated_at
		FROM meetings
		WHERE deleted_at IS NULL
		  AND cancelled = 0
		  AND notified_at IS NULL
		  AND start_at - COALESCE(notify_min, ?) * 60 <= ?
		  AND start_at >= ?
		ORDER BY start_at ASC`,
		defaultNotifyMin, now, now)
	if err != nil {
		return nil, fmt.Errorf("pending notify: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Meeting
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkNotified sets notified_at on a single row.
func (r *MeetingRepo) MarkNotified(ctx context.Context, id string, at int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE meetings SET notified_at=? WHERE id=?`, at, id)
	if err != nil {
		return fmt.Errorf("mark notified: %w", err)
	}
	return nil
}

func scanMeeting(s sessionScanner) (api.Meeting, error) {
	var m api.Meeting
	var srcStr string
	var extUID, extETag sql.NullString
	var notifyMin sql.NullInt64
	var notifiedAt sql.NullInt64
	var cancelledI int
	if err := s.Scan(&m.ID, &srcStr, &extUID, &extETag,
		&m.Title, &m.Description, &m.Location,
		&m.StartAt, &m.EndAt, &notifyMin, &notifiedAt, &cancelledI,
		&m.CreatedAt, &m.UpdatedAt); err != nil {
		return api.Meeting{}, err
	}
	m.Source = api.MeetingSource(srcStr)
	if extUID.Valid {
		m.ExternalUID = extUID.String
	}
	if extETag.Valid {
		m.ExternalETag = extETag.String
	}
	if notifyMin.Valid {
		v := int(notifyMin.Int64)
		m.NotifyMin = &v
	}
	if notifiedAt.Valid {
		v := notifiedAt.Int64
		m.NotifiedAt = &v
	}
	m.Cancelled = cancelledI != 0
	return m, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

Note: `sessionScanner` is the same scanner interface from `timer_repo.go` (already exists in the `store` package). `nullStr` already exists from `event_repo.go`.

- [ ] **Step 4.2: Write `internal/store/note_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

// NoteRepo is the SQLite-backed implementation of note.NoteRepo. //nolint:revive // domain naming intentional
type NoteRepo struct {
	db *sql.DB
}

// NewNoteRepo constructs a NoteRepo over db.
func NewNoteRepo(db *sql.DB) *NoteRepo { return &NoteRepo{db: db} }

// Upsert inserts or replaces a note by id.
func (r *NoteRepo) Upsert(ctx context.Context, n api.Note) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notes(id, meeting_id, todo_id, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  meeting_id = excluded.meeting_id,
		  todo_id = excluded.todo_id,
		  body = excluded.body,
		  updated_at = excluded.updated_at
	`, n.ID, nullStr(n.MeetingID), nullStr(n.TodoID), n.Body, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert note: %w", err)
	}
	return nil
}

// Get returns a non-deleted note by id.
func (r *NoteRepo) Get(ctx context.Context, id string) (api.Note, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
		FROM notes WHERE id = ? AND deleted_at IS NULL`, id)
	n, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Note{}, note.ErrNotFound
	}
	return n, err
}

// Delete soft-deletes the note.
func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE notes SET deleted_at=strftime('%s','now') WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return note.ErrNotFound
	}
	return nil
}

// List filters notes by attachment.
func (r *NoteRepo) List(ctx context.Context, f note.NoteFilter) ([]api.Note, error) {
	q := `SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
	      FROM notes WHERE deleted_at IS NULL`
	var args []any
	if f.MeetingID != "" {
		q += ` AND meeting_id = ?`
		args = append(args, f.MeetingID)
	}
	if f.TodoID != "" {
		q += ` AND todo_id = ?`
		args = append(args, f.TodoID)
	}
	q += ` ORDER BY updated_at DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit) //nolint:gosec // controlled int
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// FindByAttachment returns the (single) note attached to the given meeting OR todo.
func (r *NoteRepo) FindByAttachment(ctx context.Context, meetingID, todoID string) (api.Note, error) {
	if meetingID == "" && todoID == "" {
		return api.Note{}, errors.New("meetingID or todoID required")
	}
	var row *sql.Row
	if meetingID != "" {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
			FROM notes WHERE meeting_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`,
			meetingID)
	} else {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
			FROM notes WHERE todo_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`,
			todoID)
	}
	n, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Note{}, note.ErrNotFound
	}
	return n, err
}

func scanNote(s sessionScanner) (api.Note, error) {
	var n api.Note
	if err := s.Scan(&n.ID, &n.MeetingID, &n.TodoID, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return api.Note{}, err
	}
	return n, nil
}
```

- [ ] **Step 4.3: Write `internal/store/secret_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/secret"
)

// SecretRepo is the SQLite-backed implementation of secret.SecretRepo. //nolint:revive // domain naming intentional
type SecretRepo struct {
	db *sql.DB
}

// NewSecretRepo constructs a SecretRepo over db.
func NewSecretRepo(db *sql.DB) *SecretRepo { return &SecretRepo{db: db} }

// Get returns the encrypted secret by name.
func (r *SecretRepo) Get(ctx context.Context, name string) (secret.EncryptedSecret, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT name, ciphertext, nonce, updated_at FROM secrets WHERE name = ?`, name)
	var s secret.EncryptedSecret
	if err := row.Scan(&s.Name, &s.Ciphertext, &s.Nonce, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return secret.EncryptedSecret{}, secret.ErrNotFound
		}
		return secret.EncryptedSecret{}, fmt.Errorf("get secret: %w", err)
	}
	return s, nil
}

// Set inserts or updates a secret.
func (r *SecretRepo) Set(ctx context.Context, s secret.EncryptedSecret) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO secrets(name, ciphertext, nonce, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET ciphertext=excluded.ciphertext, nonce=excluded.nonce, updated_at=excluded.updated_at
	`, s.Name, s.Ciphertext, s.Nonce, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("set secret: %w", err)
	}
	return nil
}

// Delete removes a secret.
func (r *SecretRepo) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM secrets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// ListNames returns all known secret names (no values).
func (r *SecretRepo) ListNames(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4.4: Write `internal/store/sync_state_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

// SyncStateRepo is the SQLite-backed implementation of meeting.SyncStateRepo (re-used for any future syncer).
type SyncStateRepo struct {
	db *sql.DB
}

// NewSyncStateRepo constructs a SyncStateRepo over db.
func NewSyncStateRepo(db *sql.DB) *SyncStateRepo { return &SyncStateRepo{db: db} }

// Get returns the sync state for source. If absent, returns an empty entry (no error).
func (r *SyncStateRepo) Get(ctx context.Context, source string) (api.SyncStateEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT source, cursor, last_ok_at, last_err FROM sync_state WHERE source = ?`, source)
	var entry api.SyncStateEntry
	var lastOk sql.NullInt64
	if err := row.Scan(&entry.Source, &entry.Cursor, &lastOk, &entry.LastErr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return api.SyncStateEntry{Source: source}, nil
		}
		return api.SyncStateEntry{}, fmt.Errorf("get sync_state: %w", err)
	}
	if lastOk.Valid {
		v := lastOk.Int64
		entry.LastOkAt = &v
	}
	return entry, nil
}

// Save upserts a sync state entry.
func (r *SyncStateRepo) Save(ctx context.Context, e api.SyncStateEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sync_state(source, cursor, last_ok_at, last_err) VALUES (?, ?, ?, ?)
		ON CONFLICT(source) DO UPDATE SET cursor=excluded.cursor, last_ok_at=excluded.last_ok_at, last_err=excluded.last_err
	`, e.Source, e.Cursor, e.LastOkAt, e.LastErr)
	if err != nil {
		return fmt.Errorf("save sync_state: %w", err)
	}
	return nil
}

// List returns all sync state entries.
func (r *SyncStateRepo) List(ctx context.Context) ([]api.SyncStateEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT source, cursor, last_ok_at, last_err FROM sync_state ORDER BY source`)
	if err != nil {
		return nil, fmt.Errorf("list sync_state: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.SyncStateEntry
	for rows.Next() {
		var e api.SyncStateEntry
		var lastOk sql.NullInt64
		if err := rows.Scan(&e.Source, &e.Cursor, &lastOk, &e.LastErr); err != nil {
			return nil, err
		}
		if lastOk.Valid {
			v := lastOk.Int64
			e.LastOkAt = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4.5: Verify build + commit**

```bash
go build ./internal/store/...
golangci-lint run
git add internal/store/meeting_repo.go internal/store/note_repo.go internal/store/secret_repo.go internal/store/sync_state_repo.go
git commit -m "feat: implement SQLite repos for meeting/note/secret/sync_state"
```

---

## Task 5: Fake repos + conformance tests

**Files:**
- Create: `internal/meeting/fakerepo/meeting_repo.go`
- Create: `internal/note/fakerepo/note_repo.go`
- Create: `internal/secret/fakerepo/secret_repo.go`
- Modify: `internal/store/conformance_test.go`

- [ ] **Step 5.1: Write `internal/meeting/fakerepo/meeting_repo.go`**

```go
// Package fakerepo provides an in-memory meeting.MeetingRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// Meeting is an in-memory meeting.MeetingRepo.
type Meeting struct {
	mu      sync.Mutex
	byID    map[string]api.Meeting
	deleted map[string]bool
}

// NewMeeting constructs an empty in-memory meeting repo.
func NewMeeting() *Meeting {
	return &Meeting{byID: map[string]api.Meeting{}, deleted: map[string]bool{}}
}

// Create inserts m.
func (r *Meeting) Create(_ context.Context, m api.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[m.ID] = m
	return nil
}

// Get returns a non-deleted meeting.
func (r *Meeting) Get(_ context.Context, id string) (api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Meeting{}, meeting.ErrNotFound
	}
	m, ok := r.byID[id]
	if !ok {
		return api.Meeting{}, meeting.ErrNotFound
	}
	return m, nil
}

// Update applies mutate to the existing meeting.
func (r *Meeting) Update(_ context.Context, id string, mutate func(*api.Meeting) error) (api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Meeting{}, meeting.ErrNotFound
	}
	m, ok := r.byID[id]
	if !ok {
		return api.Meeting{}, meeting.ErrNotFound
	}
	if err := mutate(&m); err != nil {
		return api.Meeting{}, err
	}
	r.byID[id] = m
	return m, nil
}

// Delete soft-deletes by id.
func (r *Meeting) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return meeting.ErrNotFound
	}
	if r.deleted[id] {
		return meeting.ErrNotFound
	}
	r.deleted[id] = true
	return nil
}

// List filters in memory.
func (r *Meeting) List(_ context.Context, f meeting.MeetingFilter) ([]api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Meeting
	for id, m := range r.byID {
		if r.deleted[id] {
			continue
		}
		if !f.IncludeDone && m.Cancelled {
			continue
		}
		if f.FromUnix > 0 && m.StartAt < f.FromUnix {
			continue
		}
		if f.ToUnix > 0 && m.StartAt > f.ToUnix {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt < out[j].StartAt })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// UpsertExternal inserts-or-updates by (source, externalUID).
func (r *Meeting) UpsertExternal(_ context.Context, m api.Meeting) (api.Meeting, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, existing := range r.byID {
		if r.deleted[id] {
			continue
		}
		if existing.Source == m.Source && existing.ExternalUID == m.ExternalUID && m.ExternalUID != "" {
			preserved := existing
			preserved.ExternalETag = m.ExternalETag
			preserved.Title = m.Title
			preserved.Description = m.Description
			preserved.Location = m.Location
			preserved.StartAt = m.StartAt
			preserved.EndAt = m.EndAt
			preserved.Cancelled = m.Cancelled
			preserved.UpdatedAt = m.UpdatedAt
			if m.StartAt != existing.StartAt {
				preserved.NotifiedAt = nil
			}
			r.byID[id] = preserved
			return preserved, false, nil
		}
	}
	r.byID[m.ID] = m
	return m, true, nil
}

// MarkCancelled sets cancelled on the matching meeting.
func (r *Meeting) MarkCancelled(_ context.Context, source api.MeetingSource, externalUID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, m := range r.byID {
		if r.deleted[id] {
			continue
		}
		if m.Source == source && m.ExternalUID == externalUID {
			m.Cancelled = true
			r.byID[id] = m
		}
	}
	return nil
}

// PendingNotification scans in memory.
func (r *Meeting) PendingNotification(_ context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Meeting
	for id, m := range r.byID {
		if r.deleted[id] || m.Cancelled || m.NotifiedAt != nil {
			continue
		}
		nm := defaultNotifyMin
		if m.NotifyMin != nil {
			nm = *m.NotifyMin
		}
		if m.StartAt-int64(nm)*60 <= now && m.StartAt >= now {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt < out[j].StartAt })
	return out, nil
}

// MarkNotified sets notified_at.
func (r *Meeting) MarkNotified(_ context.Context, id string, at int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byID[id]
	if !ok {
		return meeting.ErrNotFound
	}
	m.NotifiedAt = &at
	r.byID[id] = m
	return nil
}
```

- [ ] **Step 5.2: Write `internal/note/fakerepo/note_repo.go`**

```go
// Package fakerepo provides an in-memory note.NoteRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

// Note is an in-memory note.NoteRepo.
type Note struct {
	mu      sync.Mutex
	byID    map[string]api.Note
	deleted map[string]bool
}

// NewNote constructs an empty in-memory note repo.
func NewNote() *Note { return &Note{byID: map[string]api.Note{}, deleted: map[string]bool{}} }

// Upsert inserts or replaces by id.
func (r *Note) Upsert(_ context.Context, n api.Note) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[n.ID] = n
	delete(r.deleted, n.ID)
	return nil
}

// Get returns a non-deleted note.
func (r *Note) Get(_ context.Context, id string) (api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Note{}, note.ErrNotFound
	}
	n, ok := r.byID[id]
	if !ok {
		return api.Note{}, note.ErrNotFound
	}
	return n, nil
}

// Delete soft-deletes.
func (r *Note) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return note.ErrNotFound
	}
	if r.deleted[id] {
		return note.ErrNotFound
	}
	r.deleted[id] = true
	return nil
}

// List filters in memory.
func (r *Note) List(_ context.Context, f note.NoteFilter) ([]api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Note
	for id, n := range r.byID {
		if r.deleted[id] {
			continue
		}
		if f.MeetingID != "" && n.MeetingID != f.MeetingID {
			continue
		}
		if f.TodoID != "" && n.TodoID != f.TodoID {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// FindByAttachment returns the latest note attached to (meetingID, todoID).
func (r *Note) FindByAttachment(_ context.Context, meetingID, todoID string) (api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *api.Note
	for id, n := range r.byID {
		if r.deleted[id] {
			continue
		}
		match := false
		if meetingID != "" && n.MeetingID == meetingID {
			match = true
		}
		if todoID != "" && n.TodoID == todoID {
			match = true
		}
		if !match {
			continue
		}
		if best == nil || n.UpdatedAt > best.UpdatedAt {
			n := n
			best = &n
		}
	}
	if best == nil {
		return api.Note{}, note.ErrNotFound
	}
	return *best, nil
}
```

- [ ] **Step 5.3: Write `internal/secret/fakerepo/secret_repo.go`**

```go
// Package fakerepo provides an in-memory secret.SecretRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/secret"
)

// Secret is an in-memory secret.SecretRepo.
type Secret struct {
	mu   sync.Mutex
	rows map[string]secret.EncryptedSecret
}

// NewSecret constructs an empty in-memory secret repo.
func NewSecret() *Secret { return &Secret{rows: map[string]secret.EncryptedSecret{}} }

// Get returns the encrypted secret.
func (r *Secret) Get(_ context.Context, name string) (secret.EncryptedSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.rows[name]
	if !ok {
		return secret.EncryptedSecret{}, secret.ErrNotFound
	}
	return v, nil
}

// Set upserts the secret.
func (r *Secret) Set(_ context.Context, s secret.EncryptedSecret) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[s.Name] = s
	return nil
}

// Delete removes the secret.
func (r *Secret) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, name)
	return nil
}

// ListNames returns sorted secret names.
func (r *Secret) ListNames(_ context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.rows))
	for k := range r.rows {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 5.4: Append to `internal/store/conformance_test.go`**

Add imports:

```go
"github.com/spk/spk-cockpit/internal/meeting"
meetingfake "github.com/spk/spk-cockpit/internal/meeting/fakerepo"
"github.com/spk/spk-cockpit/internal/note"
notefake "github.com/spk/spk-cockpit/internal/note/fakerepo"
"github.com/spk/spk-cockpit/internal/secret"
secretfake "github.com/spk/spk-cockpit/internal/secret/fakerepo"
```

Append at end:

```go
type meetingRepoCase struct {
	name string
	new  func(t *testing.T) meeting.MeetingRepo
}

func meetingRepoCases() []meetingRepoCase {
	return []meetingRepoCase{
		{"fake", func(_ *testing.T) meeting.MeetingRepo { return meetingfake.NewMeeting() }},
		{"sqlite", func(t *testing.T) meeting.MeetingRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewMeetingRepo(s.DB)
		}},
	}
}

func TestMeetingRepo_Conformance(t *testing.T) {
	for _, c := range meetingRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			// Create + Get
			m := api.Meeting{
				ID: "m-1", Source: api.MeetingSourceManual,
				Title: "Hello", StartAt: 1000, EndAt: 1500,
				CreatedAt: 100, UpdatedAt: 100,
			}
			require.NoError(t, r.Create(ctx, m))
			got, err := r.Get(ctx, "m-1")
			require.NoError(t, err)
			require.Equal(t, "Hello", got.Title)

			// UpsertExternal — insert
			ext := api.Meeting{
				ID: "m-2", Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "etag-1",
				Title: "External", StartAt: 2000, EndAt: 2500,
				CreatedAt: 100, UpdatedAt: 100,
			}
			ins, inserted, err := r.UpsertExternal(ctx, ext)
			require.NoError(t, err)
			require.True(t, inserted)
			require.Equal(t, "External", ins.Title)

			// UpsertExternal — update with same start_at: notified_at preserved.
			require.NoError(t, r.MarkNotified(ctx, ins.ID, 1900))
			ext2 := ext
			ext2.Title = "External v2"
			ext2.UpdatedAt = 200
			updated, inserted, err := r.UpsertExternal(ctx, ext2)
			require.NoError(t, err)
			require.False(t, inserted)
			require.Equal(t, "External v2", updated.Title)
			require.NotNil(t, updated.NotifiedAt)

			// UpsertExternal — update with NEW start_at: notified_at reset.
			ext3 := ext2
			ext3.StartAt = 3000
			ext3.UpdatedAt = 300
			rescheduled, _, err := r.UpsertExternal(ctx, ext3)
			require.NoError(t, err)
			require.Nil(t, rescheduled.NotifiedAt)

			// PendingNotification at t=2950, default 5 min: ext3 starts at 3000, notify_min default 5 → 3000 - 300 = 2700 ≤ 2950 → match.
			pending, err := r.PendingNotification(ctx, 2950, 5)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			require.Equal(t, "m-2", pending[0].ID)

			// MarkNotified hides it.
			require.NoError(t, r.MarkNotified(ctx, "m-2", 2950))
			pending, err = r.PendingNotification(ctx, 2950, 5)
			require.NoError(t, err)
			require.Len(t, pending, 0)

			// Delete
			require.NoError(t, r.Delete(ctx, "m-1"))
			_, err = r.Get(ctx, "m-1")
			require.ErrorIs(t, err, meeting.ErrNotFound)
		})
	}
}

type noteRepoCase struct {
	name string
	new  func(t *testing.T) note.NoteRepo
}

func noteRepoCases() []noteRepoCase {
	return []noteRepoCase{
		{"fake", func(_ *testing.T) note.NoteRepo { return notefake.NewNote() }},
		{"sqlite", func(t *testing.T) note.NoteRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewNoteRepo(s.DB)
		}},
	}
}

func TestNoteRepo_Conformance(t *testing.T) {
	for _, c := range noteRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			n := api.Note{ID: "n-1", MeetingID: "m-1", Body: "hello", CreatedAt: 100, UpdatedAt: 100}
			require.NoError(t, r.Upsert(ctx, n))
			got, err := r.Get(ctx, "n-1")
			require.NoError(t, err)
			require.Equal(t, "hello", got.Body)

			// Update body via Upsert with same ID.
			n.Body = "world"
			n.UpdatedAt = 200
			require.NoError(t, r.Upsert(ctx, n))

			byMeeting, err := r.FindByAttachment(ctx, "m-1", "")
			require.NoError(t, err)
			require.Equal(t, "world", byMeeting.Body)

			require.NoError(t, r.Delete(ctx, "n-1"))
			_, err = r.Get(ctx, "n-1")
			require.ErrorIs(t, err, note.ErrNotFound)
		})
	}
}

type secretRepoCase struct {
	name string
	new  func(t *testing.T) secret.SecretRepo
}

func secretRepoCases() []secretRepoCase {
	return []secretRepoCase{
		{"fake", func(_ *testing.T) secret.SecretRepo { return secretfake.NewSecret() }},
		{"sqlite", func(t *testing.T) secret.SecretRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewSecretRepo(s.DB)
		}},
	}
}

func TestSecretRepo_Conformance(t *testing.T) {
	for _, c := range secretRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			s := secret.EncryptedSecret{Name: "yandex_caldav", Ciphertext: []byte("ct"), Nonce: []byte("nn"), UpdatedAt: 100}
			require.NoError(t, r.Set(ctx, s))
			got, err := r.Get(ctx, "yandex_caldav")
			require.NoError(t, err)
			require.Equal(t, []byte("ct"), got.Ciphertext)

			names, err := r.ListNames(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"yandex_caldav"}, names)

			require.NoError(t, r.Delete(ctx, "yandex_caldav"))
			_, err = r.Get(ctx, "yandex_caldav")
			require.ErrorIs(t, err, secret.ErrNotFound)
		})
	}
}
```

- [ ] **Step 5.5: Run + commit**

```bash
go test ./internal/store/... -v -run "TestMeetingRepo_Conformance|TestNoteRepo_Conformance|TestSecretRepo_Conformance"
go test ./internal/...
golangci-lint run
git add internal/meeting/fakerepo/ internal/note/fakerepo/ internal/secret/fakerepo/ internal/store/conformance_test.go
git commit -m "feat: add meeting/note/secret fake repos and conformance tests"
```

All `fake` and `sqlite` subtests must PASS for all three new conformance tests.

---

## Task 6: Meeting domain service (TDD)

**Files:**
- Create: `internal/meeting/service.go`
- Create: `internal/meeting/service_test.go`

The service handles **manual** meetings (CRUD with audit-event-style invariants), **CalDAV-sourced** meetings (UpsertFromCalDAV / MarkExternalCancelled), and convenience queries (Next, Upcoming). It emits `MeetingUpserted` / `MeetingDeleted` events but DOES NOT touch notifications — that's the scheduler's job.

- [ ] **Step 6.1: Write `internal/meeting/service_test.go`**

```go
package meeting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/meeting/fakerepo"
)

func newSvc(t *testing.T, t0 time.Time) (*meeting.Service, *fakerepo.Meeting) {
	r := fakerepo.NewMeeting()
	c := clock.NewFake(t0)
	return meeting.NewService(r, c, nil), r
}

func TestService_CreateManual_AssignsIDAndTimestamps(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newSvc(t, t0)
	got, err := s.CreateManual(context.Background(), api.CreateMeetingRequest{
		Title: "Standup", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(90 * time.Minute).Unix(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, api.MeetingSourceManual, got.Source)
	require.Equal(t, "Standup", got.Title)
	require.Equal(t, t0.Unix(), got.CreatedAt)
}

func TestService_CreateManual_RejectsBadRange(t *testing.T) {
	s, _ := newSvc(t, time.Now())
	_, err := s.CreateManual(context.Background(), api.CreateMeetingRequest{
		Title: "Bad", StartAt: 200, EndAt: 100,
	})
	require.ErrorIs(t, err, meeting.ErrInvalidRange)
}

func TestService_UpdateManual_OnlyManual(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newSvc(t, t0)
	// Inject a caldav meeting directly through the repo.
	cal := api.Meeting{
		ID: "ext", Source: api.MeetingSourceCalDAV, ExternalUID: "uid", Title: "From cal",
		StartAt: t0.Unix(), EndAt: t0.Add(time.Hour).Unix(),
		CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}
	require.NoError(t, r.Create(context.Background(), cal))

	newTitle := "edited"
	_, err := s.UpdateManual(context.Background(), "ext", api.UpdateMeetingRequest{Title: &newTitle})
	require.ErrorIs(t, err, meeting.ErrManualOnly)
}

func TestService_UpsertFromCalDAV_ReschedulesResetsNotification(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newSvc(t, t0)
	ctx := context.Background()

	// First sync — insert.
	first, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		ID: "x", Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e1",
		Title: "Sync me", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(2 * time.Hour).Unix(),
	})
	require.NoError(t, err)
	require.Equal(t, "Sync me", first.Title)

	// Pretend the scheduler fired a notification.
	require.NoError(t, r.MarkNotified(ctx, first.ID, t0.Unix()))

	// Second sync — same UID, same StartAt, only title changed: notified_at stays.
	second, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e2",
		Title: "Sync me — renamed", StartAt: first.StartAt, EndAt: first.EndAt,
	})
	require.NoError(t, err)
	require.Equal(t, "Sync me — renamed", second.Title)
	require.NotNil(t, second.NotifiedAt)

	// Third sync — StartAt moved later: notified_at MUST reset.
	third, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e3",
		Title: "Sync me — renamed", StartAt: first.StartAt + 600, EndAt: first.EndAt + 600,
	})
	require.NoError(t, err)
	require.Nil(t, third.NotifiedAt)
}

func TestService_NextMeeting_ReturnsEarliestUpcoming(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newSvc(t, t0)
	ctx := context.Background()
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Later", StartAt: t0.Add(2 * time.Hour).Unix(), EndAt: t0.Add(3 * time.Hour).Unix(),
	})
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Sooner", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(90 * time.Minute).Unix(),
	})
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Past", StartAt: t0.Add(-time.Hour).Unix(), EndAt: t0.Add(-30 * time.Minute).Unix(),
	})

	next, err := s.Next(ctx)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, "Sooner", next.Title)
}

func TestService_Next_NilWhenEmpty(t *testing.T) {
	s, _ := newSvc(t, time.Now())
	got, err := s.Next(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}
```

- [ ] **Step 6.2: Run to confirm failure**

```bash
go test ./internal/meeting/...
```

Expected: undefined symbols.

- [ ] **Step 6.3: Write `internal/meeting/service.go`**

```go
package meeting

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. May be nil — service is nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the meeting domain entry point.
type Service struct {
	repo  MeetingRepo
	clock clock.Clock
	bus   EventPublisher
}

// NewService wires the service.
func NewService(r MeetingRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{repo: r, clock: c, bus: bus}
}

// Clock exposes the injected clock (used by tests / scheduler).
func (s *Service) Clock() clock.Clock { return s.clock }

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// CreateManual creates a manual meeting and returns it.
func (s *Service) CreateManual(ctx context.Context, req api.CreateMeetingRequest) (api.Meeting, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return api.Meeting{}, errors.New("title is required")
	}
	if req.EndAt <= req.StartAt {
		return api.Meeting{}, ErrInvalidRange
	}
	now := s.clock.Now().Unix()
	m := api.Meeting{
		ID:          ulid.MustNew(ulid.Now(), rand.Reader).String(),
		Source:      api.MeetingSourceManual,
		Title:       title,
		Description: req.Description,
		Location:    req.Location,
		StartAt:     req.StartAt,
		EndAt:       req.EndAt,
		NotifyMin:   req.NotifyMin,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return api.Meeting{}, fmt.Errorf("create: %w", err)
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: m})
	return m, nil
}

// UpdateManual mutates a manual-source meeting. CalDAV meetings are read-only.
func (s *Service) UpdateManual(ctx context.Context, id string, req api.UpdateMeetingRequest) (api.Meeting, error) {
	now := s.clock.Now().Unix()
	updated, err := s.repo.Update(ctx, id, func(m *api.Meeting) error {
		if m.Source != api.MeetingSourceManual {
			return ErrManualOnly
		}
		if req.Title != nil {
			t := strings.TrimSpace(*req.Title)
			if t == "" {
				return errors.New("title is required")
			}
			m.Title = t
		}
		if req.Description != nil {
			m.Description = *req.Description
		}
		if req.Location != nil {
			m.Location = *req.Location
		}
		if req.StartAt != nil && *req.StartAt != m.StartAt {
			m.StartAt = *req.StartAt
			m.NotifiedAt = nil // reschedule resets notification
		}
		if req.EndAt != nil {
			m.EndAt = *req.EndAt
		}
		if m.EndAt <= m.StartAt {
			return ErrInvalidRange
		}
		if req.NotifyMin != nil {
			m.NotifyMin = req.NotifyMin
			m.NotifiedAt = nil
		}
		m.UpdatedAt = now
		return nil
	})
	if err != nil {
		return api.Meeting{}, err
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: updated})
	return updated, nil
}

// DeleteManual soft-deletes a manual meeting (CalDAV meetings cannot be deleted via API — they're synced).
func (s *Service) DeleteManual(ctx context.Context, id string) error {
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if cur.Source != api.MeetingSourceManual {
		return ErrManualOnly
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publish(api.EventMeetingDeleted, api.MeetingDeletedData{MeetingID: id})
	return nil
}

// Get loads a meeting (any source).
func (s *Service) Get(ctx context.Context, id string) (api.Meeting, error) {
	return s.repo.Get(ctx, id)
}

// List returns meetings in the given range.
func (s *Service) List(ctx context.Context, f MeetingFilter) ([]api.Meeting, error) {
	return s.repo.List(ctx, f)
}

// Next returns the earliest upcoming non-cancelled meeting, or nil.
func (s *Service) Next(ctx context.Context) (*api.Meeting, error) {
	now := s.clock.Now().Unix()
	list, err := s.repo.List(ctx, MeetingFilter{FromUnix: now, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	m := list[0]
	return &m, nil
}

// UpsertFromCalDAV inserts or updates a CalDAV-sourced meeting. ID is assigned for new rows.
func (s *Service) UpsertFromCalDAV(ctx context.Context, m api.Meeting) (api.Meeting, error) {
	if m.Source != api.MeetingSourceCalDAV || m.ExternalUID == "" {
		return api.Meeting{}, errors.New("UpsertFromCalDAV: source must be 'caldav' and ExternalUID required")
	}
	if m.EndAt <= m.StartAt {
		return api.Meeting{}, ErrInvalidRange
	}
	now := s.clock.Now().Unix()
	if m.ID == "" {
		m.ID = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	out, _, err := s.repo.UpsertExternal(ctx, m)
	if err != nil {
		return api.Meeting{}, err
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: out})
	return out, nil
}

// MarkExternalCancelled flags a CalDAV meeting as cancelled (Yandex no longer reports it).
func (s *Service) MarkExternalCancelled(ctx context.Context, externalUID string) error {
	return s.repo.MarkCancelled(ctx, api.MeetingSourceCalDAV, externalUID)
}

// PendingNotification proxies to the repo (used by the scheduler).
func (s *Service) PendingNotification(ctx context.Context, defaultNotifyMin int) ([]api.Meeting, error) {
	return s.repo.PendingNotification(ctx, s.clock.Now().Unix(), defaultNotifyMin)
}

// MarkNotified records that a meeting was successfully notified.
func (s *Service) MarkNotified(ctx context.Context, id string) error {
	now := s.clock.Now().Unix()
	if err := s.repo.MarkNotified(ctx, id, now); err != nil {
		return err
	}
	s.publish(api.EventMeetingNotificationFired, api.MeetingNotificationFiredData{
		MeetingID: id, FiredAt: now,
	})
	return nil
}
```

- [ ] **Step 6.4: Run + commit**

```bash
go test ./internal/meeting/... -v
golangci-lint run
git add internal/meeting/service.go internal/meeting/service_test.go
git commit -m "feat: implement Meeting domain service"
```

All 6 tests PASS.

---

## Task 7: Note domain service

**Files:**
- Create: `internal/note/service.go`
- Create: `internal/note/service_test.go`

- [ ] **Step 7.1: Write `internal/note/service_test.go`**

```go
package note_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/note/fakerepo"
)

func TestService_Upsert_AttachToMeeting_AssignsIDAndTimestamps(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(t0), nil)

	got, err := s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m-1", Body: "hello"})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, "m-1", got.MeetingID)
	require.Equal(t, "hello", got.Body)
	require.Equal(t, t0.Unix(), got.CreatedAt)
	require.Equal(t, t0.Unix(), got.UpdatedAt)
}

func TestService_Upsert_ReusesExistingNoteOnSameMeeting(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := clock.NewFake(t0)
	s := note.NewService(fakerepo.NewNote(), c, nil)
	ctx := context.Background()

	first, err := s.Upsert(ctx, api.UpsertNoteRequest{MeetingID: "m-1", Body: "v1"})
	require.NoError(t, err)

	c.Advance(time.Minute)
	second, err := s.Upsert(ctx, api.UpsertNoteRequest{MeetingID: "m-1", Body: "v2"})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "same meeting should reuse same note id")
	require.Equal(t, "v2", second.Body)
	require.Equal(t, first.CreatedAt, second.CreatedAt)
	require.Greater(t, second.UpdatedAt, first.UpdatedAt)
}

func TestService_Upsert_RequiresExactlyOneAttachment(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()), nil)

	_, err := s.Upsert(context.Background(), api.UpsertNoteRequest{Body: "no attachment"})
	require.Error(t, err)

	_, err = s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m", TodoID: "t", Body: "both"})
	require.Error(t, err)
}

func TestService_Delete(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()), nil)
	got, _ := s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m-1", Body: "x"})
	require.NoError(t, s.Delete(context.Background(), got.ID))
	_, err := s.Get(context.Background(), got.ID)
	require.ErrorIs(t, err, note.ErrNotFound)
}

func TestService_FindByMeeting_NilWhenAbsent(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()), nil)
	got, err := s.FindByMeeting(context.Background(), "m-x")
	require.NoError(t, err)
	require.Nil(t, got)
}
```

- [ ] **Step 7.2: Write `internal/note/service.go`**

```go
package note

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. May be nil — service is nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the note domain entry point.
type Service struct {
	repo  NoteRepo
	clock clock.Clock
	bus   EventPublisher
}

// NewService wires the service.
func NewService(r NoteRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{repo: r, clock: c, bus: bus}
}

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// Upsert creates or updates the (single) note attached to a meeting OR a todo.
// Exactly one of MeetingID/TodoID must be non-empty.
func (s *Service) Upsert(ctx context.Context, req api.UpsertNoteRequest) (api.Note, error) {
	if req.MeetingID == "" && req.TodoID == "" {
		return api.Note{}, errors.New("MeetingID or TodoID is required")
	}
	if req.MeetingID != "" && req.TodoID != "" {
		return api.Note{}, errors.New("only one of MeetingID / TodoID may be set")
	}
	now := s.clock.Now().Unix()

	existing, err := s.repo.FindByAttachment(ctx, req.MeetingID, req.TodoID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return api.Note{}, fmt.Errorf("find: %w", err)
	}
	var n api.Note
	if errors.Is(err, ErrNotFound) {
		n = api.Note{
			ID:        ulid.MustNew(ulid.Now(), rand.Reader).String(),
			MeetingID: req.MeetingID,
			TodoID:    req.TodoID,
			Body:      req.Body,
			CreatedAt: now,
			UpdatedAt: now,
		}
	} else {
		n = existing
		n.Body = req.Body
		n.UpdatedAt = now
	}
	if err := s.repo.Upsert(ctx, n); err != nil {
		return api.Note{}, fmt.Errorf("upsert: %w", err)
	}
	s.publish(api.EventNoteUpserted, api.NoteUpsertedData{
		NoteID: n.ID, MeetingID: n.MeetingID, TodoID: n.TodoID,
	})
	return n, nil
}

// Get loads a note by id.
func (s *Service) Get(ctx context.Context, id string) (api.Note, error) {
	return s.repo.Get(ctx, id)
}

// Delete soft-deletes a note.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// FindByMeeting returns the note attached to a meeting, or (nil, nil) if absent.
func (s *Service) FindByMeeting(ctx context.Context, meetingID string) (*api.Note, error) {
	n, err := s.repo.FindByAttachment(ctx, meetingID, "")
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// FindByTodo returns the note attached to a todo, or (nil, nil) if absent.
func (s *Service) FindByTodo(ctx context.Context, todoID string) (*api.Note, error) {
	n, err := s.repo.FindByAttachment(ctx, "", todoID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// List returns notes filtered by attachment.
func (s *Service) List(ctx context.Context, f NoteFilter) ([]api.Note, error) {
	return s.repo.List(ctx, f)
}
```

- [ ] **Step 7.3: Run + commit**

```bash
go test ./internal/note/... -v
golangci-lint run
git add internal/note/service.go internal/note/service_test.go
git commit -m "feat: implement Note domain service"
```

5/5 tests must PASS.

---

## Task 8: Secret domain service (AES-256-GCM + KeyResolver)

**Files:**
- Create: `internal/secret/keyring.go` — `KeyResolver` interface + `EnvResolver` + `KeyringResolver`
- Create: `internal/secret/service.go`
- Create: `internal/secret/service_test.go`

- [ ] **Step 8.1: Add dependency**

```bash
go get github.com/zalando/go-keyring
go mod tidy
```

- [ ] **Step 8.2: Write `internal/secret/keyring.go`**

```go
package secret

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// MasterKeyName is the keyring entry name (under service "spk-cockpit").
const MasterKeyName = "master_key"

// keyringService is the OS keyring service identifier.
const keyringService = "spk-cockpit"

// KeyResolver provides the 32-byte AES-256 master key.
type KeyResolver interface {
	Resolve() ([]byte, error)
}

// EnvResolver reads a base64-encoded 32-byte key from SPK_COCKPIT_MASTER_KEY (testing / fallback).
type EnvResolver struct {
	EnvVar string
}

// NewEnvResolver constructs an EnvResolver. Empty envVar uses SPK_COCKPIT_MASTER_KEY.
func NewEnvResolver(envVar string) *EnvResolver {
	if envVar == "" {
		envVar = "SPK_COCKPIT_MASTER_KEY"
	}
	return &EnvResolver{EnvVar: envVar}
}

// Resolve decodes the base64 env var; missing => error.
func (e *EnvResolver) Resolve() ([]byte, error) {
	raw := os.Getenv(e.EnvVar)
	if raw == "" {
		return nil, fmt.Errorf("env var %s not set", e.EnvVar)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", e.EnvVar, err)
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	return key, nil
}

// KeyringResolver fetches the master key from the OS keyring; generates one on first use.
type KeyringResolver struct{}

// NewKeyringResolver constructs a KeyringResolver.
func NewKeyringResolver() *KeyringResolver { return &KeyringResolver{} }

// Resolve returns the master key, generating + storing one if absent.
func (k *KeyringResolver) Resolve() ([]byte, error) {
	raw, err := keyring.Get(keyringService, MasterKeyName)
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("decode keyring entry: %w", err)
		}
		if len(key) != 32 {
			return nil, errors.New("keyring master key not 32 bytes")
		}
		return key, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("keyring access: %w", err)
	}
	// Generate a new key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := keyring.Set(keyringService, MasterKeyName, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("store keyring: %w", err)
	}
	return key, nil
}

// ResolveOrFallback tries primary, then fallback. Useful for "keyring with env-var override".
func ResolveOrFallback(primary, fallback KeyResolver) ([]byte, error) {
	if primary != nil {
		key, err := primary.Resolve()
		if err == nil {
			return key, nil
		}
	}
	if fallback == nil {
		return nil, errors.New("no resolver available")
	}
	return fallback.Resolve()
}
```

- [ ] **Step 8.3: Write `internal/secret/service.go`**

```go
package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/clock"
)

// Service encrypts/decrypts secrets using AES-256-GCM.
type Service struct {
	repo   SecretRepo
	clock  clock.Clock
	gcm    cipher.AEAD
}

// NewService constructs a Service. masterKey must be 32 bytes (AES-256).
func NewService(r SecretRepo, c clock.Clock, masterKey []byte) (*Service, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Service{repo: r, clock: c, gcm: gcm}, nil
}

// Set encrypts and stores a secret value.
func (s *Service) Set(ctx context.Context, name, value string) error {
	if name == "" {
		return errors.New("name is required")
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	ct := s.gcm.Seal(nil, nonce, []byte(value), []byte(name))
	return s.repo.Set(ctx, EncryptedSecret{
		Name: name, Ciphertext: ct, Nonce: nonce, UpdatedAt: s.clock.Now().Unix(),
	})
}

// Get decrypts and returns a secret value.
func (s *Service) Get(ctx context.Context, name string) (string, error) {
	row, err := s.repo.Get(ctx, name)
	if err != nil {
		return "", err
	}
	pt, err := s.gcm.Open(nil, row.Nonce, row.Ciphertext, []byte(row.Name))
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", name, err)
	}
	return string(pt), nil
}

// Delete removes a secret.
func (s *Service) Delete(ctx context.Context, name string) error {
	return s.repo.Delete(ctx, name)
}

// ListNames returns sorted secret names (no values).
func (s *Service) ListNames(ctx context.Context) ([]string, error) {
	return s.repo.ListNames(ctx)
}
```

- [ ] **Step 8.4: Write `internal/secret/service_test.go`**

```go
package secret_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/secret/fakerepo"
)

func newSvc(t *testing.T) *secret.Service {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	s, err := secret.NewService(fakerepo.NewSecret(), clock.NewFake(time.Unix(1700000000, 0)), key)
	require.NoError(t, err)
	return s
}

func TestService_SetAndGet_Roundtrip(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "yandex_caldav", "secret-password-123"))
	got, err := s.Get(ctx, "yandex_caldav")
	require.NoError(t, err)
	require.Equal(t, "secret-password-123", got)
}

func TestService_Get_NotFound(t *testing.T) {
	s := newSvc(t)
	_, err := s.Get(context.Background(), "missing")
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestService_DifferentMasterKey_FailsToDecrypt(t *testing.T) {
	repo := fakerepo.NewSecret()
	c := clock.NewFake(time.Unix(1700000000, 0))

	key1 := make([]byte, 32)
	_, err := rand.Read(key1)
	require.NoError(t, err)
	s1, err := secret.NewService(repo, c, key1)
	require.NoError(t, err)
	require.NoError(t, s1.Set(context.Background(), "x", "v"))

	key2 := make([]byte, 32)
	_, err = rand.Read(key2)
	require.NoError(t, err)
	s2, err := secret.NewService(repo, c, key2)
	require.NoError(t, err)
	_, err = s2.Get(context.Background(), "x")
	require.Error(t, err)
}

func TestService_NewService_RequiresThirtyTwoByteKey(t *testing.T) {
	_, err := secret.NewService(fakerepo.NewSecret(), clock.NewFake(time.Now()), make([]byte, 16))
	require.Error(t, err)
}
```

- [ ] **Step 8.5: Run + commit**

```bash
go test ./internal/secret/... -v
golangci-lint run
git add internal/secret/
git commit -m "feat: implement Secret service with AES-256-GCM and KeyResolver"
```

4/4 tests must PASS.

---

## Task 9: HTTP handlers + daemon wiring

**Files:**
- Modify: `internal/server/server.go` (Deps gains 4 new fields)
- Modify: `internal/server/routes.go` (10+ new routes)
- Create: `internal/server/meeting_handler.go`
- Create: `internal/server/note_handler.go`
- Create: `internal/server/secret_handler.go`
- Modify: `internal/server/server_test.go` (wire fakes)

- [ ] **Step 9.1: Extend `Deps` in `internal/server/server.go`**

```go
type Deps struct {
	Todos    *todo.Service
	Tags     todo.TagRepo
	Bus      *eventbus.Bus
	Timer    *timer.Service
	Meetings *meeting.Service
	Notes    *note.Service
	Secrets  *secret.Service
	Sync     SyncTrigger
}

// SyncTrigger lets the server force a CalDAV sync from a CLI/UI request.
type SyncTrigger interface {
	TriggerNow(source string) error
	Status() []api.SyncStateEntry
}
```

Add imports:

```go
"github.com/spk/spk-cockpit/internal/meeting"
"github.com/spk/spk-cockpit/internal/note"
"github.com/spk/spk-cockpit/internal/secret"
```

- [ ] **Step 9.2: Create `internal/server/meeting_handler.go`**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
)

func handleListMeetings(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := meeting.MeetingFilter{}
		q := r.URL.Query()
		if v := q.Get("from"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.FromUnix = n
			}
		}
		if v := q.Get("to"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.ToUnix = n
			}
		}
		if v := q.Get("includeCancelled"); v == "1" || v == "true" {
			f.IncludeDone = true
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				f.Limit = n
			}
		}
		list, err := d.Meetings.List(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.list_failed", err.Error())
			return
		}
		if list == nil {
			list = []api.Meeting{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func handleNextMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next, err := d.Meetings.Next(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.next_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, next)
	}
}

func handleCreateMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateMeetingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		m, err := d.Meetings.CreateManual(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "meeting.create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, m)
	}
}

func handleGetMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := d.Meetings.Get(r.Context(), id)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

func handleUpdateMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req api.UpdateMeetingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		m, err := d.Meetings.UpdateManual(r.Context(), id, req)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if errors.Is(err, meeting.ErrManualOnly) {
			writeError(w, http.StatusForbidden, "meeting.manual_only", "only manual meetings may be edited")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "meeting.update_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

func handleDeleteMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Meetings.DeleteManual(r.Context(), id)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if errors.Is(err, meeting.ErrManualOnly) {
			writeError(w, http.StatusForbidden, "meeting.manual_only", "only manual meetings may be deleted")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSyncTrigger(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := r.PathValue("source")
		if d.Sync == nil {
			writeError(w, http.StatusServiceUnavailable, "sync.disabled", "sync is not configured")
			return
		}
		if err := d.Sync.TriggerNow(source); err != nil {
			writeError(w, http.StatusBadRequest, "sync.trigger_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleSyncStatus(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Sync == nil {
			writeJSON(w, http.StatusOK, []api.SyncStateEntry{})
			return
		}
		writeJSON(w, http.StatusOK, d.Sync.Status())
	}
}
```

- [ ] **Step 9.3: Create `internal/server/note_handler.go`**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

func handleUpsertNote(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.UpsertNoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		n, err := d.Notes.Upsert(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "note.upsert_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}

func handleDeleteNote(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Notes.Delete(r.Context(), id)
		if errors.Is(err, note.ErrNotFound) {
			writeError(w, http.StatusNotFound, "note.not_found", "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleNoteForMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		got, err := d.Notes.FindByMeeting(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.lookup_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

func handleNoteForTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		got, err := d.Notes.FindByTodo(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.lookup_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}
```

- [ ] **Step 9.4: Create `internal/server/secret_handler.go`**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/secret"
)

func handleListSecrets(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names, err := d.Secrets.ListNames(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "secret.list_failed", err.Error())
			return
		}
		out := make([]api.Secret, 0, len(names))
		for _, n := range names {
			out = append(out, api.Secret{Name: n, UpdatedAt: time.Now().Unix()})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func handleSetSecret(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req api.SetSecretRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := d.Secrets.Set(r.Context(), name, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "secret.set_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleDeleteSecret(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := d.Secrets.Delete(r.Context(), name); err != nil {
			if errors.Is(err, secret.ErrNotFound) {
				writeError(w, http.StatusNotFound, "secret.not_found", "not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "secret.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 9.5: Register routes in `internal/server/routes.go`**

After `mux.HandleFunc("GET /api/todos/{id}/sessions", handleTodoTimerSessions(d))` and before the SPA fallback, add:

```go
	mux.HandleFunc("GET /api/meetings", handleListMeetings(d))
	mux.HandleFunc("POST /api/meetings", handleCreateMeeting(d))
	mux.HandleFunc("GET /api/meetings/next", handleNextMeeting(d))
	mux.HandleFunc("GET /api/meetings/{id}", handleGetMeeting(d))
	mux.HandleFunc("PATCH /api/meetings/{id}", handleUpdateMeeting(d))
	mux.HandleFunc("DELETE /api/meetings/{id}", handleDeleteMeeting(d))
	mux.HandleFunc("GET /api/meetings/{id}/note", handleNoteForMeeting(d))

	mux.HandleFunc("PUT /api/notes", handleUpsertNote(d))
	mux.HandleFunc("DELETE /api/notes/{id}", handleDeleteNote(d))
	mux.HandleFunc("GET /api/todos/{id}/note", handleNoteForTodo(d))

	mux.HandleFunc("GET /api/secrets", handleListSecrets(d))
	mux.HandleFunc("PUT /api/secrets/{name}", handleSetSecret(d))
	mux.HandleFunc("DELETE /api/secrets/{name}", handleDeleteSecret(d))

	mux.HandleFunc("POST /api/sync/{source}", handleSyncTrigger(d))
	mux.HandleFunc("GET /api/sync", handleSyncStatus(d))
```

- [ ] **Step 9.6: Wire fakes into `newTestServer` in `internal/server/server_test.go`**

Add imports:

```go
"github.com/spk/spk-cockpit/internal/meeting"
meetingfake "github.com/spk/spk-cockpit/internal/meeting/fakerepo"
"github.com/spk/spk-cockpit/internal/note"
notefake "github.com/spk/spk-cockpit/internal/note/fakerepo"
"github.com/spk/spk-cockpit/internal/secret"
secretfake "github.com/spk/spk-cockpit/internal/secret/fakerepo"
"crypto/rand"
```

In `newTestServer`, after the existing timer wiring, add:

```go
	srv.Deps().Meetings = meeting.NewService(meetingfake.NewMeeting(), clock.NewFake(time.Unix(1700000000, 0)), bus)
	srv.Deps().Notes = note.NewService(notefake.NewNote(), clock.NewFake(time.Unix(1700000000, 0)), bus)

	masterKey := make([]byte, 32)
	_, _ = rand.Read(masterKey)
	secSvc, err := secret.NewService(secretfake.NewSecret(), clock.NewFake(time.Unix(1700000000, 0)), masterKey)
	require.NoError(t, err)
	srv.Deps().Secrets = secSvc
```

Add a basic happy-path integration test:

```go
func TestServer_MeetingCreateGetList(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()
	c := udsClient(sock)

	body, _ := json.Marshal(api.CreateMeetingRequest{
		Title: "Standup", StartAt: 2000, EndAt: 2500,
	})
	resp, err := c.Post("http://unix/api/meetings", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode)

	resp2, err := c.Get("http://unix/api/meetings")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)
	var list []api.Meeting
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&list))
	require.Len(t, list, 1)
	require.Equal(t, "Standup", list[0].Title)
}

func TestServer_NoteUpsertAndFindByMeeting(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()
	c := udsClient(sock)

	body, _ := json.Marshal(api.UpsertNoteRequest{MeetingID: "m-1", Body: "v1"})
	req, _ := http.NewRequest("PUT", "http://unix/api/notes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	resp2, err := c.Get("http://unix/api/meetings/m-1/note")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)
	var got *api.Note
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&got))
	require.NotNil(t, got)
	require.Equal(t, "v1", got.Body)
}
```

- [ ] **Step 9.7: Wire services into `runStart` in `internal/cli/start.go`**

After the timer service line, add:

```go
	meetingRepo := store.NewMeetingRepo(st.DB)
	noteRepo := store.NewNoteRepo(st.DB)
	secretRepo := store.NewSecretRepo(st.DB)
	syncStateRepo := store.NewSyncStateRepo(st.DB)

	meetingSvc := meeting.NewService(meetingRepo, clock.Real(), bus)
	noteSvc := note.NewService(noteRepo, clock.Real(), bus)

	masterKey, err := secret.ResolveOrFallback(secret.NewKeyringResolver(), secret.NewEnvResolver(""))
	if err != nil {
		return fmt.Errorf("master key: %w", err)
	}
	secretSvc, err := secret.NewService(secretRepo, clock.Real(), masterKey)
	if err != nil {
		return fmt.Errorf("secret service: %w", err)
	}
```

After `srv.Deps().Timer = timerSvc`, add:

```go
	srv.Deps().Meetings = meetingSvc
	srv.Deps().Notes = noteSvc
	srv.Deps().Secrets = secretSvc
```

Note: `Sync` is still nil at this point — Task 12 wires it.

Add imports:

```go
"github.com/spk/spk-cockpit/internal/meeting"
"github.com/spk/spk-cockpit/internal/note"
"github.com/spk/spk-cockpit/internal/secret"
```

`syncStateRepo` is unused right now; suppress with `_ = syncStateRepo` or annotate; Task 12 will use it. Use:

```go
	_ = syncStateRepo // wired in Task 12
```

- [ ] **Step 9.8: Run + commit**

```bash
go test ./internal/...
golangci-lint run
git add internal/server/ internal/cli/start.go
git commit -m "feat: wire meeting/note/secret services into UDS server"
```

---

## Task 10: CLI subcommands `cockpit meeting` and `cockpit secret`

**Files:**
- Modify: `internal/cli/client.go` (add meeting/note/secret/sync methods)
- Create: `internal/cli/meeting.go`
- Create: `internal/cli/secret.go`

- [ ] **Step 10.1: Append methods to `internal/cli/client.go`**

Append BEFORE the trailing `var ErrDaemonNotRunning ...` line:

```go
// ListMeetings lists upcoming meetings in [from, to].
func (c *Client) ListMeetings(ctx context.Context, fromUnix, toUnix int64, includeCancelled bool) ([]api.Meeting, error) {
	q := fmt.Sprintf("/api/meetings?from=%d&to=%d", fromUnix, toUnix)
	if includeCancelled {
		q += "&includeCancelled=1"
	}
	var out []api.Meeting
	if err := c.getJSON(ctx, q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// NextMeeting returns the earliest upcoming meeting or nil.
func (c *Client) NextMeeting(ctx context.Context) (*api.Meeting, error) {
	resp, err := c.do(ctx, "GET", "/api/meetings/next", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out *api.Meeting
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetSecret encrypts and stores a value under name.
func (c *Client) SetSecret(ctx context.Context, name, value string) error {
	b, _ := json.Marshal(api.SetSecretRequest{Value: value})
	resp, err := c.do(ctx, "PUT", "/api/secrets/"+name, bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// ListSecretNames returns all known secret names (no values).
func (c *Client) ListSecretNames(ctx context.Context) ([]api.Secret, error) {
	var out []api.Secret
	if err := c.getJSON(ctx, "/api/secrets", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TriggerSync forces a sync for the named source (e.g. "caldav").
func (c *Client) TriggerSync(ctx context.Context, source string) error {
	resp, err := c.do(ctx, "POST", "/api/sync/"+source, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}
```

- [ ] **Step 10.2: Create `internal/cli/meeting.go`**

```go
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var meetingCmd = &cobra.Command{
	Use:   "meeting",
	Short: "Show meetings",
}

var meetingNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the next upcoming meeting",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		next, err := c.NextMeeting(context.Background())
		if err != nil {
			return err
		}
		if next == nil {
			fmt.Println("(no upcoming meetings)")
			return nil
		}
		t := time.Unix(next.StartAt, 0).Local()
		dur := time.Until(t).Round(time.Minute)
		fmt.Printf("%s — %s (in %s)\n", t.Format("Mon 15:04"), next.Title, dur)
		if next.Location != "" {
			fmt.Printf("  @ %s\n", next.Location)
		}
		return nil
	},
}

var meetingListFlags struct {
	days int
}

var meetingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List meetings in the next N days (default 7)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		now := time.Now()
		from := now.Unix()
		to := now.AddDate(0, 0, meetingListFlags.days).Unix()
		list, err := c.ListMeetings(context.Background(), from, to, false)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no meetings in window)")
			return nil
		}
		for _, m := range list {
			t := time.Unix(m.StartAt, 0).Local()
			fmt.Printf("%s  %s\n", t.Format("Mon 02 Jan 15:04"), m.Title)
		}
		return nil
	},
}

func init() {
	meetingListCmd.Flags().IntVarP(&meetingListFlags.days, "days", "d", 7, "Window size in days")
	meetingCmd.AddCommand(meetingNextCmd, meetingListCmd)
	rootCmd.AddCommand(meetingCmd)
}
```

- [ ] **Step 10.3: Create `internal/cli/secret.go`**

```go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage encrypted secrets (CalDAV password, etc.)",
}

var secretSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Read a value from stdin and store it encrypted",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fmt.Fprintf(os.Stderr, "Enter value for %q (input is read from stdin until newline):\n", args[0])
		reader := bufio.NewReader(os.Stdin)
		raw, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		val := strings.TrimRight(raw, "\r\n")

		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.SetSecret(context.Background(), args[0], val); err != nil {
			return err
		}
		fmt.Println("ok")
		return nil
	},
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known secret names (no values)",
	RunE: func(_ *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		list, err := c.ListSecretNames(context.Background())
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no secrets)")
			return nil
		}
		for _, s := range list {
			fmt.Println(s.Name)
		}
		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretSetCmd, secretListCmd)
	rootCmd.AddCommand(secretCmd)
}
```

- [ ] **Step 10.4: Verify build + commit**

```bash
go build ./...
go test ./internal/...
golangci-lint run
git add internal/cli/
git commit -m "feat: add cockpit meeting and secret subcommands"
```

---

## Task 11: CalDAV client wrapper

**Files:**
- Create: `internal/sync/caldav/client.go`
- Create: `internal/sync/caldav/client_test.go`
- Create: `internal/testdata/caldav/single_event.ics`

The client wraps `emersion/go-webdav` (CalDAV transport) + `emersion/go-ical` (iCal parsing) behind an interface so the syncer can be unit-tested with a fake client.

- [ ] **Step 11.1: Add dependencies**

```bash
go get github.com/emersion/go-webdav
go get github.com/emersion/go-ical
go mod tidy
```

- [ ] **Step 11.2: Create `internal/testdata/caldav/single_event.ics`**

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//spk-cockpit-test//EN
CALSCALE:GREGORIAN
BEGIN:VEVENT
UID:test-event-uid-1@example.com
DTSTAMP:20260427T100000Z
DTSTART:20260427T140000Z
DTEND:20260427T150000Z
SUMMARY:Project sync
DESCRIPTION:Weekly team sync
LOCATION:Meet link
END:VEVENT
END:VCALENDAR
```

- [ ] **Step 11.3: Write `internal/sync/caldav/client.go`**

```go
// Package caldav provides a CalDAV client tailored for spk-cockpit's read-only
// sync use case (Yandex Calendar).
package caldav

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"github.com/spk/spk-cockpit/internal/api"
)

// Config holds the credentials and endpoint for a CalDAV server.
type Config struct {
	BaseURL  string // e.g. "https://caldav.yandex.ru/"
	Username string
	Password string
}

// Client is the abstraction the syncer talks to. //nolint:revive // kept short for ergonomics
type Client interface {
	// FetchEvents returns events whose start time falls in [from, to].
	// CTag is the collection ETag the server reports; pass the previously-stored
	// ctag and the client returns the new one (along with events) so the syncer
	// can short-circuit unchanged collections.
	FetchEvents(ctx context.Context, from, to time.Time, prevCTag string) (events []api.Meeting, newCTag string, unchanged bool, err error)
}

type httpClient struct {
	cfg Config
	cal *caldav.Client
}

// NewClient constructs a real CalDAV client.
func NewClient(cfg Config) (Client, error) {
	httpAuth := caldav.HTTPClientWithBasicAuth(nil, cfg.Username, cfg.Password)
	cl, err := caldav.NewClient(httpAuth, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("caldav client: %w", err)
	}
	return &httpClient{cfg: cfg, cal: cl}, nil
}

// FetchEvents implements Client.FetchEvents.
func (c *httpClient) FetchEvents(ctx context.Context, from, to time.Time, _ string) ([]api.Meeting, string, bool, error) {
	calendars, err := c.cal.FindCalendars(ctx, "")
	if err != nil {
		return nil, "", false, fmt.Errorf("find calendars: %w", err)
	}
	if len(calendars) == 0 {
		return nil, "", false, nil
	}
	// Yandex returns one or more calendar collections; sync the first (primary) for v1.
	primary := calendars[0]
	q := &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{
			Name: ical.CompCalendar,
			Comps: []caldav.CompFilter{{
				Name:  ical.CompEvent,
				Start: from,
				End:   to,
			}},
		},
	}
	objects, err := c.cal.QueryCalendar(ctx, primary.Path, q)
	if err != nil {
		return nil, "", false, fmt.Errorf("query: %w", err)
	}
	var meetings []api.Meeting
	for _, obj := range objects {
		evs := ParseICalEvents(&obj.Data, from, to)
		for _, e := range evs {
			e.ExternalETag = obj.ETag
			meetings = append(meetings, e)
		}
	}
	// We don't have a direct ctag from this query response; for v1 we return a
	// hash of the ETag set so trivial unchanged-detection works at all.
	h := sha1.New()
	for _, m := range meetings {
		_, _ = io.WriteString(h, m.ExternalUID)
		_, _ = io.WriteString(h, m.ExternalETag)
	}
	return meetings, hex.EncodeToString(h.Sum(nil)), false, nil
}

// ParseICalEvents extracts api.Meeting values from an iCal object, expanding recurrences
// inside [from, to] using the iCal iterator. It is exported for testing with fixtures.
func ParseICalEvents(cal *ical.Calendar, from, to time.Time) []api.Meeting {
	var out []api.Meeting
	if cal == nil {
		return nil
	}
	for _, comp := range cal.Children {
		if comp.Name != ical.CompEvent {
			continue
		}
		ev := ical.Event{Component: comp}
		uid := propString(ev.Component, ical.PropUID)
		if uid == "" {
			continue
		}
		summary := propString(ev.Component, ical.PropSummary)
		desc := propString(ev.Component, ical.PropDescription)
		loc := propString(ev.Component, ical.PropLocation)

		start, err := ev.DateTimeStart(time.UTC)
		if err != nil || start.IsZero() {
			continue
		}
		end, err := ev.DateTimeEnd(time.UTC)
		if err != nil || end.IsZero() {
			end = start.Add(time.Hour) // sane default
		}
		if !start.Before(to) || !end.After(from) {
			continue
		}
		out = append(out, api.Meeting{
			Source:       api.MeetingSourceCalDAV,
			ExternalUID:  uid,
			Title:        summary,
			Description:  desc,
			Location:     loc,
			StartAt:      start.Unix(),
			EndAt:        end.Unix(),
		})
	}
	return out
}

func propString(c *ical.Component, name string) string {
	if c == nil {
		return ""
	}
	p := c.Props.Get(name)
	if p == nil {
		return ""
	}
	return p.Value
}

// FakeClient is a test double — call sites set the response and the syncer calls FetchEvents.
type FakeClient struct {
	Events    []api.Meeting
	NewCTag   string
	Unchanged bool
	Err       error
	Calls     int
}

// NewFakeFromICal loads events from an in-memory iCal byte slice and returns a FakeClient.
func NewFakeFromICal(data []byte, from, to time.Time) (*FakeClient, error) {
	dec := ical.NewDecoder(bytes.NewReader(data))
	cal, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode ical: %w", err)
	}
	evs := ParseICalEvents(cal, from, to)
	for i := range evs {
		evs[i].ExternalETag = "fake-etag"
	}
	return &FakeClient{Events: evs, NewCTag: "fake-ctag"}, nil
}

// FetchEvents implements Client.FetchEvents.
func (f *FakeClient) FetchEvents(_ context.Context, _, _ time.Time, _ string) ([]api.Meeting, string, bool, error) {
	f.Calls++
	if f.Err != nil {
		return nil, "", false, f.Err
	}
	return f.Events, f.NewCTag, f.Unchanged, nil
}
```

- [ ] **Step 11.4: Write `internal/sync/caldav/client_test.go`**

```go
package caldav

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
)

func TestParseICalEvents_SingleEvent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "caldav", "single_event.ics"))
	require.NoError(t, err)

	from := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)

	fc, err := NewFakeFromICal(data, from, to)
	require.NoError(t, err)
	require.Len(t, fc.Events, 1)
	e := fc.Events[0]
	require.Equal(t, api.MeetingSourceCalDAV, e.Source)
	require.Equal(t, "test-event-uid-1@example.com", e.ExternalUID)
	require.Equal(t, "Project sync", e.Title)
	require.Equal(t, "Meet link", e.Location)

	want := time.Date(2026, 4, 27, 14, 0, 0, 0, time.UTC).Unix()
	require.Equal(t, want, e.StartAt)
}

func TestFakeClient_FetchEvents_ReturnsConfigured(t *testing.T) {
	fc := &FakeClient{
		Events:  []api.Meeting{{Source: api.MeetingSourceCalDAV, ExternalUID: "u", Title: "T", StartAt: 100, EndAt: 200}},
		NewCTag: "ctag-1",
	}
	out, ctag, unchanged, err := fc.FetchEvents(context.Background(), time.Time{}, time.Time{}, "")
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "ctag-1", ctag)
	require.False(t, unchanged)
	require.Equal(t, 1, fc.Calls)
}
```

- [ ] **Step 11.5: Run + commit**

```bash
go test ./internal/sync/caldav/... -v
golangci-lint run
git add internal/sync/caldav/ internal/testdata/caldav/single_event.ics go.mod go.sum
git commit -m "feat: add CalDAV client wrapper with iCal parser"
```

Both tests must PASS.

---

## Task 12: CalDAV syncer worker

**Files:**
- Create: `internal/sync/caldav/syncer.go`
- Create: `internal/sync/caldav/syncer_test.go`

- [ ] **Step 12.1: Write `internal/sync/caldav/syncer.go`**

```go
package caldav

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// SourceName is the sync_state key for CalDAV.
const SourceName = "caldav"

// Syncer is a periodic CalDAV worker that upserts meetings from Yandex.
type Syncer struct {
	client    Client
	meetings  *meeting.Service
	state     meeting.SyncStateRepo
	clock     clock.Clock
	logger    *slog.Logger
	bus       eventPublisher
	interval  time.Duration
	rangeBack time.Duration
	rangeFwd  time.Duration

	mu       sync.Mutex
	trigger  chan struct{}
	running  bool
	lastErr  string
	lastOkAt *int64
}

type eventPublisher interface {
	Publish(api.Event)
}

// Config configures a Syncer.
type Config struct {
	Client       Client
	Meetings     *meeting.Service
	State        meeting.SyncStateRepo
	Clock        clock.Clock
	Logger       *slog.Logger
	Bus          eventPublisher
	Interval     time.Duration // default 5m
	RangeBack    time.Duration // how far back to fetch; default 7d
	RangeForward time.Duration // how far forward to fetch; default 30d
}

// NewSyncer constructs a Syncer.
func NewSyncer(cfg Config) *Syncer {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.RangeBack == 0 {
		cfg.RangeBack = 7 * 24 * time.Hour
	}
	if cfg.RangeForward == 0 {
		cfg.RangeForward = 30 * 24 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Syncer{
		client:    cfg.Client,
		meetings:  cfg.Meetings,
		state:     cfg.State,
		clock:     cfg.Clock,
		logger:    cfg.Logger,
		bus:       cfg.Bus,
		interval:  cfg.Interval,
		rangeBack: cfg.RangeBack,
		rangeFwd:  cfg.RangeForward,
		trigger:   make(chan struct{}, 1),
	}
}

// Run blocks until ctx is done.
func (s *Syncer) Run(ctx context.Context) {
	tick := time.NewTicker(s.interval)
	defer tick.Stop()
	// Run once immediately.
	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.runOnce(ctx)
		case <-s.trigger:
			s.runOnce(ctx)
		}
	}
}

// TriggerNow asks the syncer to run immediately (non-blocking).
func (s *Syncer) TriggerNow(source string) error {
	if source != SourceName {
		return fmt.Errorf("unknown source %q", source)
	}
	select {
	case s.trigger <- struct{}{}:
	default:
	}
	return nil
}

// Status reports the syncer's current state.
func (s *Syncer) Status() []api.SyncStateEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := api.SyncStateEntry{Source: SourceName, LastErr: s.lastErr, LastOkAt: s.lastOkAt}
	return []api.SyncStateEntry{entry}
}

func (s *Syncer) runOnce(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	now := s.clock.Now()
	from := now.Add(-s.rangeBack)
	to := now.Add(s.rangeFwd)

	prev, _ := s.state.Get(ctx, SourceName)
	events, newCTag, unchanged, err := s.client.FetchEvents(ctx, from, to, prev.Cursor)
	if err != nil {
		s.recordErr(ctx, err)
		return
	}
	if unchanged {
		s.recordOK(ctx, prev.Cursor)
		return
	}

	// Track which UIDs we saw — anything previously synced but now missing → cancelled.
	seen := map[string]bool{}
	for _, ev := range events {
		seen[ev.ExternalUID] = true
		if _, err := s.meetings.UpsertFromCalDAV(ctx, ev); err != nil {
			s.logger.Warn("upsert meeting failed", "uid", ev.ExternalUID, "err", err)
		}
	}
	// Pull existing CalDAV meetings in the same window; mark missing ones cancelled.
	existing, err := s.meetings.List(ctx, meeting.MeetingFilter{FromUnix: from.Unix(), ToUnix: to.Unix(), IncludeDone: true})
	if err == nil {
		for _, m := range existing {
			if m.Source != api.MeetingSourceCalDAV {
				continue
			}
			if !seen[m.ExternalUID] && !m.Cancelled {
				if err := s.meetings.MarkExternalCancelled(ctx, m.ExternalUID); err != nil {
					s.logger.Warn("cancel meeting failed", "uid", m.ExternalUID, "err", err)
				}
			}
		}
	}

	s.recordOK(ctx, newCTag)
}

func (s *Syncer) recordErr(ctx context.Context, err error) {
	s.mu.Lock()
	s.lastErr = err.Error()
	s.mu.Unlock()
	_ = s.state.Save(ctx, api.SyncStateEntry{Source: SourceName, LastErr: err.Error()})
	if s.bus != nil {
		s.bus.Publish(api.Event{
			Type: api.EventSyncStateChanged,
			Data: api.SyncStateChangedData{Source: SourceName, Status: "failed", LastErr: err.Error()},
		})
	}
	s.logger.Warn("caldav sync failed", "err", err)
}

func (s *Syncer) recordOK(ctx context.Context, cursor string) {
	now := s.clock.Now().Unix()
	s.mu.Lock()
	s.lastErr = ""
	s.lastOkAt = &now
	s.mu.Unlock()
	_ = s.state.Save(ctx, api.SyncStateEntry{Source: SourceName, Cursor: cursor, LastOkAt: &now})
	if s.bus != nil {
		s.bus.Publish(api.Event{
			Type: api.EventSyncStateChanged,
			Data: api.SyncStateChangedData{Source: SourceName, Status: "ok"},
		})
	}
}
```

- [ ] **Step 12.2: Write `internal/sync/caldav/syncer_test.go`**

```go
package caldav_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	meetingfake "github.com/spk/spk-cockpit/internal/meeting/fakerepo"
	"github.com/spk/spk-cockpit/internal/sync/caldav"
)

type stateRepoFake struct {
	entry api.SyncStateEntry
}

func (f *stateRepoFake) Get(_ context.Context, source string) (api.SyncStateEntry, error) {
	if f.entry.Source == source {
		return f.entry, nil
	}
	return api.SyncStateEntry{Source: source}, nil
}
func (f *stateRepoFake) Save(_ context.Context, e api.SyncStateEntry) error { f.entry = e; return nil }
func (f *stateRepoFake) List(_ context.Context) ([]api.SyncStateEntry, error) {
	return []api.SyncStateEntry{f.entry}, nil
}

func TestSyncer_RunOnce_UpsertsEventsAndMarksMissingCancelled(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	mrepo := meetingfake.NewMeeting()
	c := clock.NewFake(t0)
	msvc := meeting.NewService(mrepo, c, nil)

	// Pre-seed an old CalDAV meeting that won't appear in the next sync (should be cancelled).
	require.NoError(t, mrepo.Create(context.Background(), api.Meeting{
		ID: "old", Source: api.MeetingSourceCalDAV, ExternalUID: "old-uid",
		Title: "Old", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(2 * time.Hour).Unix(),
		CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}))

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "caldav", "single_event.ics"))
	require.NoError(t, err)
	fc, err := caldav.NewFakeFromICal(data, t0.Add(-7*24*time.Hour), t0.Add(30*24*time.Hour))
	require.NoError(t, err)

	state := &stateRepoFake{}
	s := caldav.NewSyncer(caldav.Config{
		Client: fc, Meetings: msvc, State: state, Clock: c, Interval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Run() will runOnce then return.
	s.Run(ctx)

	// Old should be cancelled, new should exist.
	got, err := mrepo.Get(context.Background(), "old")
	require.NoError(t, err)
	require.True(t, got.Cancelled)

	listed, err := msvc.List(context.Background(), meeting.MeetingFilter{FromUnix: t0.Add(-time.Hour).Unix(), ToUnix: t0.Add(7 * 24 * time.Hour).Unix()})
	require.NoError(t, err)
	var found bool
	for _, m := range listed {
		if m.ExternalUID == "test-event-uid-1@example.com" {
			found = true
		}
	}
	require.True(t, found)
}

func TestSyncer_RunOnce_PropagatesErrorToState(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	mrepo := meetingfake.NewMeeting()
	c := clock.NewFake(t0)
	msvc := meeting.NewService(mrepo, c, nil)

	fc := &caldav.FakeClient{Err: errStub("boom")}
	state := &stateRepoFake{}
	s := caldav.NewSyncer(caldav.Config{
		Client: fc, Meetings: msvc, State: state, Clock: c, Interval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)

	require.Equal(t, "boom", state.entry.LastErr)
	require.Nil(t, state.entry.LastOkAt)
	st := s.Status()
	require.Equal(t, "boom", st[0].LastErr)
}

type errStub string

func (e errStub) Error() string { return string(e) }
```

- [ ] **Step 12.3: Wire syncer into `runStart`**

In `internal/cli/start.go`, after the master-key block, BEFORE `srv.Deps().Sync = ...`:

```go
	caldavCfg := loadCaldavConfig(secretSvc, st.DB) // helper defined below
	var caldavSyncer *caldav.Syncer
	if caldavCfg != nil {
		client, err := caldav.NewClient(*caldavCfg)
		if err != nil {
			logger.Warn("caldav client init failed; sync disabled", "err", err)
		} else {
			caldavSyncer = caldav.NewSyncer(caldav.Config{
				Client:   client,
				Meetings: meetingSvc,
				State:    syncStateRepo,
				Clock:    clock.Real(),
				Logger:   logger,
				Bus:      bus,
			})
		}
	}
	if caldavSyncer != nil {
		go caldavSyncer.Run(ctx)
		srv.Deps().Sync = caldavSyncer
	}
```

Add helper `loadCaldavConfig` at the bottom of `start.go`:

```go
func loadCaldavConfig(secrets *secret.Service, db *sql.DB) *caldav.Config {
	ctx := context.Background()
	url, _, err := store.NewKvRepo(db).Get(ctx, "caldav.url")
	if err != nil || url == "" {
		return nil
	}
	username, _, err := store.NewKvRepo(db).Get(ctx, "caldav.username")
	if err != nil || username == "" {
		return nil
	}
	password, err := secrets.Get(ctx, "yandex_caldav")
	if err != nil {
		return nil
	}
	return &caldav.Config{BaseURL: url, Username: username, Password: password}
}
```

(`caldav.url` and `caldav.username` are stored in the existing `kv` table by the Settings UI in Task 17. Until Settings are wired, the syncer simply doesn't start — that's the desired behaviour.)

Add imports:

```go
"database/sql"
"github.com/spk/spk-cockpit/internal/sync/caldav"
```

Remove the `_ = syncStateRepo` line from Task 9 (it's now used).

- [ ] **Step 12.4: Run + commit**

```bash
go test ./internal/sync/caldav/...
go test ./internal/...
golangci-lint run
git add internal/sync/caldav/syncer.go internal/sync/caldav/syncer_test.go internal/cli/start.go
git commit -m "feat: add CalDAV syncer worker"
```

---

## Task 13: DBus notification backend

**Files:**
- Create: `internal/notify/notify.go`
- Create: `internal/notify/notify_dbus.go`
- Create: `internal/notify/notify_noop.go`

- [ ] **Step 13.1: Add dependency**

```bash
go get github.com/godbus/dbus/v5
go mod tidy
```

- [ ] **Step 13.2: Write `internal/notify/notify.go`**

```go
// Package notify abstracts system notifications behind a Notifier interface.
// Linux uses DBus (org.freedesktop.Notifications); other platforms use no-op.
package notify

// Notifier sends a system notification.
type Notifier interface {
	// Notify shows a notification with title and body. Returns nil on success.
	// Implementations must NOT block; failures are logged and reported but never panic.
	Notify(title, body string) error

	// Close releases any resources (e.g. DBus connection).
	Close() error
}
```

- [ ] **Step 13.3: Write `internal/notify/notify_dbus.go`**

```go
//go:build linux

package notify

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	dbusObjectPath = "/org/freedesktop/Notifications"
	dbusInterface  = "org.freedesktop.Notifications"
	dbusName       = "org.freedesktop.Notifications"
)

// DBusNotifier sends notifications via libnotify's DBus interface.
type DBusNotifier struct {
	conn *dbus.Conn
}

// NewDBus connects to the session bus.
func NewDBus() (*DBusNotifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus session: %w", err)
	}
	return &DBusNotifier{conn: conn}, nil
}

// Notify sends a notification.
func (d *DBusNotifier) Notify(title, body string) error {
	if d == nil || d.conn == nil {
		return errors.New("notifier not initialized")
	}
	obj := d.conn.Object(dbusName, dbusObjectPath)
	call := obj.Call(
		dbusInterface+".Notify",
		0,
		"spk-cockpit", // app_name
		uint32(0),     // replaces_id
		"",            // app_icon
		title,
		body,
		[]string{},        // actions
		map[string]any{},  // hints
		int32(-1),         // expire_timeout (-1 = default)
	)
	if call.Err != nil {
		return fmt.Errorf("notify call: %w", call.Err)
	}
	return nil
}

// Close closes the DBus connection.
func (d *DBusNotifier) Close() error {
	if d != nil && d.conn != nil {
		return d.conn.Close()
	}
	return nil
}
```

- [ ] **Step 13.4: Write `internal/notify/notify_noop.go`**

```go
package notify

import "log/slog"

// NoopNotifier logs notifications instead of dispatching them. Used when DBus is unavailable.
type NoopNotifier struct {
	Logger *slog.Logger
}

// NewNoop returns a NoopNotifier.
func NewNoop(logger *slog.Logger) *NoopNotifier { return &NoopNotifier{Logger: logger} }

// Notify logs the event.
func (n *NoopNotifier) Notify(title, body string) error {
	if n.Logger != nil {
		n.Logger.Info("(noop notify)", "title", title, "body", body)
	}
	return nil
}

// Close is a no-op.
func (n *NoopNotifier) Close() error { return nil }
```

- [ ] **Step 13.5: Verify build + commit**

```bash
go build ./internal/notify/...
golangci-lint run
git add internal/notify/notify.go internal/notify/notify_dbus.go internal/notify/notify_noop.go go.mod go.sum
git commit -m "feat: add DBus notification backend with noop fallback"
```

---

## Task 14: NotificationScheduler worker

**Files:**
- Create: `internal/notify/scheduler.go`
- Create: `internal/notify/scheduler_test.go`

- [ ] **Step 14.1: Write `internal/notify/scheduler_test.go`**

```go
package notify_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/meeting/fakerepo"
	"github.com/spk/spk-cockpit/internal/notify"
)

type captureNotifier struct {
	mu   sync.Mutex
	rows []string
}

func (c *captureNotifier) Notify(title, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, title+"|"+body)
	return nil
}
func (c *captureNotifier) Close() error { return nil }
func (c *captureNotifier) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.rows))
	copy(out, c.rows)
	return out
}

func TestScheduler_FiresOnceWhenWindowReached(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := clock.NewFake(t0)
	mrepo := fakerepo.NewMeeting()
	msvc := meeting.NewService(mrepo, c, nil)

	notifyMin := 5
	require.NoError(t, mrepo.Create(context.Background(), api.Meeting{
		ID: "m-1", Source: api.MeetingSourceManual, Title: "Standup",
		StartAt: t0.Add(4 * time.Minute).Unix(), EndAt: t0.Add(30 * time.Minute).Unix(),
		NotifyMin: &notifyMin, CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}))

	cap := &captureNotifier{}
	sch := notify.NewScheduler(notify.SchedulerConfig{
		Meetings:         msvc,
		Notifier:         cap,
		Clock:            c,
		DefaultNotifyMin: 5,
		Tick:             10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go sch.Run(ctx)

	// Wait until the scheduler fires (poll up to 1s).
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(cap.snapshot()) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	got := cap.snapshot()
	require.Len(t, got, 1)
	require.Contains(t, got[0], "Standup")

	// Second iteration must not double-fire.
	got2 := cap.snapshot()
	require.Equal(t, got, got2)
}
```

- [ ] **Step 14.2: Write `internal/notify/scheduler.go`**

```go
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// Scheduler scans for meetings due for notification and fires them via Notifier.
type Scheduler struct {
	meetings         *meeting.Service
	notifier         Notifier
	clock            clock.Clock
	logger           *slog.Logger
	defaultNotifyMin int
	tick             time.Duration
}

// SchedulerConfig configures a Scheduler.
type SchedulerConfig struct {
	Meetings         *meeting.Service
	Notifier         Notifier
	Clock            clock.Clock
	Logger           *slog.Logger
	DefaultNotifyMin int           // default 5
	Tick             time.Duration // default 30s
}

// NewScheduler constructs a Scheduler.
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.DefaultNotifyMin == 0 {
		cfg.DefaultNotifyMin = 5
	}
	if cfg.Tick == 0 {
		cfg.Tick = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Scheduler{
		meetings:         cfg.Meetings,
		notifier:         cfg.Notifier,
		clock:            cfg.Clock,
		logger:           cfg.Logger,
		defaultNotifyMin: cfg.DefaultNotifyMin,
		tick:             cfg.Tick,
	}
}

// Run blocks until ctx is done, scanning every tick.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	s.scanOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.scanOnce(ctx)
		}
	}
}

func (s *Scheduler) scanOnce(ctx context.Context) {
	pending, err := s.meetings.PendingNotification(ctx, s.defaultNotifyMin)
	if err != nil {
		s.logger.Warn("pending notify failed", "err", err)
		return
	}
	for _, m := range pending {
		body := fmt.Sprintf("Starts in %s", time.Until(time.Unix(m.StartAt, 0)).Round(time.Minute))
		if err := s.notifier.Notify(m.Title, body); err != nil {
			s.logger.Warn("notify failed", "id", m.ID, "err", err)
			continue
		}
		if err := s.meetings.MarkNotified(ctx, m.ID); err != nil {
			s.logger.Warn("mark notified failed", "id", m.ID, "err", err)
		}
	}
}
```

- [ ] **Step 14.3: Wire into `runStart`**

In `internal/cli/start.go`, after the syncer wiring, add:

```go
	var notifier notify.Notifier
	dbusN, err := notify.NewDBus()
	if err != nil {
		logger.Warn("dbus init failed; using noop notifier", "err", err)
		notifier = notify.NewNoop(logger)
	} else {
		notifier = dbusN
	}
	defer func() { _ = notifier.Close() }()

	defaultNotifyMin := 5
	if v, ok, _ := store.NewKvRepo(st.DB).Get(ctx, "meeting.default_notify_min"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			defaultNotifyMin = n
		}
	}

	scheduler := notify.NewScheduler(notify.SchedulerConfig{
		Meetings:         meetingSvc,
		Notifier:         notifier,
		Clock:            clock.Real(),
		Logger:           logger,
		DefaultNotifyMin: defaultNotifyMin,
	})
	go scheduler.Run(ctx)
```

Add imports:

```go
"strconv"
"github.com/spk/spk-cockpit/internal/notify"
```

- [ ] **Step 14.4: Run + commit**

```bash
go test ./internal/notify/... -v
go test ./internal/...
golangci-lint run
git add internal/notify/scheduler.go internal/notify/scheduler_test.go internal/cli/start.go
git commit -m "feat: add NotificationScheduler worker"
```

The scheduler test must PASS within ~1 second.

---

## Task 15: Tray tooltip — surface next meeting and notifications

**Files:**
- Modify: `internal/tray/tooltip.go`

The existing tooltip subscriber tracks the active timer. Extend it to also report the next meeting. Tooltip priority: active timer > next meeting (within 24h) > default.

- [ ] **Step 15.1: Extend `Subscriber` in `internal/tray/tooltip.go`**

Replace the file with:

```go
package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

// Subscriber polls the event bus and updates the tray tooltip on timer / meeting events.
type Subscriber struct {
	bus       EventSource
	tray      Backend
	timer     activeTimer
	nextMeet  nextMeeting
	mtgFetch  func() *api.Meeting
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

type nextMeeting struct {
	id      string
	title   string
	startAt int64
}

// NewSubscriber wires the bus and tray. mtgFetch may be nil; when present it's
// invoked on each tick (and after MeetingUpserted/Deleted/NotificationFired) to
// refresh the cached next-meeting summary.
func NewSubscriber(bus EventSource, t Backend, mtgFetch func() *api.Meeting) *Subscriber {
	return &Subscriber{bus: bus, tray: t, mtgFetch: mtgFetch}
}

// Run subscribes and updates the tooltip until ctx is done.
func (s *Subscriber) Run(ctx context.Context) {
	ch := s.bus.Subscribe(64)
	defer s.bus.Unsubscribe(ch)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	s.refreshMeeting()
	s.refresh()

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
			s.refreshMeeting()
			s.refresh()
		}
	}
}

func (s *Subscriber) handleEvent(e api.Event) {
	switch e.Type {
	case api.EventTimerStarted:
		if d, ok := e.Data.(api.TimerStartedData); ok {
			s.timer = activeTimer{todoID: d.TodoID, startedAt: d.StartedAt}
			s.refresh()
		}
	case api.EventTimerStopped:
		s.timer = activeTimer{}
		s.refresh()
	case api.EventMeetingUpserted, api.EventMeetingDeleted, api.EventMeetingNotificationFired:
		s.refreshMeeting()
		s.refresh()
	}
}

func (s *Subscriber) refreshMeeting() {
	if s.mtgFetch == nil {
		s.nextMeet = nextMeeting{}
		return
	}
	m := s.mtgFetch()
	if m == nil {
		s.nextMeet = nextMeeting{}
		return
	}
	s.nextMeet = nextMeeting{id: m.ID, title: m.Title, startAt: m.StartAt}
}

func (s *Subscriber) refresh() {
	switch {
	case s.timer.todoID != "":
		elapsed := time.Since(time.Unix(s.timer.startedAt, 0)).Round(time.Second)
		s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • %s on %s", elapsed, shortTodoID(s.timer.todoID)))
	case s.nextMeet.id != "" && time.Until(time.Unix(s.nextMeet.startAt, 0)) < 24*time.Hour:
		until := time.Until(time.Unix(s.nextMeet.startAt, 0)).Round(time.Minute)
		s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • next: %s in %s", s.nextMeet.title, until))
	default:
		s.tray.SetTooltip("spk-cockpit")
	}
}

func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}
```

- [ ] **Step 15.2: Update `runStart` to pass mtgFetch**

In `internal/cli/start.go`, replace:

```go
go tray.NewSubscriber(bus, trayBackend).Run(ctx)
```

with:

```go
mtgFetch := func() *api.Meeting {
	m, err := meetingSvc.Next(context.Background())
	if err != nil {
		return nil
	}
	return m
}
go tray.NewSubscriber(bus, trayBackend, mtgFetch).Run(ctx)
```

Add import `"github.com/spk/spk-cockpit/internal/api"` if not already present.

- [ ] **Step 15.3: Build + commit**

```bash
make build
go test ./internal/...
golangci-lint run
git add internal/tray/tooltip.go internal/cli/start.go
git commit -m "feat: tray tooltip surfaces next meeting"
```

---

## Task 16: Web — types, API client, store extensions

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/store.ts`

- [ ] **Step 16.1: Append to `web/src/lib/types.ts`**

```ts
export type MeetingSource = "manual" | "caldav";

export interface Meeting {
  id: string;
  source: MeetingSource;
  externalUid?: string;
  externalEtag?: string;
  title: string;
  description: string;
  location: string;
  startAt: number;
  endAt: number;
  notifyMin?: number;
  notifiedAt?: number;
  cancelled: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface CreateMeetingRequest {
  title: string;
  description?: string;
  location?: string;
  startAt: number;
  endAt: number;
  notifyMin?: number;
}

export interface UpdateMeetingRequest {
  title?: string;
  description?: string;
  location?: string;
  startAt?: number;
  endAt?: number;
  notifyMin?: number;
}

export interface Note {
  id: string;
  meetingId?: string;
  todoId?: string;
  body: string;
  createdAt: number;
  updatedAt: number;
}

export interface UpsertNoteRequest {
  meetingId?: string;
  todoId?: string;
  body: string;
}

export interface Secret {
  name: string;
  updatedAt: number;
}

export interface SyncStateEntry {
  source: string;
  cursor: string;
  lastOkAt?: number;
  lastErr?: string;
}
```

- [ ] **Step 16.2: Update `web/src/lib/api.ts`**

Replace import line:

```ts
import type {
  Todo, Tag, CreateTodoRequest, UpdateTodoRequest,
  TimerSession, TodoTimeTotal,
  Meeting, CreateMeetingRequest, UpdateMeetingRequest,
  Note, UpsertNoteRequest,
  Secret, SyncStateEntry,
} from "./types";
```

Inside the `api` object, append (preserving existing methods):

```ts
  listMeetings: (fromUnix: number, toUnix: number, includeCancelled = false) =>
    request<Meeting[]>(
      `/api/meetings?from=${fromUnix}&to=${toUnix}${includeCancelled ? "&includeCancelled=1" : ""}`,
    ),
  nextMeeting: () => request<Meeting | null>("/api/meetings/next"),
  createMeeting: (req: CreateMeetingRequest) =>
    request<Meeting>("/api/meetings", { method: "POST", body: JSON.stringify(req) }),
  updateMeeting: (id: string, req: UpdateMeetingRequest) =>
    request<Meeting>(`/api/meetings/${id}`, { method: "PATCH", body: JSON.stringify(req) }),
  deleteMeeting: (id: string) =>
    request<void>(`/api/meetings/${id}`, { method: "DELETE" }),
  meetingNote: (id: string) => request<Note | null>(`/api/meetings/${id}/note`),
  upsertNote: (req: UpsertNoteRequest) =>
    request<Note>("/api/notes", { method: "PUT", body: JSON.stringify(req) }),
  deleteNote: (id: string) =>
    request<void>(`/api/notes/${id}`, { method: "DELETE" }),
  todoNote: (id: string) => request<Note | null>(`/api/todos/${id}/note`),

  listSecrets: () => request<Secret[]>("/api/secrets"),
  setSecret: (name: string, value: string) =>
    request<void>(`/api/secrets/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
  deleteSecret: (name: string) =>
    request<void>(`/api/secrets/${encodeURIComponent(name)}`, { method: "DELETE" }),

  syncStatus: () => request<SyncStateEntry[]>("/api/sync"),
  triggerSync: (source: string) =>
    request<void>(`/api/sync/${encodeURIComponent(source)}`, { method: "POST" }),

  getKv: (key: string) => request<{ key: string; value: string | null }>(`/api/kv/${encodeURIComponent(key)}`),
  setKv: (key: string, value: string) =>
    request<void>(`/api/kv/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
```

Note: the `getKv`/`setKv` API endpoints are NEW; they expose the existing `KvRepo` for the Settings UI. Add them to the server in Step 16.3.

- [ ] **Step 16.3: Add KV handlers and routes**

Append to `internal/server/secret_handler.go` (or create a small new file `internal/server/kv_handler.go` — your choice; keep one file per logical domain):

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/spk/spk-cockpit/internal/todo"
)

// Deps already includes Bus + others; add a KvRepo field.
// In server.go, extend Deps with: Kv todo.KvRepo  (todo.KvRepo is the KV interface from Phase 1).

func handleGetKv(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		v, ok, err := d.Kv.Get(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "kv.get_failed", err.Error())
			return
		}
		var val *string
		if ok {
			val = &v
		}
		writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": val})
	}
}

func handleSetKv(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := d.Kv.Set(r.Context(), key, body.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "kv.set_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

var _ = todo.ErrNotFound // keep import; ErrNotFound used only when adding a delete endpoint later.
```

Update `Deps` in `internal/server/server.go`:

```go
type Deps struct {
	Todos    *todo.Service
	Tags     todo.TagRepo
	Bus      *eventbus.Bus
	Timer    *timer.Service
	Meetings *meeting.Service
	Notes    *note.Service
	Secrets  *secret.Service
	Sync     SyncTrigger
	Kv       todo.KvRepo
}
```

Register routes in `routes.go` (after the secrets routes):

```go
	mux.HandleFunc("GET /api/kv/{key}", handleGetKv(d))
	mux.HandleFunc("PUT /api/kv/{key}", handleSetKv(d))
```

Wire in `runStart`:

```go
	srv.Deps().Kv = store.NewKvRepo(st.DB)
```

- [ ] **Step 16.4: Update `web/src/lib/store.ts`**

Replace with extended state:

```ts
import { create } from "zustand";
import { api } from "./api";
import type { Todo, ApiEvent, TimerSession, Meeting, SyncStateEntry } from "./types";

interface AppState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;
  activeTimer: TimerSession | null;

  meetings: Meeting[];
  meetingsLoading: boolean;

  syncStates: SyncStateEntry[];

  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  loadActiveTimer: () => Promise<void>;
  loadMeetings: (fromUnix: number, toUnix: number) => Promise<void>;
  loadSyncStatus: () => Promise<void>;
  applyEvent: (e: ApiEvent) => void;
}

export const useTodoStore = create<AppState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  activeTimer: null,
  meetings: [],
  meetingsLoading: false,
  syncStates: [],

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
  async loadMeetings(fromUnix, toUnix) {
    set({ meetingsLoading: true });
    try {
      const list = await api.listMeetings(fromUnix, toUnix);
      set({ meetings: list, meetingsLoading: false });
    } catch {
      set({ meetingsLoading: false });
    }
  },
  async loadSyncStatus() {
    try {
      const list = await api.syncStatus();
      set({ syncStates: list });
    } catch {
      // ignore
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
        activeTimer: { id: d.sessionId, todoId: d.todoId, startedAt: d.startedAt, source: "manual" },
      });
    } else if (e.type === "timer.stopped") {
      set({ activeTimer: null });
    } else if (e.type === "meeting.upserted") {
      const { meeting } = e.data as { meeting: Meeting };
      const others = get().meetings.filter((m) => m.id !== meeting.id);
      set({ meetings: [...others, meeting].sort((a, b) => a.startAt - b.startAt) });
    } else if (e.type === "meeting.deleted") {
      const { meetingId } = e.data as { meetingId: string };
      set({ meetings: get().meetings.filter((m) => m.id !== meetingId) });
    } else if (e.type === "sync.state_changed") {
      void get().loadSyncStatus();
    }
  },
}));
```

- [ ] **Step 16.5: Build + commit**

```bash
cd web && pnpm build && pnpm lint
cd /home/spk/IdeaProjects/spk-task-manager
go test ./internal/...
golangci-lint run
git add web/src/lib/ internal/server/ internal/cli/start.go
git commit -m "feat: web types/api/store + KV API for meetings/notes/secrets"
```

---

## Task 17: Calendar page + meeting card

**Files:**
- Create: `web/src/components/MeetingCard.tsx`
- Create: `web/src/pages/Calendar.tsx`
- Modify: `web/src/App.tsx` (add /calendar route + nav link)

- [ ] **Step 17.1: Create `web/src/components/MeetingCard.tsx`**

```tsx
import { Calendar, MapPin, Bell } from "lucide-react";
import type { Meeting } from "../lib/types";

export interface MeetingCardProps {
  meeting: Meeting;
  onClick?: (m: Meeting) => void;
  selected?: boolean;
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function relTime(unix: number): string {
  const ms = unix * 1000 - Date.now();
  if (ms < 0) return "started";
  const min = Math.round(ms / 60000);
  if (min < 60) return `in ${min}m`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `in ${hr}h`;
  const day = Math.round(hr / 24);
  return `in ${day}d`;
}

export function MeetingCard({ meeting, onClick, selected }: MeetingCardProps) {
  const cls = `flex flex-col gap-1 p-3 rounded border cursor-pointer ${
    selected ? "bg-bgsub border-accent" : "bg-bg border-bgmute hover:border-fgmute"
  } ${meeting.cancelled ? "opacity-50 line-through" : ""}`;
  return (
    <div className={cls} onClick={() => onClick?.(meeting)}>
      <div className="flex items-center justify-between">
        <span className="font-medium">{meeting.title}</span>
        <span className="text-fgmute text-xs">{relTime(meeting.startAt)}</span>
      </div>
      <div className="flex items-center gap-3 text-fgmute text-xs">
        <span className="inline-flex items-center gap-1">
          <Calendar size={12} />
          {formatTime(meeting.startAt)} – {formatTime(meeting.endAt)}
        </span>
        {meeting.location && (
          <span className="inline-flex items-center gap-1">
            <MapPin size={12} />
            {meeting.location}
          </span>
        )}
        {meeting.notifyMin !== undefined && (
          <span className="inline-flex items-center gap-1">
            <Bell size={12} />
            {meeting.notifyMin}m
          </span>
        )}
        {meeting.source === "caldav" && <span className="text-low text-[10px] uppercase">caldav</span>}
      </div>
    </div>
  );
}
```

- [ ] **Step 17.2: Create `web/src/pages/Calendar.tsx`**

```tsx
import { useEffect, useMemo, useState } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { MeetingCard } from "../components/MeetingCard";
import type { Meeting } from "../lib/types";

function startOfDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

export function Calendar() {
  const { meetings, meetingsLoading, loadMeetings } = useTodoStore();
  const [selected, setSelected] = useState<Meeting | null>(null);
  const [noteBody, setNoteBody] = useState("");
  const [savingNote, setSavingNote] = useState(false);

  useEffect(() => {
    const now = new Date();
    const from = Math.floor(startOfDay(now).getTime() / 1000) - 24 * 3600;
    const to = Math.floor(startOfDay(now).getTime() / 1000) + 30 * 24 * 3600;
    void loadMeetings(from, to);
  }, [loadMeetings]);

  useEffect(() => {
    if (!selected) {
      setNoteBody("");
      return;
    }
    void api.meetingNote(selected.id).then((n) => setNoteBody(n?.body ?? ""));
  }, [selected]);

  const sections = useMemo(() => {
    const today = startOfDay(new Date()).getTime() / 1000;
    const tomorrow = today + 24 * 3600;
    const dayAfter = today + 2 * 24 * 3600;
    return [
      { label: "Today", items: meetings.filter((m) => m.startAt >= today && m.startAt < tomorrow) },
      { label: "Tomorrow", items: meetings.filter((m) => m.startAt >= tomorrow && m.startAt < dayAfter) },
      { label: "Later", items: meetings.filter((m) => m.startAt >= dayAfter) },
    ];
  }, [meetings]);

  async function saveNote() {
    if (!selected) return;
    setSavingNote(true);
    try {
      await api.upsertNote({ meetingId: selected.id, body: noteBody });
    } finally {
      setSavingNote(false);
    }
  }

  return (
    <div className="flex gap-6 h-full">
      <div className="flex-1 flex flex-col gap-4 max-w-2xl">
        <h2 className="text-xl font-semibold">Calendar</h2>
        {meetingsLoading && <div className="text-fgmute">loading…</div>}
        {!meetingsLoading && meetings.length === 0 && (
          <div className="text-fgmute py-8 text-center">no meetings in window</div>
        )}
        {sections.map((section) =>
          section.items.length > 0 ? (
            <section key={section.label} className="flex flex-col gap-2">
              <h3 className="text-fgmute text-xs uppercase">{section.label}</h3>
              {section.items.map((m) => (
                <MeetingCard
                  key={m.id}
                  meeting={m}
                  selected={selected?.id === m.id}
                  onClick={setSelected}
                />
              ))}
            </section>
          ) : null,
        )}
      </div>

      {selected && (
        <aside className="w-96 flex flex-col gap-3 border-l border-bgmute pl-4">
          <h3 className="font-semibold">{selected.title}</h3>
          {selected.description && <p className="text-fgmute text-sm">{selected.description}</p>}
          <div className="text-fgmute text-xs">
            {new Date(selected.startAt * 1000).toLocaleString()}
          </div>
          <textarea
            value={noteBody}
            onChange={(e) => setNoteBody(e.target.value)}
            placeholder="Notes (markdown)"
            className="flex-1 min-h-64 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent"
          />
          <div className="flex gap-2">
            <button
              onClick={saveNote}
              disabled={savingNote}
              className="px-3 py-1 bg-accent text-bg rounded text-sm"
            >
              {savingNote ? "saving…" : "save note"}
            </button>
            <button
              onClick={() => setSelected(null)}
              className="px-3 py-1 text-fgmute hover:text-fg text-sm"
            >
              close
            </button>
          </div>
        </aside>
      )}
    </div>
  );
}
```

- [ ] **Step 17.3: Update `web/src/App.tsx`**

Replace with the routes extended:

```tsx
import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import { Todos } from "./pages/Todos";
import { Popover } from "./pages/Popover";
import { Calendar } from "./pages/Calendar";
import { Settings } from "./pages/Settings";

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
  const navItem = (to: string, label: string) => (
    <Link to={to} className={loc.pathname === to ? "text-fg" : "text-fgmute"}>
      {label}
    </Link>
  );
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1">
          {navItem("/", "Todos")}
          {navItem("/calendar", "Calendar")}
          {navItem("/settings", "Settings")}
          {navItem("/popover", "Compact view")}
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Routes>
          <Route path="/" element={<Todos />} />
          <Route path="/calendar" element={<Calendar />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Todos />} />
        </Routes>
      </main>
    </div>
  );
}
```

- [ ] **Step 17.4: Build + commit (combine with Task 18 since Settings.tsx is required)**

Defer commit to Task 18.5.

---

## Task 18: Settings page + final wiring

**Files:**
- Create: `web/src/pages/Settings.tsx`
- Create: `web/src/components/SyncStatusBadge.tsx`

- [ ] **Step 18.1: Create `web/src/components/SyncStatusBadge.tsx`**

```tsx
import { Check, AlertCircle } from "lucide-react";
import type { SyncStateEntry } from "../lib/types";

export function SyncStatusBadge({ state }: { state: SyncStateEntry }) {
  if (state.lastErr) {
    return (
      <span className="inline-flex items-center gap-1 text-urgent text-xs">
        <AlertCircle size={12} />
        {state.lastErr}
      </span>
    );
  }
  if (state.lastOkAt) {
    const ago = Math.max(0, Math.round((Date.now() / 1000 - state.lastOkAt) / 60));
    return (
      <span className="inline-flex items-center gap-1 text-success text-xs">
        <Check size={12} />
        {ago < 1 ? "just synced" : `synced ${ago}m ago`}
      </span>
    );
  }
  return <span className="text-fgmute text-xs">never synced</span>;
}
```

- [ ] **Step 18.2: Create `web/src/pages/Settings.tsx`**

```tsx
import { useEffect, useState } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { SyncStatusBadge } from "../components/SyncStatusBadge";

export function Settings() {
  const { syncStates, loadSyncStatus } = useTodoStore();

  const [caldavUrl, setCaldavUrl] = useState("https://caldav.yandex.ru/");
  const [caldavUser, setCaldavUser] = useState("");
  const [caldavPass, setCaldavPass] = useState("");
  const [defaultNotifyMin, setDefaultNotifyMin] = useState("5");
  const [savingCaldav, setSavingCaldav] = useState(false);
  const [savingNotifyMin, setSavingNotifyMin] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void loadSyncStatus();
    void api.getKv("caldav.url").then((r) => r.value && setCaldavUrl(r.value));
    void api.getKv("caldav.username").then((r) => r.value && setCaldavUser(r.value));
    void api.getKv("meeting.default_notify_min").then((r) => r.value && setDefaultNotifyMin(r.value));
  }, [loadSyncStatus]);

  async function saveCaldav() {
    setSavingCaldav(true);
    try {
      await api.setKv("caldav.url", caldavUrl);
      await api.setKv("caldav.username", caldavUser);
      if (caldavPass) {
        await api.setSecret("yandex_caldav", caldavPass);
        setCaldavPass("");
      }
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingCaldav(false);
    }
  }

  async function saveNotifyMin() {
    setSavingNotifyMin(true);
    try {
      await api.setKv("meeting.default_notify_min", defaultNotifyMin);
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingNotifyMin(false);
    }
  }

  async function syncNow() {
    await api.triggerSync("caldav");
    setTimeout(() => void loadSyncStatus(), 1000);
  }

  const caldavState = syncStates.find((s) => s.source === "caldav");

  return (
    <div className="flex flex-col gap-8 max-w-2xl">
      <h2 className="text-xl font-semibold">Settings</h2>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Yandex CalDAV</h3>
        <label className="flex flex-col gap-1">
          <span className="text-sm">URL</span>
          <input
            type="text"
            value={caldavUrl}
            onChange={(e) => setCaldavUrl(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Username</span>
          <input
            type="text"
            value={caldavUser}
            onChange={(e) => setCaldavUser(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Password (leave blank to keep existing)</span>
          <input
            type="password"
            value={caldavPass}
            onChange={(e) => setCaldavPass(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <div className="flex items-center gap-3">
          <button
            onClick={saveCaldav}
            disabled={savingCaldav}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingCaldav ? "saving…" : "save credentials"}
          </button>
          <button
            onClick={syncNow}
            className="px-3 py-1 bg-bgsub border border-bgmute rounded text-sm hover:border-fgmute"
          >
            sync now
          </button>
          {caldavState && <SyncStatusBadge state={caldavState} />}
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Notifications</h3>
        <label className="flex flex-col gap-1 max-w-xs">
          <span className="text-sm">Default minutes before meeting</span>
          <input
            type="number"
            min={0}
            value={defaultNotifyMin}
            onChange={(e) => setDefaultNotifyMin(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <div>
          <button
            onClick={saveNotifyMin}
            disabled={savingNotifyMin}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingNotifyMin ? "saving…" : "save"}
          </button>
        </div>
      </section>

      {savedAt && <div className="text-fgmute text-xs">saved at {savedAt}</div>}
    </div>
  );
}
```

- [ ] **Step 18.3: Build + smoke**

```bash
cd web && pnpm build && pnpm lint
cd /home/spk/IdeaProjects/spk-task-manager && make build
```

- [ ] **Step 18.4: Commit Tasks 17 + 18 together**

```bash
git add web/src/components/MeetingCard.tsx web/src/components/SyncStatusBadge.tsx \
        web/src/pages/Calendar.tsx web/src/pages/Settings.tsx web/src/App.tsx
git commit -m "feat: Calendar + Settings pages with CalDAV + notify_min"
```

---

## Task 19: README + tag

**Files:**
- Modify: `README.md`

- [ ] **Step 19.1: Update `README.md`**

Replace the `## Status` section with:

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
- Time-tracking on todos with one-active-globally invariant
- TimerBadge component (live elapsed counter)
- Quick-add inline syntax (`!priority #tag due:tomorrow`) with live preview
- Compact `/popover` route showing today / active timer / quick add
- CLI: `cockpit timer start | stop | status`
- Tray tooltip reflects active timer

### Phase 3 ✅
- Read-only sync of meetings from Yandex Calendar (CalDAV)
- AES-256-GCM-encrypted secrets backed by OS keyring
- DBus system notifications fired N minutes before each meeting (default 5, per-meeting override)
- Markdown notes attached to meetings
- Calendar page (Today / Tomorrow / Later)
- Settings page (CalDAV credentials, default notify_min, sync now)
- CLI: `cockpit meeting next | list`, `cockpit secret set | list`
- Tray tooltip surfaces next meeting (within 24h)

Phase 4 (standup helper, GitLab/Tracker integrations, autostart, GitHub Actions release) is planned separately.
```

In the dependencies / build section, add to the `apt install` line:

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-1-dev
```

`libsecret-1-dev` is required at build time for `zalando/go-keyring` on Linux.

In the CLI examples block, append:

```bash
cockpit meeting list -d 14                # next 14 days
cockpit meeting next                      # closest upcoming
cockpit secret set yandex_caldav          # reads value from stdin
echo "my-app-password" | cockpit secret set yandex_caldav
```

- [ ] **Step 19.2: Final smoke (skip visual due to known multi-monitor screenshot issue)**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
make build
go test ./internal/...
golangci-lint run
cd web && pnpm build && pnpm lint
```

All commands must succeed cleanly.

- [ ] **Step 19.3: Commit + tag**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add README.md
git commit -m "docs: update README for phase 3 completion"
git tag v0.3.0-phase3
```

---

## Phase 3 Done — Definition of Done

- [ ] Migration 0003 applied; meetings, notes, secrets, sync_state tables exist.
- [ ] `MeetingRepo`, `NoteRepo`, `SecretRepo` (SQLite + fake) pass conformance tests.
- [ ] `meeting.Service` enforces ManualOnly / valid range / reschedule-resets-notification rules.
- [ ] `note.Service` upserts a single note per attachment.
- [ ] `secret.Service` round-trips AES-256-GCM ciphertext under env-var or keyring master key.
- [ ] CalDAV syncer parses an iCal fixture, upserts meetings, marks vanished events cancelled.
- [ ] DBus notifier sends real notifications on Linux; falls back to noop logger when DBus unavailable.
- [ ] NotificationScheduler fires once per meeting and is dedup'd via `notified_at`.
- [ ] HTTP endpoints work: `/api/meetings`, `/api/meetings/next`, `/api/meetings/{id}/note`, `/api/notes`, `/api/secrets`, `/api/sync`, `/api/kv`.
- [ ] CLI works: `cockpit meeting next`, `cockpit meeting list`, `cockpit secret set`, `cockpit secret list`.
- [ ] Web has Calendar page with Today/Tomorrow/Later sections, click-to-edit notes.
- [ ] Web has Settings page with CalDAV creds (saved to `kv` + `secrets`), default notify_min, sync-now button.
- [ ] Tray tooltip shows the next meeting when no timer is active and the meeting is < 24h away.
- [ ] `make build` succeeds; `go test ./internal/...` PASS; `golangci-lint run` 0 issues; `pnpm build` clean; `pnpm lint` clean.
- [ ] README updated; `v0.3.0-phase3` tag created.
