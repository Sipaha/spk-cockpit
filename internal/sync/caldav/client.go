// Package caldav provides a CalDAV client for spk-cockpit's read-only meeting
// sync. Works with any RFC 4791-compliant server (Yandex, Fastmail, iCloud,
// Nextcloud, Posteo, mailbox.org, …); the user supplies the collection URL and
// credentials in Settings.
package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
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
	raw *http.Client // used for raw PROPFIND (cs:getctag delta-sync probe)
}

// NewClient constructs a real CalDAV client. The HTTP transport is wrapped with
// quoteETagTransport so servers that emit unquoted ETags (Yandex among them)
// don't trip the strict RFC 7232 parser inside emersion/go-webdav.
func NewClient(cfg Config) (Client, error) {
	raw := &http.Client{
		Transport: &quoteETagTransport{inner: http.DefaultTransport},
	}
	httpAuth := webdav.HTTPClientWithBasicAuth(raw, cfg.Username, cfg.Password)
	cl, err := caldav.NewClient(httpAuth, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("caldav client: %w", err)
	}
	return &httpClient{cfg: cfg, cal: cl, raw: raw}, nil
}

// FetchEvents queries the calendar for events in [from, to].
//
// Tries discovery first (FindCalendars) and uses the first event-bearing
// collection found. Falls back to querying the configured BaseURL directly
// when discovery returns nothing or fails — this is the case for Yandex,
// which does not advertise calendars at the well-known principal location and
// expects clients to know the events-default URL upfront.
func (c *httpClient) FetchEvents(ctx context.Context, from, to time.Time, prevCTag string) ([]api.Meeting, string, bool, error) {
	paths := c.discoverCalendarPaths(ctx)
	slog.Debug("caldav.FetchEvents: discovered paths", "count", len(paths), "paths", paths)
	if len(paths) == 0 {
		slog.Debug("caldav.FetchEvents: discovery empty, using BaseURL directly", "url", c.cfg.BaseURL)
		paths = []string{c.cfg.BaseURL}
	}

	// Cheap delta-sync probe: PROPFIND cs:getctag for each collection. If the
	// server returns the same combined CTag we stored last time, nothing in any
	// collection changed and we can skip the (much heavier) calendar-query
	// REPORT entirely. Falling back to a full fetch if any collection's CTag is
	// missing keeps us correct against servers that don't expose getctag.
	combined := c.combinedCTag(ctx, paths)
	if combined != "" && combined == prevCTag {
		slog.Debug("caldav.FetchEvents: ctag unchanged, skipping query", "ctag", combined)
		return nil, combined, true, nil
	}

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
	slog.Debug("caldav.FetchEvents: total parsed events", "count", len(meetings), "from", from, "to", to, "ctag", combined)
	return meetings, combined, false, nil
}

// combinedCTag returns a stable string representing the union of all
// collections' CTags. Returns "" when CTag isn't reliably available, which
// suppresses the delta-sync short-circuit and keeps the full fetch path.
func (c *httpClient) combinedCTag(ctx context.Context, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(paths))
	for _, p := range paths {
		ctag, err := c.getCTag(ctx, p)
		if err != nil || ctag == "" {
			slog.Debug("caldav.combinedCTag: missing ctag, falling back to full fetch", "path", p, "err", err)
			return ""
		}
		pairs = append(pairs, p+"="+ctag)
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "\n")
}

// getCTag issues a PROPFIND Depth: 0 for cs:getctag against a single
// collection. The Calendar Server extension is the de-facto delta-sync
// primitive for CalDAV (RFC 6578's sync-collection is technically newer but
// less universally implemented — Yandex doesn't advertise it). Returns ""
// without error if the server replies but omits the property.
func (c *httpClient) getCTag(ctx context.Context, path string) (string, error) {
	target, err := c.absoluteURL(path)
	if err != nil {
		return "", err
	}
	body := strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/">
  <prop><cs:getctag/></prop>
</propfind>`)
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", target, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)

	resp, err := c.raw.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("propfind ctag: status %s", resp.Status)
	}

	var ms struct {
		XMLName  xml.Name `xml:"DAV: multistatus"`
		Response []struct {
			Propstat []struct {
				Prop struct {
					CTag string `xml:"http://calendarserver.org/ns/ getctag"`
				} `xml:"DAV: prop"`
				Status string `xml:"DAV: status"`
			} `xml:"DAV: propstat"`
		} `xml:"DAV: response"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return "", fmt.Errorf("decode multistatus: %w", err)
	}
	for _, r := range ms.Response {
		for _, ps := range r.Propstat {
			if ps.Prop.CTag != "" {
				return ps.Prop.CTag, nil
			}
		}
	}
	return "", nil
}

// absoluteURL turns a discovered collection path (which may be a relative
// "/calendars/.../events-default/" or already a full URL) into a full URL
// suitable for a raw PROPFIND request.
func (c *httpClient) absoluteURL(p string) (string, error) {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p, nil
	}
	base, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	if strings.HasPrefix(p, "/") {
		base.Path = p
	} else {
		base.Path = strings.TrimSuffix(base.Path, "/") + "/" + p
	}
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
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

// ParseICalEvents extracts api.Meeting values from an iCal object, restricted
// to [from, to]. Recurring series are expanded locally — see expandEvents for
// the rationale (CalDAV servers commonly return the full series on a
// time-range REPORT instead of pre-expanded occurrences).
func ParseICalEvents(cal *ical.Calendar, from, to time.Time) []api.Meeting {
	out := expandEvents(cal, from, to)
	slog.Debug("ParseICalEvents: expanded", "count", len(out), "window_from", from, "window_to", to)
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

// propText reads a TEXT-typed property and unescapes per RFC 5545 §3.3.11
// (\\ → \, \; → ;, \, → ,, \n / \N → linebreak). emersion/go-ical leaves the
// raw escaped value, so meeting Summary/Description/Location pass through
// unchanged otherwise — including literal "\n" sequences that would otherwise
// land inside hyperlinks.
func propText(c *ical.Component, name string) string {
	return unescapeICalText(propString(c, name))
}

func unescapeICalText(s string) string {
	if s == "" {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n', 'N':
				b = append(b, '\n')
				i++
				continue
			case '\\', ',', ';':
				b = append(b, s[i+1])
				i++
				continue
			}
		}
		b = append(b, s[i])
	}
	return string(b)
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
