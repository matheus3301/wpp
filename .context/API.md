# WPP-TUI Local API Contract Summary (v1)

## 1. Purpose and Stability Notice
This document defines the v1 local API surface and behavior expectations for first-party clients.

Stability notice:
- API contracts are private and unstable pre-1.0.
- This file is intentionally method-level and behavior-focused.
- Field-level proto schema is now defined in `proto/wpp/v1/` with generated code in `gen/wpp/v1/`.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [OPERATIONS](./OPERATIONS.md)
- [DECISIONS](./DECISIONS.md)

## 2. Transport and Session Resolution
Transport:
- gRPC over Unix domain socket (local-only).
- One daemon endpoint per session.

Session resolution precedence for clients:
1. CLI override `--session <name>`.
2. Config `default_session`.
3. First-run fallback initializes default to `main`.

Canonical first-party CLI contracts:
- `wpptui [--session <name>]`
- `wppd --session <name>`
- `wppctl --session <name> <command>`

## 3. Service Contract Summary
### 3.1 `SessionService`
| Method | Responsibility | Input/Output Intent | Side Effects | Stream Semantics |
|---|---|---|---|---|
| `GetSessionStatus` | Return current session runtime/auth state and metadata | Input: current session context; Output: session status snapshot including phone_number, chat_count, message_count | None | Unary |
| `StartAuth` | Initiate QR auth flow for current session | Input: auth start request; Output: stream of auth lifecycle events | May persist credentials to `session.db` on success | Server streaming |
| `Logout` | Invalidate active linked session | Input: logout request; Output: operation result | Clears/invalidate auth state; may trigger sync stop | Unary |
| `ListSessions` | Return known sessions from local registry | Input: optional filters; Output: session descriptors | None | Unary |

### 3.2 `SyncService`
| Method | Responsibility | Input/Output Intent | Side Effects | Stream Semantics |
|---|---|---|---|---|
| `GetSyncStatus` | Return current sync lifecycle and health | Input: current session context; Output: sync status snapshot | None | Unary |
| `StartSync` | Start or resume sync processing | Input: sync mode/options; Output: operation result | Connects to protocol client and starts ingestion | Unary |
| `StopSync` | Stop sync processing gracefully | Input: stop request; Output: operation result | Stops ingestion loops and transitions status | Unary |
| `WatchSyncEvents` | Stream sync lifecycle events | Input: watch request with optional cursor | None | Server streaming |

### 3.3 `ChatService`
| Method | Responsibility | Input/Output Intent | Side Effects | Stream Semantics |
|---|---|---|---|---|
| `ListChats` | Return paginated/filterable chat list | Input: filters/pagination; Output: chat summaries | None | Unary |
| `GetChat` | Return chat details | Input: chat identifier; Output: chat metadata | None | Unary |
| `WatchChatUpdates` | Stream chat metadata changes | Input: watch request with optional cursor | None | Server streaming |

### 3.4 `MessageService`
| Method | Responsibility | Input/Output Intent | Side Effects | Stream Semantics |
|---|---|---|---|---|
| `ListMessages` | Return paginated message history | Input: chat + bounds/pagination; Output: message page | None | Unary |
| `SearchMessages` | Return ranked message matches | Input: query + filters; Output: ranked results | None | Unary |
| `SendText` | Send a text message through daemon pipeline | Input: `client_msg_id`, destination, text; Output: accepted/rejected result | Writes outbox state, triggers protocol send path | Unary |
| `WatchMessageEvents` | Stream message updates and send outcomes | Input: watch request with optional cursor | None | Server streaming |

## 4. Event Contract Summary
Event namespaces:
- `session.*`
- `sync.*`
- `message.*`

### 4.1 Session/Auth Events (`session.*`)
Examples:
- `session.qr_generated`
- `session.authenticated`
- `session.auth_failed`
- `session.logged_out`

Intent:
- Drive explicit auth UX in `wpptui`.
- Surface actionable auth failures in `wppctl`.

### 4.2 Sync Lifecycle Events (`sync.*`)
Examples:
- `sync.connecting`
- `sync.connected`
- `sync.history_batch`
- `sync.reconnecting`
- `sync.disconnected`
- `sync.degraded`

Intent:
- Keep runtime state visible and debuggable.
- Support deterministic UI status transitions.

### 4.3 Message Events (`message.*`)
Examples:
- `message.upserted`
- `message.send_ack`
- `message.send_failed`

Intent:
- Keep message views live without polling.
- Reconcile optimistic send state with server outcomes.

### 4.4 Event Envelope and Delivery
Envelope fields:
- `event_id`
- `session`
- `occurred_at_unix_ms`
- `kind`
- `payload_version`
- `correlation_id` (when applicable)

Delivery expectations:
- In-order delivery per active stream connection.
- At-least-once behavior across reconnects.
- Client deduplication by `event_id`.

## 5. Error and Status Model
Status model categories:
- `BOOTING`
- `AUTH_REQUIRED`
- `CONNECTING`
- `SYNCING`
- `READY`
- `RECONNECTING`
- `DEGRADED`
- `ERROR`

Error model expectations:
- Use structured, machine-readable error codes plus human-readable message.
- Include session context in error payloads.
- Include actionable next-step hints for operator-facing failures.

Representative error classes:
- `LOCK_HELD`
- `STALE_SOCKET`
- `AUTH_INVALID`
- `SYNC_DISCONNECTED`
- `DB_BUSY`
- `REQUEST_INVALID`
- `INTERNAL`

Operational handling guidance is defined in [OPERATIONS](./OPERATIONS.md).

## 6. Compatibility and Versioning Policy
- Pre-1.0: private API, breaking changes allowed.
- Breaking changes require:
  - updates to first-party clients (`wpptui`, `wppctl`) in same change set
  - documentation updates in [ARCHITECTURE](./ARCHITECTURE.md), [API](./API.md), and [DECISIONS](./DECISIONS.md)
- Event kind names should remain stable within short implementation windows; if renamed, include migration notes.

## 7. Contract-to-Architecture Mapping
| API Contract Area | Architecture Reference | Notes |
|---|---|---|
| Session resolution precedence | [ARCHITECTURE: Session Model](./ARCHITECTURE.md#5-session-model) | Canonical `session` term and selection behavior |
| Transport boundary | [ARCHITECTURE: System Context](./ARCHITECTURE.md#2-system-context) | Local-only gRPC over UDS |
| Service families | [ARCHITECTURE: Public Interfaces and Contracts](./ARCHITECTURE.md#7-public-interfaces-and-contracts) | First-party private surface |
| Event families and delivery expectations | [ARCHITECTURE: Public Interfaces and Contracts](./ARCHITECTURE.md#7-public-interfaces-and-contracts) | Session/sync/message namespaces |
| Failure status semantics | [ARCHITECTURE: Reliability and Failure Modes](./ARCHITECTURE.md#10-reliability-and-failure-modes) | Status and failure classes |
