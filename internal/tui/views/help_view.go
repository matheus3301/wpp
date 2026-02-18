package views

import (
	"fmt"

	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// HelpView displays key binding reference.
type HelpView struct {
	*tview.TextView
	theme *ui.Theme
}

// NewHelpView creates a new help view.
func NewHelpView(theme *ui.Theme) *HelpView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true)
	tv.SetBorderColor(theme.BorderColor)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetTextColor(theme.FgColor)
	tv.SetTitle(" Help ")
	tv.SetTitleColor(theme.TitleColor)

	hv := &HelpView{
		TextView: tv,
		theme:    theme,
	}
	hv.render()
	return hv
}

// Name implements Component.
func (hv *HelpView) Name() string { return "Help" }

// Init implements Component.
func (hv *HelpView) Init() {}

// Start implements Component.
func (hv *HelpView) Start() {}

// Stop implements Component.
func (hv *HelpView) Stop() {}

// Hints implements Component.
func (hv *HelpView) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "Esc", Description: "Back"},
	}
}

func (hv *HelpView) render() {
	keyColor := ui.DefaultTheme().MenuKeyColor
	kc := fmt.Sprintf("#%06x", keyColor.Hex())

	help := fmt.Sprintf(`
  [::b]Global Keys[-:-:-]

  [%s]:[    -:-:-] Command mode       [%s]Esc[-:-:-]   Cancel / Go back
  [%s]/[-:-:-]    Filter mode         [%s]?[-:-:-]     Help
  [%s]q[-:-:-]    Quit / Back         [%s]Ctrl-C[-:-:-] Quit immediately

  [::b]Conversation List[-:-:-]

  [%s]Enter[-:-:-]  Open conversation  [%s]0[-:-:-]     Show all (clear filter)
  [%s]1-9[-:-:-]    Jump to Nth chat   [%s]s[-:-:-]     Cycle sort mode
  [%s]j/Down[-:-:-] Move down          [%s]k/Up[-:-:-]  Move up

  [::b]Message Thread[-:-:-]

  [%s]i[-:-:-]    Focus composer      [%s]d[-:-:-]     Show conversation details
  [%s]Esc[-:-:-]  Exit composer       [%s]Enter[-:-:-] Send message (in composer)

  [::b]Commands (: mode)[-:-:-]

  [%s]:search <query>[-:-:-]    Search messages
  [%s]:chat <name>[-:-:-]       Open chat by name
  [%s]:logout[-:-:-]            Logout current session
  [%s]:help[-:-:-] / [%s]:h[-:-:-]       Show this help
  [%s]:quit[-:-:-] / [%s]:q[-:-:-]       Quit application
`,
		kc, kc, kc, kc, kc, kc,
		kc, kc, kc, kc, kc, kc,
		kc, kc, kc, kc,
		kc, kc, kc, kc, kc, kc, kc,
	)

	_, _ = fmt.Fprint(hv, help)
}
