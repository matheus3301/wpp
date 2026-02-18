package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Crumbs is a breadcrumb bar showing the current navigation path.
type Crumbs struct {
	*tview.TextView
	theme *Theme
}

// NewCrumbs creates a new breadcrumb bar.
func NewCrumbs(theme *Theme) *Crumbs {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgColor)

	return &Crumbs{
		TextView: tv,
		theme:    theme,
	}
}

// Update renders the breadcrumb trail from the page stack.
func (c *Crumbs) Update(stack []string) {
	c.Clear()
	if len(stack) == 0 {
		return
	}

	var parts []string
	for i, name := range stack {
		if i == len(stack)-1 {
			// Active crumb.
			parts = append(parts, fmt.Sprintf("[%s:%s:b] %s [-:-:-]",
				colorName(c.theme.CrumbActiveFg), colorName(c.theme.CrumbActiveBg), name))
		} else {
			// Inactive crumb.
			parts = append(parts, fmt.Sprintf("[%s:%s:] %s [-:-:-]",
				colorName(c.theme.CrumbInactiveFg), colorName(c.theme.CrumbInactiveBg), name))
		}
	}
	_, _ = fmt.Fprint(c, strings.Join(parts, " > "))
}

// colorName returns a tview-compatible color name string.
func colorName(c tcell.Color) string {
	for name, val := range tcell.ColorNames {
		if val == c {
			return name
		}
	}
	return fmt.Sprintf("#%06x", c.Hex())
}
