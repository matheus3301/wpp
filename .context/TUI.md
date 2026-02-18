# WPP-TUI Terminal Interface Specification

## 1. Purpose and Principles
WPP-TUI adopts a k9s-inspired, keyboard-first design where conversations are treated as navigable namespaces. The TUI should feel like a power-user productivity tool, not a toy terminal app.

Design principles:
- **Keyboard-first**: Every core flow is faster with keys than pointer interaction.
- **Stack navigation**: Push/pop views like a browser history. `Enter` opens, `Esc` goes back.
- **Command mode**: `:` activates a command prompt for actions, `/` activates a filter prompt.
- **Visual hierarchy**: Header, content, breadcrumbs, and flash notifications are always visible.
- **Single dark theme**: k9s-inspired color palette, no skin system in v1.

Related docs:
- [SPEC](./SPEC.md)
- [ARCHITECTURE](./ARCHITECTURE.md)
- [STACK](./STACK.md)
- [DECISIONS](./DECISIONS.md)

## 2. Layout Architecture

```
+--------------------------------------------------------------+
| HEADER (7 rows fixed, horizontal flex)                       |
|  [SessionInfo 50w] [Menu/Shortcuts flex] [Logo 26w]          |
+--------------------------------------------------------------+
| PROMPT (3 rows, dynamically shown for : or / mode)           |
+--------------------------------------------------------------+
| CONTENT (PageStack, flex fill)                                |
|  Active view: ConversationList / MessageThread / etc          |
+--------------------------------------------------------------+
| CRUMBS (1 row, breadcrumb trail)                              |
+--------------------------------------------------------------+
| FLASH (1 row, transient notifications)                        |
+--------------------------------------------------------------+
```

Layout rules:
- Header is always visible (7 rows). Contains session info (left), shortcut hints (center), logo (right).
- Prompt row is hidden by default, appears when `:` or `/` is pressed.
- Content area fills remaining space, managed by a `Pages` stack.
- Crumbs bar shows the current navigation path (1 row).
- Flash bar shows transient notifications (1 row), auto-clears.

## 3. Component System

Two-layer split:
- `internal/tui/ui/` — Reusable, domain-agnostic primitives (theme, pages, crumbs, flash, prompt, menu, etc.)
- `internal/tui/views/` — Domain-specific views that compose UI primitives (ConversationList, MessageThread, etc.)

### Component Interface
Every view implements:
```go
type Component interface {
    Name() string
    Init()
    Start()
    Stop()
    Hints() []MenuHint
}
```

`Hints()` returns shortcut descriptions for the menu bar, auto-updated when the active view changes.

### UI Primitives

| File | Component | Purpose |
|---|---|---|
| `theme.go` | `Theme` | Color constants, `DefaultTheme()` returns k9s-inspired dark palette |
| `component.go` | `Component` interface | Standard lifecycle for all views |
| `pages.go` | `Pages` | Stack-based page manager (push/pop) wrapping `tview.Pages` |
| `crumbs.go` | `Crumbs` | Breadcrumb bar (1 row), listens to page stack changes |
| `flash.go` | `Flash` | Notification bar (1 row), three levels: info/warn/err |
| `prompt.go` | `Prompt` | Command/filter input (3 rows), dynamically shown/hidden |
| `menu.go` | `Menu` | Shortcut hints grid, updates from current view's `Hints()` |
| `session_info.go` | `SessionInfo` | Header left panel: Session, Phone, Status, Synced, Uptime |
| `logo.go` | `Logo` | ASCII art "WPP" logo, 26 chars wide |
| `key.go` | Key constants | Named key bindings for readability |
| `action.go` | `KeyAction` | Action registry with thread-safe map |

## 4. Views Catalog

| View | File | Replaces | Purpose |
|---|---|---|---|
| ConversationList | `conversation_list.go` | `chat_list.go` | Table: NAME, LAST MSG, TIME, UNREAD, TYPE. Filterable, sortable. |
| MessageThread | `message_thread.go` | `message_view.go` + `composer.go` | Messages + inline composer. `i` enters insert mode, `Esc` exits. |
| ConversationInfo | `conversation_info.go` | *(new)* | Detail view: Name, JID, Type, Unread, Last Active |
| Search | `search_view.go` | `search.go` | FTS results table: CHAT, SNIPPET, TIME. Enter navigates to message. |
| Auth | `auth_view.go` | `auth.go` | QR code flow, implements Component interface |
| Help | `help_view.go` | *(new)* | Key binding reference, three-column layout |

## 5. Navigation and Key Bindings

### Global Keys (not in input mode)
| Key | Action |
|---|---|
| `:` | Activate command mode |
| `/` | Activate filter mode |
| `?` | Push help view |
| `q` | Pop view stack (quit at root) |
| `Esc` | Cancel prompt / pop stack / exit insert mode |
| `Ctrl-C` | Quit immediately |

### Table Navigation
| Key | Action |
|---|---|
| `j` / `Down` | Move selection down |
| `k` / `Up` | Move selection up |
| `Enter` | Open selected item (push view) |

### ConversationList Keys
| Key | Action |
|---|---|
| `0` | Clear filter (show all) |
| `1-9` | Jump to Nth conversation |
| `s` | Cycle sort mode |

### MessageThread Keys
| Key | Action |
|---|---|
| `i` | Focus composer (enter insert mode) |
| `d` | Push ConversationInfo view |
| `Esc` | Exit composer / pop view |

## 6. Command Mode Spec

`:` opens the prompt bar. Available commands:

| Command | Aliases | Action |
|---|---|---|
| `:search <query>` | `:s` | Push search view with query |
| `:chat <name>` | `:c` | Open conversation by name match |
| `:logout` | | Logout current session |
| `:help` | `:h` | Push help view |
| `:quit` | `:q` | Quit application |

`/` opens filter mode. Typed text filters the current view in real-time. `Esc` clears filter and closes prompt.

## 7. Visual Theme

Single dark theme based on k9s defaults. No skin system in v1.

| Element | Color |
|---|---|
| Background | black |
| Primary text | cadetblue |
| Borders (unfocused) | dodgerblue |
| Borders (focused) | lightskyblue |
| Table headers | white on black, bold |
| Table cursor row | black on aqua |
| Crumbs (inactive) | black on aqua |
| Crumbs (active) | black on orange |
| Menu keys | dodgerblue |
| Numeric shortcut keys | fuchsia |
| Title highlight | fuchsia |
| Counter badge | papayawhip |
| Flash info | navajowhite |
| Flash warn | orange |
| Flash err | orangered |

## 8. Backend Requirements

### Contact Name Resolution
Store queries JOIN the contacts table for display name fallback:
```
COALESCE(NULLIF(chat.name,''), contact.push_name, contact.name, chat.jid)
```

### Session Status Extension
`GetSessionStatusResponse` extended with:
- `phone_number` (string)
- `chat_count` (int32)
- `message_count` (int32)

These fields populate the SessionInfo header component.

## 9. Cross-References
- Product scope and principles: [SPEC](./SPEC.md)
- Runtime topology and component architecture: [ARCHITECTURE](./ARCHITECTURE.md)
- Technology choices: [STACK](./STACK.md)
- Decision log: [DECISIONS](./DECISIONS.md)
- API contracts: [API](./API.md)
