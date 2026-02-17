# WPP-TUI Decisions Log (ADR-lite)

## 1. Purpose
This file records architecture and process decisions that shape implementation behavior across WPP-TUI.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [STACK](./STACK.md)
- [API](./API.md)
- [ROADMAP](./ROADMAP.md)
- [OPERATIONS](./OPERATIONS.md)

## 2. Decision Status Legend
- `Proposed`: Candidate decision not yet adopted.
- `Accepted`: Active decision and current source of truth.
- `Superseded`: Replaced by a newer accepted decision.

## 3. ADR-lite Template
Use this template for each new decision entry.

```markdown
### D-XXXX - <short title>
- Date: YYYY-MM-DD
- Status: Proposed | Accepted | Superseded
- Context:
  <why this decision is needed>
- Decision:
  <what we decided>
- Consequences:
  - <positive outcome>
  - <tradeoff or cost>
- Related Docs:
  - <link(s)>
```

## 4. Decision Index
| ID | Title | Status | Date |
|---|---|---|---|
| D-0001 | Canonical identity term is `session` | Accepted | 2026-02-16 |
| D-0002 | One daemon and one data directory per session | Accepted | 2026-02-16 |
| D-0003 | Local API transport is gRPC over Unix domain socket | Accepted | 2026-02-16 |
| D-0004 | API remains private and unstable pre-1.0 | Accepted | 2026-02-16 |
| D-0005 | Persistence split into `session.db` and `wpp.db` | Accepted | 2026-02-16 |
| D-0006 | Adopt `fx` for daemon lifecycle and DI only | Accepted | 2026-02-16 |
| D-0007 | Adopt `zap` for daemon structured logging | Accepted | 2026-02-16 |
| D-0008 | `.references` is read-only inspiration, no direct copy | Accepted | 2026-02-16 |
| D-0009 | Spec-first sync rule for markdown docs | Accepted | 2026-02-16 |
| D-0010 | Testing workflow is mandatory for code changes | Accepted | 2026-02-16 |
| D-0011 | Adopt `golang-migrate` for embedded SQL migrations | Accepted | 2026-02-16 |
| D-0012 | Adopt `BurntSushi/toml` for config format | Accepted | 2026-02-16 |

## 5. Active Decisions
### D-0001 - Canonical identity term is `session`
- Date: 2026-02-16
- Status: Accepted
- Context:
  Multiple terms (`account`, `context`, `session`) were being used for the same concept, which increases implementation ambiguity.
- Decision:
  Use `session` as the canonical term in all project-owned documentation and interfaces.
- Consequences:
  - Improves naming consistency across docs and APIs.
  - Existing references to alternate terms should be normalized unless quoting external sources.
- Related Docs:
  - [SPEC](./SPEC.md)
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [API](./API.md)

### D-0002 - One daemon and one data directory per session
- Date: 2026-02-16
- Status: Accepted
- Context:
  Multi-session support needs strong isolation to avoid state leakage and runtime coupling.
- Decision:
  Run one `wppd` process per session and isolate runtime artifacts in a session-specific directory.
- Consequences:
  - Predictable process and storage boundaries.
  - Requires explicit session resolution and daemon management logic.
- Related Docs:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [OPERATIONS](./OPERATIONS.md)

### D-0003 - Local API transport is gRPC over Unix domain socket
- Date: 2026-02-16
- Status: Accepted
- Context:
  The UI needs typed contracts plus stream-oriented updates with low local overhead.
- Decision:
  Use gRPC over Unix domain sockets as the only v1 IPC transport for first-party clients.
- Consequences:
  - Strong contracts and streaming support for session/sync/message updates.
  - Windows-specific IPC adaptation is deferred post-v1.
- Related Docs:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [API](./API.md)

### D-0004 - API remains private and unstable pre-1.0
- Date: 2026-02-16
- Status: Accepted
- Context:
  Early implementation requires fast iteration without long-term compatibility constraints.
- Decision:
  Treat all local API contracts as private pre-1.0 and reserve the right to make breaking changes.
- Consequences:
  - Faster architecture evolution while building v1.
  - Requires coordinated first-party client updates on contract changes.
- Related Docs:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [API](./API.md)

### D-0005 - Persistence split into `session.db` and `wpp.db`
- Date: 2026-02-16
- Status: Accepted
- Context:
  Protocol/session internals and app query model have different ownership and migration needs.
