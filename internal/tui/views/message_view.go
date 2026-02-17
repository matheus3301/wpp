package views

import (
	"fmt"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/rivo/tview"
)

// MessageView displays messages for a single chat.
type MessageView struct {
	*tview.TextView
	chatName string
}

// NewMessageView creates a new message view.
func NewMessageView() *MessageView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" Messages ")

	return &MessageView{TextView: tv}
}

// SetChatName updates the title with the chat name.
func (mv *MessageView) SetChatName(name string) {
	mv.chatName = name
	mv.SetTitle(fmt.Sprintf(" %s ", name))
}

// Update refreshes the message view with new messages.
func (mv *MessageView) Update(msgs []*wppv1.Message) {
	mv.Clear()

	// Messages come in reverse chronological order; display oldest first.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		sender := m.SenderName
		if sender == "" {
			sender = m.SenderJid
		}
		if m.FromMe {
			sender = "You"
		}

		ts := formatTimestamp(m.TimestampUnixMs)
		line := fmt.Sprintf("[::b]%s[-:-:-] [::d]%s[-:-:-]\n%s\n\n", sender, ts, m.Body)
		_, _ = fmt.Fprint(mv, line)
	}

	mv.ScrollToEnd()
}
