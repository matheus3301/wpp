# WPP-TUI Stack Policy (v1)

## 1. Purpose
This document defines the canonical v1 technology stack for WPP-TUI and the boundaries for how each library is used.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [DECISIONS](./DECISIONS.md)

## 2. Stack Principles
- Use the smallest set of proven libraries that reinforce the product pillars (speed, clarity, flow, confidence).
- Keep clear ownership boundaries between UI, daemon runtime, API transport, protocol integration, and persistence.
- Prefer explicit operational behavior over implicit framework magic.
- Add dependencies only when they reduce long-term complexity.

## 3. Core Runtime and Language Policy
- Primary language: Go.
- Go baseline version: 1.26 (must stay aligned with `go.mod` and `mise.toml`).
- Repository module baseline: `github.com/matheus3301/wpp`.
- Runtime scope:
  - `wppd` (daemon): session runtime, sync, storage, and local API server.
  - `wpptui` (TUI): presentation and user interaction, no direct DB access.
  - `wppctl` (optional): local operator/debug client.
- Platform target for v1: macOS + Linux.

## 4. Adopted Libraries (v1)
| Layer | Library | Status | Scope | Rationale | Notes |
|---|---|---|---|---|---|
| Language/Runtime | Go | Adopted | All binaries | Fast compilation, single-binary distribution, strong tooling | Keep implementation portable and operationally simple |
| Daemon lifecycle/DI | `go.uber.org/fx` | Adopted | `wppd` only | Standardized lifecycle orchestration and dependency wiring | Do not use in `wpptui` v1 |
| Logging | `go.uber.org/zap` | Adopted | `wppd` primary, optional `wppctl` | Structured, performant logging with explicit fields | Log message content only when explicitly needed for debug |
| Local API transport | `google.golang.org/grpc` | Adopted | `wppd` server, `wpptui`/`wppctl` clients | Typed contracts + streaming support over local socket | Private local API pre-1.0 |
| API schema | `google.golang.org/protobuf` | Adopted | API contracts and generated types | Stable contract definition and generation workflow | Field-level proto is defined later in implementation |
| WhatsApp protocol | `go.mau.fi/whatsmeow` | Adopted | `wppd` integration layer only | Production-grade MD protocol support | Treat protocol/session store schema as external-owned |
| Persistence driver | `github.com/mattn/go-sqlite3` | Adopted | `wppd` store layer | Local-first storage and predictable ops model | v1 persistence split: `session.db` + `wpp.db` |
| TUI runtime | `github.com/gdamore/tcell/v2` | Adopted | `wpptui` only | Terminal control, input handling, rendering primitives | Foundation for keyboard-first UX |
| TUI components | `github.com/rivo/tview` | Adopted | `wpptui` only | Productive layout and widget composition with tcell | Keep custom widgets focused on domain needs |
| Schema migration | `github.com/golang-migrate/migrate/v4` | Adopted | `wppd` store layer | Embedded SQL migrations for `wpp.db` schema versioning | Migrations embedded via `embed.FS` + `iofs` driver |
| Config format | `github.com/BurntSushi/toml` | Adopted | Shared (config loader) | Minimal TOML parser for `~/.wpp/config.toml` | Low dependency footprint, human-readable config format |

## 5. Usage Boundaries by Component
### `wppd`
- Must use `fx` for application lifecycle composition.
- Must use `zap` for structured operational logs.
- Must own all writes to `session.db` and `wpp.db`.
- Must expose local API through gRPC over Unix domain socket.

### `wpptui`
- Must not use `fx` in v1.
- Uses `tcell` and `tview` for terminal UX.
- Must consume daemon APIs only; no direct SQLite access.
- May use lightweight internal state containers, but no alternate persistence engines.

### `wppctl` (optional)
- Uses gRPC client contracts against local daemon.
- Uses concise text/JSON output for diagnostics and automation.
- No direct DB writes or protocol-layer bypasses.

## 6. Non-Goals / Not Adopted Yet
- No framework sprawl beyond the adopted v1 baseline.
- No remote HTTP API stack in v1.
- No public plugin runtime in v1.
- No alternative DB engine in v1.
- No use of `fx` in `wpptui` v1.

Candidate topics for future evaluation:
- Metrics export stack for production observability.
- Windows-specific IPC/runtime libraries when Windows support is in scope.
- Optional API gateway layers after private API stabilization.

## 7. Review and Change Policy
- Stack changes must be recorded in [DECISIONS](./DECISIONS.md) before broad adoption.
- If a stack change affects architecture boundaries, update [ARCHITECTURE](./ARCHITECTURE.md) in the same PR.
- If a stack change impacts product scope or user promises, update [SPEC](./SPEC.md) in the same PR.
- Any new dependency should document:
  - Scope of use
  - Expected removal cost
  - Operational impact
  - Security and maintenance considerations
