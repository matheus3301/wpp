package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// MessageThread displays messages and a composer for a single chat.
type MessageThread struct {
	*tview.Flex
	theme    *ui.Theme
	messages *tview.TextView
	composer *tview.InputField
	chatName string
	chatJID  string
	onSend   func(text string)
}

// NewMessageThread creates a new message thread view.
func NewMessageThread(theme *ui.Theme) *MessageThread {
	messages := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	messages.SetBorder(true)
	messages.SetBorderColor(theme.BorderColor)
	messages.SetBackgroundColor(theme.BgColor)
	messages.SetTextColor(theme.FgColor)
	messages.SetTitle(" Messages ")
	messages.SetTitleColor(theme.TitleColor)

	composer := tview.NewInputField().
		SetLabel(" > ").
		SetFieldWidth(0)
	composer.SetBorder(true)
	composer.SetBorderColor(theme.BorderColor)
	composer.SetBackgroundColor(theme.BgColor)
	composer.SetFieldBackgroundColor(theme.BgColor)
	composer.SetFieldTextColor(theme.FgColor)
	composer.SetLabelColor(theme.MenuKeyColor)
	composer.SetTitle(" Compose (i to focus) ")
	composer.SetTitleColor(theme.TitleColor)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(messages, 0, 1, true).
		AddItem(composer, 3, 0, false)

	mt := &MessageThread{
		Flex:     flex,
		theme:    theme,
		messages: messages,
		composer: composer,
	}

	composer.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && mt.onSend != nil {
			text := composer.GetText()
			if text != "" {
				mt.onSend(text)
				composer.SetText("")
			}
		}
	})

	return mt
}

// Name implements Component.
func (mt *MessageThread) Name() string {
	if mt.chatName != "" {
		return mt.chatName
	}
	return "Messages"
}

// Init implements Component.
func (mt *MessageThread) Init() {}

// Start implements Component.
func (mt *MessageThread) Start() {}

// Stop implements Component.
func (mt *MessageThread) Stop() {}

// Hints implements Component.
func (mt *MessageThread) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "i", Description: "Compose"},
		{Key: "d", Description: "Details"},
		{Key: "Esc", Description: "Back"},
		{Key: ":", Description: "Command"},
		{Key: "?", Description: "Help"},
	}
}

// SetChatName updates the chat name and title.
func (mt *MessageThread) SetChatName(name string) {
	mt.chatName = name
	mt.messages.SetTitle(fmt.Sprintf(" %s ", name))
}

// SetChatJID stores the current chat JID.
func (mt *MessageThread) SetChatJID(jid string) {
	mt.chatJID = jid
}

// ChatJID returns the current chat JID.
func (mt *MessageThread) ChatJID() string {
	return mt.chatJID
}

// SetOnSend sets the callback when a message is sent.
func (mt *MessageThread) SetOnSend(fn func(text string)) {
	mt.onSend = fn
}

// Update refreshes the message view with new messages.
func (mt *MessageThread) Update(msgs []*wppv1.Message) {
	mt.messages.Clear()

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
		line := fmt.Sprintf("[::b]%s[-:-:-] [::d]%s[-:-:-]\n%s\n\n",
			tview.Escape(sanitizeForTerminal(sender)), ts,
			tview.Escape(sanitizeForTerminal(m.Body)))
		_, _ = fmt.Fprint(mt.messages, line)
	}

	mt.messages.ScrollToEnd()
}

// Messages returns the messages text view (for focus management).
func (mt *MessageThread) Messages() *tview.TextView {
	return mt.messages
}

// Composer returns the composer input field (for focus management).
func (mt *MessageThread) Composer() *tview.InputField {
	return mt.composer
}
