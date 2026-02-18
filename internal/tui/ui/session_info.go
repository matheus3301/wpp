package ui

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// SessionData holds session information for display.
type SessionData struct {
	Session      string
	Phone        string
	Status       string
	ChatCount    int32
	MessageCount int32
	Uptime       time.Duration
}

// SessionInfo displays session metadata in the header.
type SessionInfo struct {
	*tview.TextView
	theme *Theme
}

// NewSessionInfo creates a new session info panel.
func NewSessionInfo(theme *Theme) *SessionInfo {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetBorderPadding(0, 0, 1, 1)

	return &SessionInfo{
		TextView: tv,
		theme:    theme,
	}
}

// Update renders the session info.
func (si *SessionInfo) Update(data *SessionData) {
	si.Clear()
	if data == nil {
		return
	}

	fgColor := colorName(si.theme.FgColor)
	counterColor := colorName(si.theme.CounterColor)

	phone := data.Phone
	if phone == "" {
		phone = "-"
	}

	uptime := formatDuration(data.Uptime)

	text := fmt.Sprintf(
		"[%s::b]Session:[-:-:-] [%s]%s[-]\n"+
			"[%s::b]Phone:[-:-:-]   [%s]%s[-]\n"+
			"[%s::b]Status:[-:-:-]  [%s]%s[-]\n"+
			"[%s::b]Chats:[-:-:-]   [%s]%d[-]\n"+
			"[%s::b]Msgs:[-:-:-]    [%s]%d[-]\n"+
			"[%s::b]Uptime:[-:-:-]  [%s]%s[-]",
		fgColor, counterColor, data.Session,
		fgColor, counterColor, phone,
		fgColor, counterColor, data.Status,
		fgColor, counterColor, data.ChatCount,
		fgColor, counterColor, data.MessageCount,
		fgColor, counterColor, uptime,
	)

	_, _ = fmt.Fprint(si, text)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
