package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

// Menu displays keyboard shortcut hints in a vertical list.
type Menu struct {
	*tview.TextView
	theme *Theme
}

// NewMenu creates a new menu hint bar.
func NewMenu(theme *Theme) *Menu {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetBorderPadding(0, 0, 2, 0)

	return &Menu{
		TextView: tv,
		theme:    theme,
	}
}

// Update renders menu hints as a vertical list (one per line).
func (m *Menu) Update(hints []MenuHint) {
	m.Clear()

	keyColor := colorName(m.theme.MenuKeyColor)
	numColor := colorName(m.theme.NumericKeyColor)

	for _, h := range hints {
		kc := keyColor
		if h.Numeric {
			kc = numColor
		}
		_, _ = fmt.Fprintf(m, "[%s::b]<%s>[-:-:-] %s\n", kc, h.Key, h.Description)
	}
}
