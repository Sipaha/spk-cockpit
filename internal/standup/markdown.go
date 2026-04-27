package standup

import (
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
)

// Markdown renders a StandupReport as a copy-paste-friendly markdown block.
// The format is:
//
//	# Standup — YYYY-MM-DD
//
//	## Yesterday
//	- [todo] Title — done
//	- [git] feat: thing — team/x
//	- [pt] TICKET-1: Title — done
//
//	## Today
//	- [todo] Title — in progress
//
//	## Blockers
//	- [todo] Title — overdue
//
// Empty sections are still emitted with "_(none)_" so the structure stays predictable.
func Markdown(r api.StandupReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Standup — %s\n\n", r.Day)
	writeSection(&b, "Yesterday", r.Yesterday)
	writeSection(&b, "Today", r.Today)
	writeSection(&b, "Blockers", r.Blockers)
	if len(r.Errors) > 0 {
		b.WriteString("\n_Source errors:_\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}

func writeSection(b *strings.Builder, name string, items []api.StandupItem) {
	fmt.Fprintf(b, "## %s\n", name)
	if len(items) == 0 {
		b.WriteString("_(none)_\n\n")
		return
	}
	for _, it := range items {
		tag := tagFor(it.Source)
		line := fmt.Sprintf("- %s %s", tag, it.Title)
		if it.Detail != "" {
			line += " — " + it.Detail
		}
		if it.URL != "" {
			line += " (" + it.URL + ")"
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func tagFor(s api.StandupItemSource) string {
	switch s {
	case api.StandupSourceTodo:
		return "[todo]"
	case api.StandupSourceGitLab:
		return "[git]"
	case api.StandupSourceTracker:
		return "[pt]"
	default:
		return "[?]"
	}
}
