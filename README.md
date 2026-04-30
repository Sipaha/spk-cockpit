# spk-cockpit

A personal productivity tray app for Linux. Manage todos, your calendar, and time-tracking — all from a single Go binary with an embedded React UI that lives in your system tray and stays out of your way.

Licensed under [Apache 2.0](LICENSE) — free for personal and commercial use.

## Features

- **Todos** with priority, tags, due dates, and a full audit history. Quick-add inline syntax: `Fix login !urgent #backend due:tomorrow`.
- **Time tracking** — start/stop a timer on any todo. Only one runs at a time; daily totals are aggregated automatically.
- **Calendar** — read-only sync from any CalDAV server (Yandex, Fastmail, iCloud, Nextcloud, Posteo, mailbox.org, …). DBus desktop notifications fire N minutes before each meeting (default 5; per-meeting override) and a separate small popup window opens 1 minute before.
- **Markdown notes** attached to meetings or todos, with revision history.
- **Info-rich tray menu** — live status (active timer, next meeting, overdue count, sync errors) and quick actions (open window, quick-add todo, stop timer).
- **Encrypted secrets** — AES-256-GCM with the master key sourced from the OS keyring (libsecret on Linux).
- **Single self-contained binary** — Go server + embedded React/Vite/Tailwind UI, served over a Unix domain socket. CGO links libgtk-3, libwebkit2gtk-4.1, and libsecret-1 (see Build below). No Docker, no daemon manager other than systemd-user.

## Build

System dependencies (Ubuntu/Debian):

```bash
sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-1-dev
```

Then:

```bash
make build         # dev binary with DevTools (recommended for daily use)
./build/bin/spk-cockpit
```

`make release` produces a stripped production binary at `build/bin/spk-cockpit-release` with DevTools disabled.

To autostart on login, install the systemd-user unit:

```bash
./build/bin/spk-cockpit install --autostart   # use --uninstall to remove
```

## CLI

The `spk-cockpit` CLI talks to the running daemon over its Unix socket — same code path as the UI.

```bash
# todos
spk-cockpit todo add "Review MR !1245" -p high -t backend
spk-cockpit todo list                                       # use -a for done/cancelled
spk-cockpit todo start abc123                               # set in_progress (short id suffix)
spk-cockpit todo update abc123 -p urgent --due 2026-05-01
spk-cockpit todo done abc123
spk-cockpit todo rm abc123

# timer
spk-cockpit timer start abc123
spk-cockpit timer status
spk-cockpit timer stop

# calendar / secrets
spk-cockpit meeting list -d 14
spk-cockpit meeting next
echo "my-app-password" | spk-cockpit secret set caldav_password

# lifecycle
spk-cockpit                               # launches tray + window + daemon (default action)
spk-cockpit install --autostart           # systemd-user unit
spk-cockpit stop                          # stop running daemon
```

## Configuration

Runtime settings (theme, sync intervals, integration URLs) live in the SQLite KV
store; secrets in the encrypted `secrets` table. Both are managed via the
Settings page or the CLI/API. Log level is set via the `SPK_COCKPIT_LOG_LEVEL`
environment variable.

CalDAV (read-only meeting sync) needs both KV keys and the password secret:

```bash
SOCK=~/.local/state/spk-cockpit/cockpit.sock
curl --unix-socket "$SOCK" -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "https://caldav.example.com/principals/alice/"}' http://unix/api/kv/caldav.url
curl --unix-socket "$SOCK" -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "alice"}' http://unix/api/kv/caldav.username
echo "my-app-password" | spk-cockpit secret set caldav_password
```

## Filesystem layout

| Path | Contents |
|---|---|
| `~/.local/share/spk-cockpit/cockpit.db` | SQLite database |
| `~/.local/state/spk-cockpit/cockpit.sock` | Unix socket (HTTP + SSE API) |
| `~/.local/state/spk-cockpit/log/cockpit.log` | Daemon log |
| `~/.config/systemd/user/spk-cockpit.service` | Autostart unit (when installed) |

Override paths via `SPK_COCKPIT_DATA_DIR`, `SPK_COCKPIT_STATE_DIR`, `SPK_COCKPIT_CONFIG_DIR`.

## Architecture

The daemon owns one tray icon, one Wails window (hide-on-close), a Unix-domain-socket HTTP server (REST + SSE) at `~/.local/state/spk-cockpit/cockpit.sock`, and a handful of background workers (CalDAV syncer, notification scheduler, GC). The CLI is a thin client over the same UDS — there is no second SQLite path.

Sync is **read-only** in v1: CalDAV updates the local mirror, manual meetings stay local.

The domain layer (`internal/{todo,meeting,timer,note,secret}`) is transport-agnostic — services take repository interfaces and a `Clock`, and know nothing about HTTP, Wails, or SQLite directly. This keeps the door open for a future MCP transport.

## Development

```bash
make test    # Go tests (-race, -tags wails) + Vitest
make lint    # golangci-lint --build-tags wails + eslint
make fmt
```

For frontend hot-reload run `cd web && pnpm dev` — Vite serves the UI directly and proxies `/api/*` to the running daemon's Unix socket. The Go binary only serves the embedded build. To debug Go and JS together with the WebKit inspector, run `make build` and launch `./build/bin/spk-cockpit` — DevTools (right-click → Inspect Element) are on by default in dev builds and disabled in `make release` builds.

## License

[Apache License 2.0](LICENSE). Third-party copyright notices and license texts are reproduced in [NOTICE](NOTICE) and [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md) (regenerate with `make licenses`).
