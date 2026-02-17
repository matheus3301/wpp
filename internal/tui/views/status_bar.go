package views

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// StatusBar displays persistent session/sync status.
type StatusBar struct {
	*tview.TextView
	session string
	status  string
	syncing bool
	flash   string
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(tview.Styles.MoreContrastBackgroundColor)

	return &StatusBar{TextView: tv}
}

// SetSession updates the session name display.
func (sb *StatusBar) SetSession(name string) {
	sb.session = name
	sb.render()
}

// SetStatus updates the status display.
func (sb *StatusBar) SetStatus(status string) {
	sb.status = status
	sb.render()
}

// SetSyncing updates the sync indicator.
func (sb *StatusBar) SetSyncing(syncing bool) {
	sb.syncing = syncing
	sb.render()
}

// SetFlash sets a temporary message.
func (sb *StatusBar) SetFlash(msg string) {
	sb.flash = msg
	sb.render()
}

func (sb *StatusBar) render() {
	sb.Clear()

	syncIcon := " "
	if sb.syncing {
		syncIcon = "[green]~[-]"
	}

	clock := time.Now().Format("15:04")

	line := fmt.Sprintf(" [::b]%s[-:-:-] | %s %s | %s", sb.session, sb.status, syncIcon, clock)
	if sb.flash != "" {
		line += fmt.Sprintf(" | [yellow]%s[-]", sb.flash)
	}

	_, _ = fmt.Fprint(sb, line)
}
