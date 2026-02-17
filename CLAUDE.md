# WPP-TUI Agent Guidelines

## 1. Purpose and Scope
This file defines how AI agents and contributors should work with project markdown documentation and the `.references` folder.

Scope:
- Documentation and planning workflow under `.context`.
- Safe usage of external reference material in `.references`.
- Required sync behavior when architecture/API/session decisions change.

## 2. Source-of-Truth Order
When docs conflict, follow this precedence:
1. [SPEC](.context/SPEC.md)
2. [ARCHITECTURE](.context/ARCHITECTURE.md)
3. Specialized docs in this order:
   - [API](.context/API.md)
   - [STACK](.context/STACK.md)
   - [OPERATIONS](.context/OPERATIONS.md)
   - [ROADMAP](.context/ROADMAP.md)
   - [DECISIONS](.context/DECISIONS.md)

If a conflict indicates stale documentation, update affected docs in the same change set.

## 3. Working with `.context` Markdown
- `.context` is the canonical planning/documentation directory.
- New planning docs belong in `.context` unless explicitly requested otherwise.
- Keep terminology canonical: use `session`, not `account` or `context`, unless quoting external text.
- Keep documents concise, actionable, and cross-linked.
- Use project-relative paths in docs and links (for example: `.context/SPEC.md`).
- Every new `.context` doc should link back to:
  - [SPEC](.context/SPEC.md)
  - [ARCHITECTURE](.context/ARCHITECTURE.md)

## 4. Working with `.references` (Strict Read-Only)
- `.references` is strict read-only material.
- Never modify, rename, delete, or reformat files under `.references`.
- `.references` is for pattern research and rationale only.
- Do not directly copy code from `.references` into project implementation.
- Re-implement ideas in project-native structure and conventions.

## 5. Documentation Sync Rules
- Spec-first sync rule is mandatory:
  - Any change affecting architecture, API contracts, session model, runtime topology, or stack boundaries must update relevant `.context` docs in the same PR/change set.
- Minimum sync expectations:
  - Architecture boundary change -> update `ARCHITECTURE.md`, `API.md`, `DECISIONS.md`
  - Stack/library policy change -> update `STACK.md`, `DECISIONS.md`
  - Operational behavior change -> update `OPERATIONS.md`
  - Product scope/value change -> update `SPEC.md` and dependent docs

## 6. Code Quality Gate for Code Changes
- For code changes, contributors and agents must run:
  - `make lint`
  - `make test`
- If these commands fail, the task must not be treated as complete.
- If a change is docs-only, this command gate is optional.
- If commands are skipped due no Go packages, confirm that skip output was expected.

## 7. Change Classification and Required Doc Updates
| Change Type | Required Doc Updates |
|---|---|
| Session model or naming change | `SPEC.md`, `ARCHITECTURE.md`, `API.md`, `DECISIONS.md` |
| IPC/API transport or service contract change | `ARCHITECTURE.md`, `API.md`, `DECISIONS.md`, `OPERATIONS.md` |
| Stack/library adoption/removal | `STACK.md`, `DECISIONS.md`, possibly `ARCHITECTURE.md` |
| Runtime failure/recovery behavior changes | `ARCHITECTURE.md`, `OPERATIONS.md`, `API.md` |
| Roadmap or phase scope changes | `ROADMAP.md`, `DECISIONS.md`, possibly `SPEC.md` |

## 8. Writing Standards for New `.md` Files
- Use clear section headers and short paragraphs.
- Prefer explicit contracts, responsibilities, and boundaries over narrative ambiguity.
- Use tables for matrices (errors, phases, mappings, decision indexes).
- Keep implementation code snippets minimal unless specifically required.
- Link related docs directly with relative links when possible.
- Keep docs ASCII unless existing file requires Unicode.

## 9. Prohibited Actions
- Editing anything inside `.references`.
- Copy-pasting external reference code into project code/docs as authoritative implementation.
- Introducing alternate canonical terminology when `session` is the project standard.
- Changing architecture/API/session behavior without updating `.context` docs.
- Creating new planning docs outside `.context` without explicit request.

## 10. Quick Checklist Before Finishing a Task
- Did I keep `.references` untouched?
- Did I use `session` terminology consistently?
- Did I update all impacted `.context` docs for any architecture/API/session changes?
- For code changes: did `make lint` pass?
- For code changes: did `make test` pass?
- If commands were skipped because no packages exist, was that skip expected?
- Did I keep source-of-truth precedence intact?
- Did I add/update links between related docs?
- Are decisions recorded in [DECISIONS](.context/DECISIONS.md) when policy changed?
