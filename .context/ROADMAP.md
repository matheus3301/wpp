# WPP-TUI Roadmap (Phase-Based)

## 1. Purpose and Planning Horizon
This roadmap defines the delivery path from current documentation baseline to a v1 daily-driver messaging experience.

The roadmap is phase-based (not calendar-based) and uses explicit exit criteria to gate progression.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [DECISIONS](./DECISIONS.md)
- [API](./API.md)
- [OPERATIONS](./OPERATIONS.md)

## 2. Phase Model
Rules for all phases:
- Each phase has one primary goal.
- Exit criteria are mandatory gates.
- If scope decisions change, update [DECISIONS](./DECISIONS.md) in the same change set.
- If architecture contracts change, update [ARCHITECTURE](./ARCHITECTURE.md) and [API](./API.md) in the same change set.

## 3. Phase 0: Documentation and Contract Baseline
### Goal
Establish decision-complete documentation so contributors can implement v1 without re-deciding core boundaries.

### Deliverables
- Canonical product and architecture docs maintained:
  - [SPEC](./SPEC.md)
  - [ARCHITECTURE](./ARCHITECTURE.md)
- Core operations/context docs created:
  - [STACK](./STACK.md)
  - [DECISIONS](./DECISIONS.md)
  - [API](./API.md)
  - [OPERATIONS](./OPERATIONS.md)
  - [AGENTS](../AGENTS.md)

### Exit Criteria
- All core docs exist and cross-link correctly.
- No unresolved architecture/tooling decision blocks daemon/TUI/API implementation.
- Canonical terminology (`session`) is consistent across docs.

### Risks
- Documentation inconsistency between files.
- Missing contract details that force re-design during implementation.

### Dependencies
- Agreement on v1 scope and architecture boundaries.

## 4. Phase 1: Daemon Foundation
### Goal
Implement `wppd` runtime foundation with session isolation and local API skeleton.

### Deliverables
- Session resolution (`default_session` + `--session` override).
- One daemon per session with lock management.
- gRPC server bootstrap over Unix socket.
- `fx` lifecycle module skeleton and `zap` logger initialization.
- Basic `SessionService` and `SyncService` status endpoints.

### Exit Criteria
- Daemon starts reliably per session and exposes health/status.
- Socket/lock lifecycle is deterministic across restart scenarios.
- Structured logs are emitted with stable core fields.

### Risks
- Lock and stale socket edge cases.
- Overcoupling startup logic to provisional APIs.

### Dependencies
- Decisions D-0002, D-0003, D-0006, D-0007.

## 5. Phase 2: Sync + Storage Reliability
### Goal
Implement robust message sync ingestion with idempotent persistence model.

### Deliverables
- WhatsApp adapter connection flow and auth state transitions.
- Ingestion pipeline for history and live message events.
- App-owned schema/migrations for `wpp.db`.
- Session store separation with `session.db`.
- Sync lifecycle events and reconnect handling with backoff.

### Exit Criteria
- Replayed history and reconnect events do not produce duplicate message rows.
- Sync state is observable through local API.
- Failure modes (disconnect, auth invalidation, DB busy) surface deterministic statuses.

### Risks
- Protocol event variability causing parser instability.
- SQLite contention under high event throughput.

### Dependencies
- Phase 1 daemon baseline.
- Decisions D-0004 and D-0005.

## 6. Phase 3: TUI Core Daily Driver
### Goal
Deliver the keyboard-first core UX: browse, search, and send text through daemon APIs.

### Deliverables
- `wpptui` startup with daemon auto-start behavior.
- Chat list and message view consumption from API.
- Search flow and result navigation.
- Text composer and send action with ack/failure handling.
- Persistent session/sync status indicators in UI.

### Exit Criteria
- Core flows work end-to-end for a returning session:
  - read chats/messages
  - search messages
  - send text
- UI uses daemon API exclusively (no direct DB access).
- Degraded states are visible and recoverable from UI.

### Risks
- UI state race conditions from stream reconnection.
- Keyboard interaction complexity eroding usability.

### Dependencies
- Phase 2 API/event stability for core flows.

## 7. Phase 4: Hardening and DX
### Goal
Improve contributor and operator confidence through diagnostics, docs, and quality improvements.

### Deliverables
- `wppctl` operator/debug command set for status and recovery.
- Operational runbook refinements in [OPERATIONS](./OPERATIONS.md).
- Test harness coverage for critical runtime flows.
- Documentation refresh based on implementation learnings.

### Exit Criteria
- Contributors can diagnose common failure cases using docs + `wppctl`.
- Core workflows remain stable under repeated restart/disconnect scenarios.
- Documentation remains aligned with behavior (no known contract drift).

### Risks
- Test strategy lagging behind system complexity.
- Operational guidance becoming stale after implementation iteration.

### Dependencies
- Stable outcomes from Phases 1-3.

## 8. Deferred/Post-v1 Themes
- Full WhatsApp feature parity.
- Broad media workflows as core product promise.
- Advanced group administration.
- Public third-party API guarantees.
- Windows IPC/runtime model.

If deferred scope is pulled forward, corresponding updates are required in:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [DECISIONS](./DECISIONS.md)

## 9. Risk Register
| Risk | Impact | Mitigation | Owner Area |
|---|---|---|---|
| Session lifecycle edge cases | Startup/recovery failures | Enforce lock/socket policy and failure matrix in ops docs | Daemon runtime |
| Protocol event variability | Sync instability | Normalize events through adapter and idempotent write path | Sync engine |
| Contract drift across docs/code | Contributor confusion | Spec-first doc sync rule (D-0009) | Project-wide |
| Stream reconnect complexity | UI inconsistency | Define deterministic event semantics and dedupe strategy | API + TUI |
| Scope creep beyond v1 | Delayed daily-driver value | Strict deferred list and phase exit gates | Product + architecture |
