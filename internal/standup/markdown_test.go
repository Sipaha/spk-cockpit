package standup_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/standup"
)

func TestMarkdown_FullReport(t *testing.T) {
	r := api.StandupReport{
		Day: "2026-04-27",
		Yesterday: []api.StandupItem{
			{Source: api.StandupSourceTodo, Title: "Ship X", Detail: "done"},
			{Source: api.StandupSourceGitLab, Title: "feat: thing", Detail: "team/x", URL: "https://gl/x"},
		},
		Today: []api.StandupItem{
			{Source: api.StandupSourceTodo, Title: "Polish UI", Detail: "in progress"},
		},
		Blockers: nil,
	}
	md := standup.Markdown(r)
	require.True(t, strings.HasPrefix(md, "# Standup — 2026-04-27"))
	require.Contains(t, md, "## Yesterday\n")
	require.Contains(t, md, "- [todo] Ship X — done\n")
	require.Contains(t, md, "- [git] feat: thing — team/x (https://gl/x)\n")
	require.Contains(t, md, "## Today\n")
	require.Contains(t, md, "- [todo] Polish UI — in progress\n")
	require.Contains(t, md, "## Blockers\n_(none)_\n")
}

func TestMarkdown_WithErrors(t *testing.T) {
	r := api.StandupReport{
		Day:    "2026-04-27",
		Errors: []string{"gitlab: 401"},
	}
	md := standup.Markdown(r)
	require.Contains(t, md, "_Source errors:_")
	require.Contains(t, md, "- gitlab: 401")
}
