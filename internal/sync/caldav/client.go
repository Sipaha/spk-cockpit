// Package caldav provides a CalDAV client for spk-cockpit's read-only meeting
// sync. Works with any RFC 4791-compliant server (Yandex, Fastmail, iCloud,
// Nextcloud, Posteo, mailbox.org, …); the user supplies the collection URL and
// credentials in Settings.
package caldav

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is used only for non-security ctag fingerprinting
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/spk/spk-cockpit/internal/api"
)

// Config holds the credentials and endpoint for a CalDAV server.
type Config struct {
	BaseURL  string
	Username string
	Password string
}

// Client is the abstraction the syncer talks to.
type Client interface {
	// FetchEvents returns events whose start time falls in [from, to].
	// prevCTag may be used by implementations for delta-sync; the new ctag is
	// always returned (even if unchanged is true).
	FetchEvents(ctx context.Context, from, to time.Time, prevCTag string) (events []api.Meeting, newCTag string, unchanged bool, err error)
}

type httpClient struct {
	cfg Config
	cal *caldav.Client
}

// NewClient constructs a real CalDAV client. The HTTP transport is wrapped with
// quoteETagTransport so servers that emit unquoted ETags (Yandex among them)
// don't trip the strict RFC 7232 parser inside emersion/go-webdav.
func NewClient(cfg Config) (Client, error) {
	base := &http.Client{
		Transport: &quoteETagTransport{inner: http.DefaultTransport},
	}
	httpAuth := webdav.HTTPClientWithBasicAuth(base, cfg.Username, cfg.Password)
	cl, err := caldav.NewClient(httpAuth, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("caldav client: %w", err)
	}
	return &httpClient{cfg: cfg, cal: cl}, nil
}

// FetchEvents queries the calendar for events in [from, to].
//
// Tries discovery first (FindCalendars) and uses the first event-bearing
// collection found. Falls back to querying the configured BaseURL directly
// when discovery returns nothing or fails — this is the case for Yandex,
// which does not advertise calendars at the well-known principal location and
// expects clients to know the events-default URL upfront.
func (c *httpClient) FetchEvents(ctx context.Context, from, to time.Time, _ string) ([]api.Meeting, string, bool, error) {
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

	paths := c.discoverCalendarPaths(ctx)
	slog.Debug("caldav.FetchEvents: discovered paths", "count", len(paths), "paths", paths)
	if len(paths) == 0 {
		slog.Debug("caldav.FetchEvents: discovery empty, using BaseURL directly", "url", c.cfg.BaseURL)
		paths = []string{c.cfg.BaseURL}
	}

	var (
		objects []caldav.CalendarObject
		lastErr error
	)
	for _, p := range paths {
		objs, err := c.cal.QueryCalendar(ctx, p, q)
		if err != nil {
			slog.Debug("caldav.FetchEvents: QueryCalendar failed", "path", p, "err", err)
			lastErr = err
			continue
		}
		slog.Debug("caldav.FetchEvents: QueryCalendar ok", "path", p, "objects", len(objs))
		objects = append(objects, objs...)
	}
	slog.Debug("caldav.FetchEvents: total objects", "count", len(objects))
	if len(objects) == 0 && lastErr != nil {
		return nil, "", false, fmt.Errorf("query: %w", lastErr)
	}
	var meetings []api.Meeting
	for _, obj := range objects {
		evs := ParseICalEvents(obj.Data, from, to)
		slog.Debug("caldav.FetchEvents: parsed object", "etag", obj.ETag, "events", len(evs))
		for _, e := range evs {
			e.ExternalETag = obj.ETag
			meetings = append(meetings, e)
		}
	}
	slog.Debug("caldav.FetchEvents: total parsed events", "count", len(meetings), "from", from, "to", to)
	h := sha1.New() //nolint:gosec // non-security use: ctag fingerprint only
	for _, m := range meetings {
		_, _ = io.WriteString(h, m.ExternalUID)
		_, _ = io.WriteString(h, m.ExternalETag)
	}
	return meetings, hex.EncodeToString(h.Sum(nil)), false, nil
}

// discoverCalendarPaths runs FindCalendars and returns the paths of every
// returned collection. If discovery fails (or returns nothing), the caller
// falls back to the explicit BaseURL configured by the user.
func (c *httpClient) discoverCalendarPaths(ctx context.Context) []string {
	cals, err := c.cal.FindCalendars(ctx, "")
	if err != nil || len(cals) == 0 {
		return nil
	}
	out := make([]string, 0, len(cals))
	for _, cal := range cals {
		if cal.Path != "" {
			out = append(out, cal.Path)
		}
	}
	return out
}

// ParseICalEvents extracts api.Meeting values from an iCal object, restricted to [from, to].
func ParseICalEvents(cal *ical.Calendar, from, to time.Time) []api.Meeting {
	var out []api.Meeting
	if cal == nil {
		return nil
	}
	for _, comp := range cal.Children {
		if comp.Name != ical.CompEvent {
			slog.Debug("ParseICalEvents: skip non-VEVENT", "name", comp.Name)
			continue
		}
		ev := ical.Event{Component: comp}
		uid := propString(ev.Component, ical.PropUID)
		summary := propString(ev.Component, ical.PropSummary)
		hasRRULE := propString(ev.Component, ical.PropRecurrenceRule) != ""
		recurID := propString(ev.Component, ical.PropRecurrenceID)
		dtStartRaw := propString(ev.Component, ical.PropDateTimeStart)
		dtEndRaw := propString(ev.Component, ical.PropDateTimeEnd)
		duration := propString(ev.Component, ical.PropDuration)

		if uid == "" {
			slog.Debug("ParseICalEvents: skip — no UID", "summary", summary)
			continue
		}
		desc := propString(ev.Component, ical.PropDescription)
		loc := propString(ev.Component, ical.PropLocation)

		start, err := ev.DateTimeStart(time.UTC)
		if err != nil {
			slog.Debug("ParseICalEvents: skip — DateTimeStart error", "uid", uid, "summary", summary, "dtstart_raw", dtStartRaw, "err", err)
			continue
		}
		if start.IsZero() {
			slog.Debug("ParseICalEvents: skip — DateTimeStart zero", "uid", uid, "summary", summary, "dtstart_raw", dtStartRaw)
			continue
		}
		end, err := ev.DateTimeEnd(time.UTC)
		if err != nil || end.IsZero() {
			end = start.Add(time.Hour)
		}
		if !start.Before(to) || !end.After(from) {
			slog.Debug("ParseICalEvents: skip — outside window",
				"uid", uid, "summary", summary,
				"start", start, "end", end, "window_from", from, "window_to", to,
				"hasRRULE", hasRRULE, "recurrenceId", recurID,
				"dtstart_raw", dtStartRaw, "dtend_raw", dtEndRaw, "duration_raw", duration)
			continue
		}
		out = append(out, api.Meeting{
			Source:      api.MeetingSourceCalDAV,
			ExternalUID: uid,
			Title:       summary,
			Description: desc,
			Location:    loc,
			StartAt:     start.Unix(),
			EndAt:       end.Unix(),
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
