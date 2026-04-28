package caldav

import (
	"log/slog"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"

	"github.com/spk/spk-cockpit/internal/api"
)

// recurIDFormat is the wire format CalDAV uses for RECURRENCE-ID values
// (YYYYMMDDTHHMMSS[Z]). We use the same format to:
//   - look up overrides emitted by the server,
//   - mint a unique external_uid suffix per expanded occurrence so the meetings
//     UNIQUE INDEX (source, external_uid) treats each instance as its own row.
const recurIDFormat = "20060102T150405"

// eventBundle groups a recurring event series: one master VEVENT (no
// RECURRENCE-ID) plus zero-or-more overrides keyed by their RECURRENCE-ID
// in `recurIDFormat`. For non-recurring events the master is set, overrides
// is empty, and the master alone produces one meeting.
type eventBundle struct {
	master    *ical.Component
	overrides map[string]*ical.Component
}

// expandEvents walks a parsed iCal calendar and emits one api.Meeting per
// occurrence inside [from, to], with full RRULE expansion. RECURRENCE-ID
// overrides replace the corresponding master occurrence; EXDATE entries are
// dropped. CalDAV servers (Yandex notably) return the whole series for any
// time-range REPORT instead of pre-expanded occurrences, so doing the
// expansion locally is the only way to populate a forward-looking window when
// the master DTSTART sits in the past.
func expandEvents(cal *ical.Calendar, from, to time.Time) []api.Meeting {
	if cal == nil {
		return nil
	}
	bundles := map[string]*eventBundle{}
	for _, comp := range cal.Children {
		if comp.Name != ical.CompEvent {
			continue
		}
		uid := propString(comp, ical.PropUID)
		if uid == "" {
			continue
		}
		b := bundles[uid]
		if b == nil {
			b = &eventBundle{overrides: map[string]*ical.Component{}}
			bundles[uid] = b
		}
		recID := propString(comp, ical.PropRecurrenceID)
		if recID == "" {
			b.master = comp
		} else {
			b.overrides[normalizeRecurID(recID)] = comp
		}
	}

	var out []api.Meeting
	for uid, b := range bundles {
		out = append(out, expandBundle(uid, b, from, to)...)
	}
	return out
}

func expandBundle(uid string, b *eventBundle, from, to time.Time) []api.Meeting {
	if b.master == nil {
		// Server gave us overrides without the master (rare). Treat each as a
		// one-off, keyed by its own RECURRENCE-ID for uniqueness.
		var out []api.Meeting
		for recID, ovr := range b.overrides {
			if m, ok := componentToMeeting(uid+"::"+recID, ovr, ovr, from, to); ok {
				out = append(out, m)
			}
		}
		return out
	}

	masterStart, err := (&ical.Event{Component: b.master}).DateTimeStart(time.UTC)
	if err != nil || masterStart.IsZero() {
		slog.Debug("expandEvents: bad master DTSTART", "uid", uid, "err", err)
		return nil
	}

	rruleStr := propString(b.master, ical.PropRecurrenceRule)
	if rruleStr == "" {
		// Single event — no expansion needed.
		if m, ok := componentToMeeting(uid, b.master, b.master, from, to); ok {
			return []api.Meeting{m}
		}
		return nil
	}

	r, err := rrule.StrToRRule(rruleStr)
	if err != nil {
		slog.Debug("expandEvents: invalid RRULE; emitting master only", "uid", uid, "rrule", rruleStr, "err", err)
		if m, ok := componentToMeeting(uid, b.master, b.master, from, to); ok {
			return []api.Meeting{m}
		}
		return nil
	}
	r.DTStart(masterStart)

	exdates := parseExDates(b.master)
	masterDur := durationOf(b.master)

	var out []api.Meeting
	// Pad the window slightly on the master side so meetings whose start is
	// just before `from` but end inside the window aren't dropped.
	for _, occ := range r.Between(from.Add(-masterDur), to, true) {
		if isEXDated(occ, exdates) {
			continue
		}
		recID := occ.UTC().Format(recurIDFormat)
		var startAt, endAt time.Time
		src := b.master
		if ovr, ok := b.overrides[recID]; ok {
			src = ovr
			if s, err := (&ical.Event{Component: ovr}).DateTimeStart(time.UTC); err == nil && !s.IsZero() {
				startAt = s
			}
			if e, err := (&ical.Event{Component: ovr}).DateTimeEnd(time.UTC); err == nil && !e.IsZero() {
				endAt = e
			}
		}
		if startAt.IsZero() {
			startAt = occ
		}
		if endAt.IsZero() {
			endAt = startAt.Add(masterDur)
		}
		if !startAt.Before(to) || !endAt.After(from) {
			continue
		}
		out = append(out, api.Meeting{
			Source:      api.MeetingSourceCalDAV,
			ExternalUID: uid + "::" + recID,
			Title:       propString(src, ical.PropSummary),
			Description: propString(src, ical.PropDescription),
			Location:    propString(src, ical.PropLocation),
			StartAt:     startAt.Unix(),
			EndAt:       endAt.Unix(),
		})
	}
	return out
}

