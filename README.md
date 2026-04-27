# spk-cockpit

Personal productivity tray app — todo list, time-tracking, meetings, standup helper. Single Go binary with embedded React UI.

## Status

Phase 1 (foundation + todo). See `docs/superpowers/plans/` for the implementation plan.

## Build

```bash
make build           # full build (Go + React)
make test            # all tests
./build/bin/spk-cockpit start --foreground
```
