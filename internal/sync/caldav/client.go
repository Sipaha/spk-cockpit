// Package caldav provides a CalDAV client tailored for spk-cockpit's read-only
// sync use case (Yandex Calendar).
package caldav

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is used only for non-security ctag fingerprinting
	"encoding/hex"
	"fmt"
	"io"
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

// NewClient constructs a real CalDAV client.
func NewClient(cfg Config) (Client, error) {
	httpAuth := webdav.HTTPClientWithBasicAuth(nil, cfg.Username, cfg.Password)
	cl, err := caldav.NewClient(httpAuth, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("caldav client: %w", err)
	}
	return &httpClient{cfg: cfg, cal: cl}, nil
}

// FetchEvents queries the primary calendar for events in [from, to].
func (c *httpClient) FetchEvents(ctx context.Context, from, to time.Time, _ string) ([]api.Meeting, string, bool, error) {
	calendars, err := c.cal.FindCalendars(ctx, "")
	if err != nil {
		return nil, "", false, fmt.Errorf("find calendars: %w", err)
	}
	if len(calendars) == 0 {
		return nil, "", false, nil
	}
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
		evs := ParseICalEvents(obj.Data, from, to)
		for _, e := range evs {
			e.ExternalETag = obj.ETag
			meetings = append(meetings, e)
		}
	}
	h := sha1.New() //nolint:gosec // non-security use: ctag fingerprint only
	for _, m := range meetings {
		_, _ = io.WriteString(h, m.ExternalUID)
		_, _ = io.WriteString(h, m.ExternalETag)
	}
	return meetings, hex.EncodeToString(h.Sum(nil)), false, nil
}

// ParseICalEvents extracts api.Meeting values from an iCal object, restricted to [from, to].
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
			end = start.Add(time.Hour)
		}
		if !start.Before(to) || !end.After(from) {
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
