package views

import (
	"fmt"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// ConversationInfo displays detailed information about a conversation.
type ConversationInfo struct {
	*tview.TextView
	theme *ui.Theme
}

// NewConversationInfo creates a new conversation info view.
func NewConversationInfo(theme *ui.Theme) *ConversationInfo {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBorder(true)
	tv.SetBorderColor(theme.BorderColor)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetTextColor(theme.FgColor)
	tv.SetTitle(" Conversation Details ")
	tv.SetTitleColor(theme.TitleColor)

	return &ConversationInfo{
		TextView: tv,
		theme:    theme,
	}
}

// Name implements Component.
func (ci *ConversationInfo) Name() string { return "Details" }

// Init implements Component.
func (ci *ConversationInfo) Init() {}

// Start implements Component.
func (ci *ConversationInfo) Start() {}

// Stop implements Component.
func (ci *ConversationInfo) Stop() {}

// Hints implements Component.
func (ci *ConversationInfo) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "Esc", Description: "Back"},
		{Key: ":", Description: "Command"},
		{Key: "?", Description: "Help"},
	}
}

// Update renders conversation details.
func (ci *ConversationInfo) Update(chat *wppv1.Chat) {
	ci.Clear()
	if chat == nil {
		return
	}

	fgColor := ui.DefaultTheme().FgColor
	counterColor := ui.DefaultTheme().CounterColor

	fg := colorNameFromTheme(fgColor)
	ct := colorNameFromTheme(counterColor)

	chatType := "Direct Message"
	if chat.IsGroup {
		chatType = "Group"
	}

	lastActive := formatTimestamp(chat.LastMessageAtUnixMs)
	if lastActive == "" {
		lastActive = "-"
	}

	text := fmt.Sprintf(
		"\n [%s::b]Name:[-:-:-]        [%s]%s[-]\n"+
			" [%s::b]JID:[-:-:-]         [%s]%s[-]\n"+
			" [%s::b]Type:[-:-:-]        [%s]%s[-]\n"+
			" [%s::b]Unread:[-:-:-]      [%s]%d[-]\n"+
			" [%s::b]Last Active:[-:-:-] [%s]%s[-]\n"+
			" [%s::b]Last Message:[-:-:-] [%s]%s[-]",
		fg, ct, chat.Name,
		fg, ct, chat.Jid,
		fg, ct, chatType,
		fg, ct, chat.UnreadCount,
		fg, ct, lastActive,
		fg, ct, chat.LastMessagePreview,
	)

	_, _ = fmt.Fprint(ci, text)
	ci.SetTitle(fmt.Sprintf(" %s Details ", chat.Name))
}

func colorNameFromTheme(c interface{ Hex() int32 }) string {
	return fmt.Sprintf("#%06x", c.Hex())
}
