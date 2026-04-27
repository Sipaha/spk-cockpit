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

const meetingCols = `id, source, external_uid, external_etag, title, description, location,
	       start_at, end_at, notify_min, notified_at, popup_min, popup_fired_at, cancelled, created_at, updated_at`

// Create inserts a meeting. ID and timestamps must be set by caller.
func (r *MeetingRepo) Create(ctx context.Context, m api.Meeting) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO meetings(`+meetingCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, string(m.Source), nullStr(m.ExternalUID), nullStr(m.ExternalETag),
		m.Title, m.Description, m.Location,
		m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, m.PopupMin, m.PopupFiredAt,
		boolToInt(m.Cancelled), m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert meeting: %w", err)
	}
	return nil
}

// Get returns a non-deleted meeting by id.
func (r *MeetingRepo) Get(ctx context.Context, id string) (api.Meeting, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+meetingCols+` FROM meetings WHERE id = ? AND deleted_at IS NULL`, id)
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

	row := tx.QueryRowContext(ctx, `SELECT `+meetingCols+` FROM meetings WHERE id = ? AND deleted_at IS NULL`, id)
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
		                    start_at=?, end_at=?, notify_min=?, notified_at=?, popup_min=?, popup_fired_at=?,
		                    cancelled=?, updated_at=?
		WHERE id=?`,
		string(m.Source), nullStr(m.ExternalUID), nullStr(m.ExternalETag),
		m.Title, m.Description, m.Location,
		m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, m.PopupMin, m.PopupFiredAt,
		boolToInt(m.Cancelled), m.UpdatedAt, m.ID)
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
	q := `SELECT ` + meetingCols + ` FROM meetings WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY start_at ASC` //nolint:gosec // condition values are controlled literals, not user input
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit) //nolint:gosec // controlled int
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

// UpsertExternal inserts or updates by (source, external_uid). When start_at changes,
// notified_at AND popup_fired_at reset so re-arming works for both notification channels.
func (r *MeetingRepo) UpsertExternal(ctx context.Context, m api.Meeting) (api.Meeting, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Meeting{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+meetingCols+` FROM meetings WHERE source = ? AND external_uid = ? AND deleted_at IS NULL`,
		string(m.Source), m.ExternalUID)
	existing, err := scanMeeting(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO meetings(`+meetingCols+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, string(m.Source), m.ExternalUID, nullStr(m.ExternalETag),
			m.Title, m.Description, m.Location,
			m.StartAt, m.EndAt, m.NotifyMin, m.NotifiedAt, m.PopupMin, m.PopupFiredAt,
			boolToInt(m.Cancelled), m.CreatedAt, m.UpdatedAt,
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
			preserved.PopupFiredAt = nil
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE meetings SET external_etag=?, title=?, description=?, location=?,
			                    start_at=?, end_at=?, notified_at=?, popup_fired_at=?, cancelled=?, updated_at=?
			WHERE id=?`,
			nullStr(preserved.ExternalETag), preserved.Title, preserved.Description, preserved.Location,
			preserved.StartAt, preserved.EndAt, preserved.NotifiedAt, preserved.PopupFiredAt,
			boolToInt(preserved.Cancelled), preserved.UpdatedAt, preserved.ID); err != nil {
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

// PendingNotification returns meetings ready to be notified via the DBus channel.
func (r *MeetingRepo) PendingNotification(ctx context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+meetingCols+`
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

// PendingPopup returns meetings ready for the on-screen window popup. Independent
// from PendingNotification: each channel has its own time threshold and antifire marker.
func (r *MeetingRepo) PendingPopup(ctx context.Context, now int64, defaultPopupMin int) ([]api.Meeting, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+meetingCols+`
		FROM meetings
		WHERE deleted_at IS NULL
		  AND cancelled = 0
		  AND popup_fired_at IS NULL
		  AND start_at - COALESCE(popup_min, ?) * 60 <= ?
		  AND start_at >= ?
		ORDER BY start_at ASC`,
		defaultPopupMin, now, now)
	if err != nil {
		return nil, fmt.Errorf("pending popup: %w", err)
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

// MarkPopupFired sets popup_fired_at on a single row.
func (r *MeetingRepo) MarkPopupFired(ctx context.Context, id string, at int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE meetings SET popup_fired_at=? WHERE id=?`, at, id)
	if err != nil {
		return fmt.Errorf("mark popup fired: %w", err)
	}
	return nil
}

func scanMeeting(s sessionScanner) (api.Meeting, error) {
	var m api.Meeting
	var srcStr string
	var extUID, extETag sql.NullString
	var notifyMin, notifiedAt, popupMin, popupFiredAt sql.NullInt64
	var cancelledI int
	if err := s.Scan(&m.ID, &srcStr, &extUID, &extETag,
		&m.Title, &m.Description, &m.Location,
		&m.StartAt, &m.EndAt, &notifyMin, &notifiedAt, &popupMin, &popupFiredAt, &cancelledI,
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
	if popupMin.Valid {
		v := int(popupMin.Int64)
		m.PopupMin = &v
	}
	if popupFiredAt.Valid {
		v := popupFiredAt.Int64
		m.PopupFiredAt = &v
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
