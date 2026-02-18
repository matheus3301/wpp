package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

// Logo displays a compact ASCII art logo.
type Logo struct {
	*tview.TextView
	theme *Theme
}

// NewLogo creates a new logo component.
func NewLogo(theme *Theme) *Logo {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetBorderPadding(1, 0, 1, 0)

	l := &Logo{
		TextView: tv,
		theme:    theme,
	}
	l.render()
	return l
}

func (l *Logo) render() {
	titleColor := colorName(l.theme.TitleColor)
	fgColor := colorName(l.theme.FgColor)

	_, _ = fmt.Fprintf(l,
		"[%s::b] ╦ ╦╔═╗╔═╗[-:-:-]\n"+
			"[%s::b] ║║║╠═╝╠═╝[-:-:-]\n"+
			"[%s::b] ╚╩╝╩  ╩[-:-:-]\n"+
			"[%s]Terminal UI[-:-:-]",
		titleColor, titleColor, titleColor, fgColor,
	)
}
