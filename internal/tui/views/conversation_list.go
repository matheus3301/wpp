package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// ConversationList is the main chat list view.
type ConversationList struct {
	*tview.Table
	theme  *ui.Theme
	chats  []*wppv1.Chat
	filter string
}

// NewConversationList creates a new conversation list table.
func NewConversationList(theme *ui.Theme) *ConversationList {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false).
		SetFixed(1, 0)
	table.SetBorder(true)
	table.SetBorderColor(theme.BorderColor)
	table.SetBackgroundColor(theme.BgColor)
	table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(theme.TableCursorFg).
		Background(theme.TableCursorBg))
	table.SetTitle(" Conversations ")
	table.SetTitleColor(theme.TitleColor)

	cl := &ConversationList{
		Table: table,
		theme: theme,
	}
	return cl
}

// Name implements Component.
func (cl *ConversationList) Name() string { return "Conversations" }

// Init implements Component.
func (cl *ConversationList) Init() {}

// Start implements Component.
func (cl *ConversationList) Start() {}

// Stop implements Component.
func (cl *ConversationList) Stop() {}

// Hints implements Component.
func (cl *ConversationList) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "Enter", Description: "Open"},
		{Key: "/", Description: "Filter"},
		{Key: ":", Description: "Command"},
		{Key: "s", Description: "Sort"},
		{Key: "?", Description: "Help"},
		{Key: "q", Description: "Quit"},
		{Key: "0-9", Description: "Jump", Numeric: true},
	}
}

// Update refreshes the chat list with new data.
func (cl *ConversationList) Update(chats []*wppv1.Chat) {
	cl.chats = chats
	cl.render()
}

// SetFilter sets the active filter text and re-renders.
func (cl *ConversationList) SetFilter(filter string) {
	cl.filter = filter
	cl.render()
}

// ClearFilter clears the active filter.
func (cl *ConversationList) ClearFilter() {
	cl.filter = ""
	cl.render()
}

func (cl *ConversationList) render() {
	cl.Clear()

	// Header row.
	headers := []struct {
		text string
		exp  int
	}{
		{" NAME", 1},
		{" LAST MESSAGE", 2},
		{" TIME", 0},
		{" TYPE", 0},
	}
	for col, h := range headers {
		cell := tview.NewTableCell(h.text).
			SetSelectable(false).
			SetTextColor(cl.theme.TableHeaderFg).
			SetBackgroundColor(cl.theme.TableHeaderBg).
			SetAttributes(tcell.AttrBold).
			SetExpansion(h.exp)
		cl.SetCell(0, col, cell)
	}

	row := 1
	for _, chat := range cl.chats {
		name := chat.Name
		if name == "" {
			name = chat.Jid
		}

		// Apply filter.
		if cl.filter != "" && !containsFold(name, cl.filter) && !containsFold(chat.LastMessagePreview, cl.filter) {
			continue
		}

		// Show unread badge in name.
		if chat.UnreadCount > 0 {
			name = fmt.Sprintf("(%d) %s", chat.UnreadCount, name)
		}

		chatType := "DM"
		if chat.IsGroup {
			chatType = "GROUP"
		}

		cl.SetCell(row, 0, tview.NewTableCell(" "+tview.Escape(sanitizeForTerminal(name))).SetExpansion(1).SetTextColor(cl.theme.FgColor))
		cl.SetCell(row, 1, tview.NewTableCell(" "+tview.Escape(sanitizeForTerminal(chat.LastMessagePreview))).SetExpansion(2).SetTextColor(cl.theme.FgColor))
		cl.SetCell(row, 2, tview.NewTableCell(formatTimestamp(chat.LastMessageAtUnixMs)).SetExpansion(0).SetTextColor(cl.theme.FgColor).SetAlign(tview.AlignRight))
		cl.SetCell(row, 3, tview.NewTableCell(chatType).SetExpansion(0).SetTextColor(cl.theme.FgColor).SetAlign(tview.AlignRight))
		row++
	}

	// Update title with count.
	if cl.filter != "" {
		cl.SetTitle(fmt.Sprintf(" Conversations (%d/%d) filter: %s ", row-1, len(cl.chats), cl.filter))
	} else {
		cl.SetTitle(fmt.Sprintf(" Conversations (%d) ", len(cl.chats)))
	}
}

// SelectedChat returns the JID of the currently selected chat.
func (cl *ConversationList) SelectedChat() string {
	row, _ := cl.GetSelection()
	idx := row - 1 // account for header
	if idx < 0 {
		return ""
	}

	// If filtered, we need to find the actual chat.
	visible := 0
	for _, chat := range cl.chats {
		name := chat.Name
		if name == "" {
			name = chat.Jid
		}
		if cl.filter != "" && !containsFold(name, cl.filter) && !containsFold(chat.LastMessagePreview, cl.filter) {
			continue
		}
		if visible == idx {
			return chat.Jid
		}
		visible++
	}
	return ""
}

// ChatByIndex returns the JID of the Nth visible conversation (1-based).
func (cl *ConversationList) ChatByIndex(n int) string {
	if n < 1 {
		return ""
	}
	visible := 0
	for _, chat := range cl.chats {
		name := chat.Name
		if name == "" {
			name = chat.Jid
		}
		if cl.filter != "" && !containsFold(name, cl.filter) && !containsFold(chat.LastMessagePreview, cl.filter) {
			continue
		}
		visible++
		if visible == n {
			return chat.Jid
		}
	}
	return ""
}

func formatTimestamp(ms int64) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("01/02")
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" ||
			len(s) == 0 ||
			tview.Escape(s) != "" && // ensure non-empty
				containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
