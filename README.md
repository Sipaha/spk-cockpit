# spk-cockpit

Personal productivity tray app — todo list with prioritization, filtering, history, and a single-binary architecture (Go + embedded React UI). Linux first.

## Phase 1 status

- ✅ Todo CRUD (priority, status, due, tags, audit history)
- ✅ Tray icon with menu (Open window / Quit)
- ✅ Wails main window (webview wrapping the embedded React UI)
- ✅ CLI (`cockpit start | stop | todo add/list/done/rm`)
- ✅ SQLite storage with migrations
- ✅ HTTP/UDS server with SSE for realtime UI updates

Phases 2–4 (popover, time-tracking, meetings, CalDAV, standup, secrets, autostart, releases) are planned separately.

## Build

System dependencies (Ubuntu/Debian):

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
```

Then:

```bash
make build
./build/bin/spk-cockpit start          # opens tray + window
```

The build uses the `webkit2_41` and `production` Go build tags: `webkit2_41` for webkit2gtk 4.1 compatibility (newer Ubuntu), and `production` as required by Wails to activate the real window implementation.

## CLI examples

```bash
cockpit todo add "Review MR !1245" -p high -t backend
cockpit todo list
cockpit todo done abc123              # last 6 chars of the ID
cockpit todo rm abc123
cockpit stop
```

## Filesystem

- Database: `~/.local/share/spk-cockpit/cockpit.db`
- Socket:   `~/.local/state/spk-cockpit/cockpit.sock`
- Logs:     `~/.local/state/spk-cockpit/log/cockpit.log`

Override paths via `SPK_COCKPIT_DATA_DIR`, `SPK_COCKPIT_STATE_DIR`, `SPK_COCKPIT_CONFIG_DIR`.

## Development

```bash
make test        # Go + Vitest
make lint        # golangci-lint + eslint
make fmt
```

For frontend hot-reload, run `pnpm dev` from `web/` and point the daemon at the same UDS — the daemon already serves the embedded build, but during development you can run vite separately.
