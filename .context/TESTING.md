# WPP-TUI Testing Policy (v1)

## 1. Purpose and Scope
This document defines mandatory testing requirements for WPP-TUI development.

It is the canonical source for testing expectations across daemon, API, and TUI code paths.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [API](./API.md)
- [ROADMAP](./ROADMAP.md)
- [DECISIONS](./DECISIONS.md)

## 2. Mandatory Testing Policy
- Testing is required for every code change.
- A code change is not complete if required testing checks fail.
- Required local command gate for code changes:
  - `make lint`
  - `make test`
- Documentation-only changes are exempt from this command gate.

## 3. Required Test Types
All code-delivery work must include appropriate coverage across these types:

1. Unit tests
- Validate isolated logic (parsers, state transitions, store helpers, utilities).
- Must not depend on external network/services.

2. Integration tests
- Validate component interactions (session resolution, daemon lifecycle, store, API flow).
- Focus on runtime boundaries and failure behavior.

3. Contract tests
- Validate local API service and event contracts.
- Cover service families:
  - `SessionService`
  - `SyncService`
  - `ChatService`
  - `MessageService`
- Cover event namespaces:
  - `session.*`
  - `sync.*`
  - `message.*`

## 4. Coverage Baseline
- Baseline target: 80% coverage per package.
- Coverage exceptions are allowed only when:
  - explicitly documented in PR notes
  - justified with rationale and risk
  - recorded in [DECISIONS](./DECISIONS.md) when recurring/persistent

Coverage baseline is a quality target, not a substitute for required scenario coverage.

## 5. Contract Test Policy
- Golden fixtures are required for contract tests (request/response and stream payload expectations).
- Compatibility checks are required for service and event contracts.
- Breaking behavior changes must include:
  - fixture updates
  - compatibility impact notes
  - synchronized doc updates in `.context`

## 6. Local Execution Workflow
For every code change:
1. Run `make lint`.
2. Run `make test`.
3. Confirm failures are addressed before marking work complete.

Expected early-bootstrap behavior:
- If no Go packages exist yet, `make vet`, `make test`, and package-based lint steps may skip with explicit messages.
- This skip is valid only when expected for current repository state.

## 7. Exceptions and Temporary Waivers
- Waivers are temporary and must include:
  - scope of exception
  - reason
  - risk assessment
  - expiry/removal condition
- Waivers should be minimized and revisited quickly.
- Long-lived exceptions require decision tracking in [DECISIONS](./DECISIONS.md).

## 8. Ownership and Review Expectations
- Authors are responsible for adding and running required tests.
- Reviewers should block code merges when mandatory testing requirements are not met.
- Testing requirements evolve with architecture and API changes; keep this file aligned with:
  - [ARCHITECTURE](./ARCHITECTURE.md)
  - [API](./API.md)
  - [ROADMAP](./ROADMAP.md)
