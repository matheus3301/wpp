# WPP-TUI Operations Runbook (v1 Local Contributor Scope)

## 1. Purpose and Scope
This runbook covers local developer/contributor operations for v1 workflows.

In scope:
- Local startup and session selection.
- Runtime inspection and troubleshooting.
- Safe recovery/reset procedures.

Out of scope:
- Production deployment architecture.
- Cloud-native infrastructure patterns.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [API](./API.md)
- [DECISIONS](./DECISIONS.md)

## 2. Prerequisites
- Supported host: macOS or Linux.
- Working Go toolchain aligned with repo tooling policy.
- Access to local filesystem under `~/.wpp/`.
- Ability to run first-party binaries (`wppd`, `wpptui`, optional `wppctl`) once implemented.
- Session name selected by:
  - `--session <name>` override, or
  - `default_session` in config.

## 3. Session Selection and Runtime Layout
Session resolution precedence:
1. CLI override `--session <name>`.
2. `default_session` in `~/.wpp/config.toml`.
3. First-run default session `main`.

Per-session runtime layout:
- `~/.wpp/sessions/<session>/session.db`
- `~/.wpp/sessions/<session>/wpp.db`
- `~/.wpp/sessions/<session>/daemon.sock`
- `~/.wpp/sessions/<session>/LOCK`
- `~/.wpp/sessions/<session>/logs/wppd.log`

Isolation rule:
- Never share runtime files between sessions.

## 4. Startup Flows (`wpptui`, `wppd`, optional `wppctl`)
### 4.1 Preferred Daily Flow (`wpptui`)
1. Launch `wpptui` with optional `--session`.
2. `wpptui` resolves session.
3. `wpptui` probes daemon health for resolved session.
4. If daemon is missing/unhealthy, `wpptui` auto-starts `wppd`.
5. UI subscribes to status and message event streams.

### 4.2 Manual Daemon Flow (`wppd`)
1. Launch `wppd --session <name>`.
2. Daemon acquires session lock.
3. Daemon initializes stores and socket.
4. Daemon serves local API.

### 4.3 Operator/Debug Flow (`wppctl`)
1. Run `wppctl --session <name> <command>`.
2. Query status/auth/sync or perform safe control actions.
3. Use alongside logs for diagnosis.

## 5. Log and Status Inspection
Primary log source:
- `~/.wpp/sessions/<session>/logs/wppd.log`

Inspection goals:
- Confirm session selected matches intent.
- Confirm daemon state transitions (`BOOTING`, `CONNECTING`, `READY`, etc.).
- Identify last error code and correlated event context.

Recommended status inspection workflow:
1. Check session status through API/client.
2. Check sync status and last error.
3. Inspect recent daemon logs.
4. Apply troubleshooting matrix below.

## 6. Troubleshooting Matrix
| Symptom | Likely Cause | Checks | Recovery |
|---|---|---|---|
| Daemon fails to start with lock error | Lock contention (another daemon active) | Verify owning process for session lock | Stop conflicting process or switch session |
| Connection fails but socket file exists | Stale socket after crash | Health probe fails on existing socket | Remove stale socket after process validation, then restart daemon |
| UI shows auth required unexpectedly | Session invalidated or expired | Check session/auth status and recent auth events | Re-run auth flow for target session |
| Reconnect loops with no stable `READY` | Network instability or protocol-side issue | Review sync lifecycle events and reconnect logs | Keep backoff loop active; inspect connectivity and auth validity |
| Search is degraded/slow | DB busy or FTS unavailable path | Check DB busy errors and search-mode logs | Retry after contention clears; keep degraded fallback behavior |
| UI loses updates after daemon crash | Stream/subscription interruption | Confirm daemon process restarted and stream reconnected | Auto-restart daemon and re-subscribe streams for same session |

## 7. Safe Recovery and Reset Procedures
### 7.1 Safe Daemon Restart (Per Session)
1. Identify target session.
2. Stop daemon process for that session.
3. Confirm lock release and socket cleanup.
4. Start daemon (or relaunch `wpptui` to auto-start).
5. Verify status transitions to `READY` or `AUTH_REQUIRED`.

### 7.2 Auth Re-Initialization (Per Session)
1. Confirm current state is `AUTH_REQUIRED` or invalid auth.
2. Start auth flow for same session.
3. Confirm QR flow completion and authenticated event.
4. Confirm sync status progression.

### 7.3 Session Data Reset (Last Resort)
Use only when session state is unrecoverable.
1. Stop daemon for target session.
2. Backup session directory before deletion.
3. Remove only target session directory artifacts.
4. Recreate session through normal startup + auth flow.

## 8. Device Identity
- WPP-TUI registers as `WPP-TUI` in WhatsApp's linked devices list (via `store.SetOSInfo`).
- Re-linking is required after changing the device name (delete linked device on phone, scan QR again).

## 9. Security Hygiene Checklist
- Use strict file permissions (`0700` directories, `0600` files/sockets).
- Avoid sharing session directories between users/machines without explicit secure transfer.
- Keep sensitive identifiers and message content out of routine logs.
- Treat `session.db` as sensitive credential material.
- Use OS-level disk encryption where possible.

## 10. Operational Anti-Patterns
- Running multiple daemons for the same session intentionally.
- Manually editing SQLite files while daemon is active.
- Reusing one session directory for multiple identities.
- Ignoring repeated degraded/reconnect signals in logs.
- Treating `.references` examples as operational truth for this project.
