# spk-cockpit

A personal productivity tray app for Linux. Manage todos, your calendar, time-tracking, and a daily standup — all from a single Go binary with an embedded React UI that lives in your system tray and stays out of your way.

Licensed under [Apache 2.0](LICENSE) — free for personal and commercial use.

## Features

- **Todos** with priority, tags, due dates, and a full audit history. Quick-add inline syntax: `Fix login !urgent #backend due:tomorrow`.
- **Time tracking** — start/stop a timer on any todo. Only one runs at a time; daily totals are aggregated automatically.
- **Calendar** — read-only sync from Yandex Calendar via CalDAV. DBus desktop notifications fire N minutes before each meeting (default 5; per-meeting override).
- **Markdown notes** attached to meetings or todos, with revision history.
- **Daily standup helper** — auto-aggregates "Yesterday / Today / Blockers" from completed todos, your GitLab commits, and your Citeck Project Tracker activity. One-click copy as markdown.
- **Info-rich tray menu** — live status (active timer, next meeting, overdue count, sync errors) and quick actions (open standup, stop timer, refresh sync).
- **Encrypted secrets** — AES-256-GCM with the master key sourced from the OS keyring (libsecret on Linux).
- **Single static binary** — Go server + embedded React/Vite/Tailwind UI, served over a Unix domain socket. No Docker, no daemon manager other than systemd-user.

## Build

System dependencies (Ubuntu/Debian):

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-1-dev
```

Then:

```bash
make build
./build/bin/spk-cockpit start
```

`make build` uses `-tags "webkit2_41 production"` for webkit2gtk 4.1 compatibility and Wails production mode.

To autostart on login, install the systemd-user unit:

```bash
./build/bin/spk-cockpit install --autostart   # use --uninstall to remove
```

## CLI

The `cockpit` CLI talks to the running daemon over its Unix socket — same code path as the UI.

```bash
# todos
cockpit todo add "Review MR !1245" -p high -t backend
cockpit todo list                                       # use -a for done/cancelled
cockpit todo start abc123                               # set in_progress (short id suffix)
cockpit todo update abc123 -p urgent --due 2026-05-01
cockpit todo done abc123
cockpit todo rm abc123

# timer
cockpit timer start abc123
cockpit timer status
cockpit timer stop

# calendar / secrets
cockpit meeting list -d 14
cockpit meeting next
echo "my-app-password" | cockpit secret set yandex_caldav

# standup
cockpit standup                       # markdown for today
cockpit standup --date 2026-04-26     # markdown for any day

# lifecycle
cockpit start                         # launches tray + window + daemon
cockpit install --autostart           # systemd-user unit
cockpit stop                          # stop running daemon
```

## Configuration

Static config lives in `~/.config/spk-cockpit/config.yml` (theme, log level, sync intervals). Runtime settings live in the SQLite KV store; secrets in the encrypted `secrets` table — both managed via the Settings page or the CLI/API.

GitLab and Tracker integrations for the standup helper are optional; configure them via:

```bash
cockpit secret set gitlab_token       # personal access token, read_api scope

SOCK=~/.local/state/spk-cockpit/cockpit.sock
curl --unix-socket "$SOCK" -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "https://gitlab.example.com"}' http://unix/api/kv/gitlab.url
curl --unix-socket "$SOCK" -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "alice"}' http://unix/api/kv/gitlab.author_username

# Citeck Project Tracker — same pattern with tracker.url, tracker.username, tracker_token.
```

## Filesystem layout

| Path | Contents |
|---|---|
| `~/.local/share/spk-cockpit/cockpit.db` | SQLite database |
| `~/.local/state/spk-cockpit/cockpit.sock` | Unix socket (HTTP + SSE API) |
| `~/.local/state/spk-cockpit/log/cockpit.log` | Daemon log |
| `~/.config/spk-cockpit/config.yml` | Static config |
| `~/.config/systemd/user/spk-cockpit.service` | Autostart unit (when installed) |

Override paths via `SPK_COCKPIT_DATA_DIR`, `SPK_COCKPIT_STATE_DIR`, `SPK_COCKPIT_CONFIG_DIR`.

## Architecture

The daemon owns one tray icon, one Wails window (hide-on-close), a Unix-domain-socket HTTP server (REST + SSE) at `~/.local/state/spk-cockpit/cockpit.sock`, and a handful of background workers (CalDAV syncer, notification scheduler, GC). The CLI is a thin client over the same UDS — there is no second SQLite path.

Sync is **read-only** in v1: CalDAV updates the local mirror, manual meetings stay local. GitLab and Tracker are queried on demand by the standup aggregator.

The domain layer (`internal/{todo,meeting,timer,note,secret,standup}`) is transport-agnostic — services take repository interfaces and a `Clock`, and know nothing about HTTP, Wails, or SQLite directly. This keeps the door open for a future MCP transport.

## Development

```bash
make test    # Go tests (with -race) + Vitest
make lint    # golangci-lint + eslint
make fmt
```

For frontend hot-reload run `cd web && pnpm dev`; the daemon serves the embedded build, but during development you can run vite separately.

## License

[Apache License 2.0](LICENSE).