- Decision:
  Use two SQLite files per session:
  - `session.db` for WhatsApp protocol/session internals.
  - `wpp.db` for app-owned query model and indexes.
- Consequences:
  - Cleaner ownership and safer migration boundaries.
  - Slightly more operational artifacts to manage.
- Related Docs:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [OPERATIONS](./OPERATIONS.md)

### D-0006 - Adopt `fx` for daemon lifecycle and DI only
- Date: 2026-02-16
- Status: Accepted
- Context:
  Daemon startup/shutdown composition and dependency graph coordination need consistent structure.
- Decision:
  Adopt `go.uber.org/fx` for `wppd` lifecycle and dependency injection only.
- Consequences:
  - Standardized lifecycle hooks and module wiring for backend runtime.
  - `wpptui` keeps a simpler architecture and does not use `fx` in v1.
- Related Docs:
  - [STACK](./STACK.md)
  - [ARCHITECTURE](./ARCHITECTURE.md)

### D-0007 - Adopt `zap` for daemon structured logging
- Date: 2026-02-16
- Status: Accepted
- Context:
  Operational diagnostics require structured logs with stable fields and low overhead.
- Decision:
  Use `go.uber.org/zap` as the canonical daemon logging library.
- Consequences:
  - Better log searchability and troubleshooting workflows.
  - Enforces structured logging conventions and field hygiene.
- Related Docs:
  - [STACK](./STACK.md)
  - [OPERATIONS](./OPERATIONS.md)

### D-0008 - `.references` is read-only inspiration, no direct copy
- Date: 2026-02-16
- Status: Accepted
- Context:
  `.references` contains external projects used for pattern research and should remain a clean snapshot.
- Decision:
  Treat `.references` as strict read-only research material and do not copy code directly into project-owned implementation.
- Consequences:
  - Clear provenance and maintainability boundaries.
  - Contributors must re-implement concepts in project-native architecture.
- Related Docs:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [AGENTS](../AGENTS.md)

### D-0009 - Spec-first sync rule for markdown docs
- Date: 2026-02-16
- Status: Accepted
- Context:
  Architecture and API docs can drift if contributors update code plans without updating context docs.
- Decision:
  Any change that affects architecture/API/session model must update corresponding `.context` docs in the same change set.
- Consequences:
  - Reduces doc drift and onboarding confusion.
  - Adds disciplined documentation requirements to feature work.
- Related Docs:
  - [SPEC](./SPEC.md)
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [AGENTS](../AGENTS.md)

### D-0010 - Testing workflow is mandatory for code changes
- Date: 2026-02-16
- Status: Accepted
- Context:
  Rapid iteration can degrade quality unless every code change runs a consistent local validation gate.
- Decision:
  Enforce a mandatory local command gate for code changes via:
  - `make lint`
  - `make test`
  and define canonical testing policy in [TESTING](./TESTING.md).
- Consequences:
  - Stronger quality and consistency before merge.
  - Slightly longer feedback loops for contributors and agents.
- Related Docs:
  - [TESTING](./TESTING.md)
  - [AGENTS](../AGENTS.md)
  - [ROADMAP](./ROADMAP.md)

### D-0011 - Adopt `golang-migrate` for embedded SQL migrations
- Date: 2026-02-16
- Status: Accepted
- Context:
  `wpp.db` needs versioned schema management that runs at daemon startup with migrations embedded in the binary.
- Decision:
  Use `github.com/golang-migrate/migrate/v4` with `embed.FS` and the `iofs` source driver for embedded SQL migrations.
- Consequences:
  - Schema changes are versioned, auditable, and embedded in the daemon binary.
  - Adds a dependency but avoids hand-rolled migration logic.
- Related Docs:
  - [STACK](./STACK.md)
  - [ARCHITECTURE](./ARCHITECTURE.md)

### D-0012 - Adopt `BurntSushi/toml` for config format
- Date: 2026-02-16
- Status: Accepted
- Context:
  Global config (`~/.wpp/config.toml`) needs a parser. TOML is human-readable and widely used in Go CLI tools.
- Decision:
  Use `github.com/BurntSushi/toml` for reading and writing the global config file.
- Consequences:
  - Simple, well-maintained dependency for config parsing.
  - Config format is TOML, consistent with Go ecosystem conventions.
- Related Docs:
  - [STACK](./STACK.md)
