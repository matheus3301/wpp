package views

import (
	"fmt"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/rivo/tview"
)

// ChatList is the main chat list view (K9s-inspired table).
type ChatList struct {
	*tview.Table
	chats      []*wppv1.Chat
	onSelect   func(jid string)
	onSearch   func()
	selectedFn func() (int, int)
}

// NewChatList creates a new chat list table.
func NewChatList() *ChatList {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false)
	table.SetBorder(true).SetTitle(" Chats ")

	cl := &ChatList{Table: table}
	cl.selectedFn = table.GetSelection
	return cl
}

// SetOnSelect sets the callback when a chat is selected.
func (cl *ChatList) SetOnSelect(fn func(jid string)) {
	cl.onSelect = fn
}

// SetOnSearch sets the callback when search is triggered.
func (cl *ChatList) SetOnSearch(fn func()) {
	cl.onSearch = fn
}

// Update refreshes the chat list with new data.
func (cl *ChatList) Update(chats []*wppv1.Chat) {
	cl.chats = chats
	cl.Clear()

	// Header row.
	cl.SetCell(0, 0, tview.NewTableCell(" Name").SetSelectable(false).SetTextColor(tview.Styles.SecondaryTextColor))
	cl.SetCell(0, 1, tview.NewTableCell(" Last Message").SetSelectable(false).SetTextColor(tview.Styles.SecondaryTextColor))
	cl.SetCell(0, 2, tview.NewTableCell(" Time").SetSelectable(false).SetTextColor(tview.Styles.SecondaryTextColor))

	for i, chat := range chats {
		row := i + 1
		name := chat.Name
		if name == "" {
			name = chat.Jid
		}
		if chat.UnreadCount > 0 {
			name = fmt.Sprintf("* %s (%d)", name, chat.UnreadCount)
		}

		cl.SetCell(row, 0, tview.NewTableCell(" "+name).SetMaxWidth(30).SetExpansion(1))
		cl.SetCell(row, 1, tview.NewTableCell(" "+chat.LastMessagePreview).SetMaxWidth(40).SetExpansion(2))
		cl.SetCell(row, 2, tview.NewTableCell(" "+formatTimestamp(chat.LastMessageAtUnixMs)).SetMaxWidth(12))
	}
}

// SelectedChat returns the JID of the currently selected chat.
func (cl *ChatList) SelectedChat() string {
	row, _ := cl.selectedFn()
	idx := row - 1 // account for header
	if idx >= 0 && idx < len(cl.chats) {
		return cl.chats[idx].Jid
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