// componentToMeeting reads a single VEVENT component as one meeting, filtered
// by [from, to]. Used for non-recurring events and for orphan overrides.
func componentToMeeting(externalUID string, src, timing *ical.Component, from, to time.Time) (api.Meeting, bool) {
	start, err := (&ical.Event{Component: timing}).DateTimeStart(time.UTC)
	if err != nil || start.IsZero() {
		return api.Meeting{}, false
	}
	end, err := (&ical.Event{Component: timing}).DateTimeEnd(time.UTC)
	if err != nil || end.IsZero() {
		end = start.Add(durationOf(timing))
	}
	if !start.Before(to) || !end.After(from) {
		return api.Meeting{}, false
	}
	return api.Meeting{
		Source:      api.MeetingSourceCalDAV,
		ExternalUID: externalUID,
		Title:       propText(src, ical.PropSummary),
		Description: propText(src, ical.PropDescription),
		Location:    propText(src, ical.PropLocation),
		StartAt:     start.Unix(),
		EndAt:       end.Unix(),
	}, true
}

// durationOf returns DTEND-DTSTART, or 1h if DTEND is missing and DURATION
// can't be parsed.
func durationOf(c *ical.Component) time.Duration {
	ev := &ical.Event{Component: c}
	s, _ := ev.DateTimeStart(time.UTC)
	e, _ := ev.DateTimeEnd(time.UTC)
	if !s.IsZero() && !e.IsZero() {
		return e.Sub(s)
	}
	return time.Hour
}

// parseExDates collects all EXDATE values into a slice of UTC times.
// iCal allows multiple EXDATE properties and each may carry comma-separated
// values, e.g. `EXDATE;TZID=Europe/Moscow:20231124T130000,20231201T130000`.
func parseExDates(c *ical.Component) []time.Time {
	if c == nil {
		return nil
	}
	props, ok := c.Props[ical.PropExceptionDates]
	if !ok {
		return nil
	}
	var out []time.Time
	for _, p := range props {
		for _, raw := range strings.Split(p.Value, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if t, ok := parseICalTime(raw); ok {
				out = append(out, t.UTC())
			}
		}
	}
	return out
}

// parseICalTime parses iCal datetime values: YYYYMMDDTHHMMSSZ or YYYYMMDDTHHMMSS or YYYYMMDD.
func parseICalTime(s string) (time.Time, bool) {
	for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isEXDated(t time.Time, exdates []time.Time) bool {
	for _, ex := range exdates {
		if ex.Equal(t.UTC()) {
			return true
		}
	}
	return false
}

// normalizeRecurID strips a TZID parameter and trailing Z so the override map
// uses the same key shape that RRule.Between(...) outputs in recurIDFormat.
func normalizeRecurID(s string) string {
	// emersion/go-ical strips the parameters, leaving e.g. "20251016T120000".
	// Some servers send "20251016T120000Z"; trim the trailing Z.
	return strings.TrimSuffix(strings.TrimSpace(s), "Z")
}
