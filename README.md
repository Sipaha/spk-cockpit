# spk-cockpit

Personal productivity tray app — todo list with prioritization, filtering, history, and a single-binary architecture (Go + embedded React UI). Linux first.

## Status

### Phase 1 ✅
- Todo CRUD (priority, status, due, tags, audit history)
- Tray icon with menu (Open window / Quit)
- Wails main window
- CLI: `cockpit start | stop | todo add/list/done/rm`
- SQLite storage with migrations
- HTTP/UDS server with SSE for realtime UI updates

### Phase 2 ✅
- Time-tracking on todos with one-active-globally invariant
- TimerBadge component (live elapsed counter)
- Quick-add inline syntax (`!priority #tag due:tomorrow`) with live preview
- Compact `/popover` route showing today / active timer / quick add
- CLI: `cockpit timer start | stop | status`
- Tray tooltip reflects active timer

### Phase 3 ✅
- Read-only sync of meetings from Yandex Calendar (CalDAV)
- AES-256-GCM-encrypted secrets backed by OS keyring
- DBus system notifications fired N minutes before each meeting (default 5, per-meeting override)
- Markdown notes attached to meetings
- Calendar page (Today / Tomorrow / Later)
- Settings page (CalDAV credentials, default notify_min, sync now)
- CLI: `cockpit meeting next | list`, `cockpit secret set | list`
- Tray tooltip surfaces next meeting (within 24h)

### Phase 4 ✅
- Standup helper aggregates "Yesterday / Today / Blockers" from closed todos, GitLab commits, and Citeck Project Tracker activity.
- Web `/standup` page with "Copy as markdown" button.
- CLI: `cockpit standup [--date YYYY-MM-DD]` prints the same markdown to stdout.
- Read-only GitLab integration: configure `gitlab.url` + `gitlab.author_username` via KV, store `gitlab_token` as a secret.
- Read-only Citeck PT integration: configure `tracker.url` + `tracker.username`, store `tracker_token` as a secret.
- `cockpit install --autostart` installs `~/.config/systemd/user/spk-cockpit.service` and enables it (`--uninstall` to remove).
- GitHub Actions release workflow on `v*.*.*` tags builds and publishes a `linux/amd64` binary.

#### Configuring GitLab and Tracker

```bash
# GitLab
cockpit secret set gitlab_token < /dev/stdin     # paste a personal access token
curl --unix-socket ~/.local/state/spk-cockpit/cockpit.sock \
     -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "https://gitlab.example.com"}' http://unix/api/kv/gitlab.url
curl --unix-socket ~/.local/state/spk-cockpit/cockpit.sock \
     -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "alice"}' http://unix/api/kv/gitlab.author_username

# Citeck PT — same pattern with tracker.url, tracker.username, tracker_token.
```

## Build

System dependencies (Ubuntu/Debian):

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-1-dev
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
cockpit timer start abc123             # timer on the todo whose id ends with abc123
cockpit timer status                   # see what's running
cockpit timer stop                     # stop the active timer
cockpit meeting list -d 14                # next 14 days
cockpit meeting next                      # closest upcoming
cockpit secret set yandex_caldav          # reads value from stdin
echo "my-app-password" | cockpit secret set yandex_caldav
cockpit standup                           # markdown for today
cockpit standup --date 2026-04-26         # markdown for a specific day
cockpit install --autostart               # systemd-user unit; --uninstall to remove
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
