package views

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// AuthView displays the QR code for authentication.
type AuthView struct {
	*tview.TextView
	theme *ui.Theme
}

// NewAuthView creates a new auth view.
func NewAuthView(theme *ui.Theme) *AuthView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	tv.SetBorder(true)
	tv.SetBorderColor(theme.BorderColor)
	tv.SetBackgroundColor(theme.BgColor)
	tv.SetTextColor(theme.FgColor)
	tv.SetTitle(" Authentication Required ")
	tv.SetTitleColor(theme.TitleColor)

	return &AuthView{
		TextView: tv,
		theme:    theme,
	}
}

// Name implements Component.
func (av *AuthView) Name() string { return "Auth" }

// Init implements Component.
func (av *AuthView) Init() {}

// Start implements Component.
func (av *AuthView) Start() {}

// Stop implements Component.
func (av *AuthView) Stop() {}

// Hints implements Component.
func (av *AuthView) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "Esc", Description: "Back"},
	}
}

// ShowQR renders a QR code string as a scannable ASCII art block.
func (av *AuthView) ShowQR(content string) {
	av.Clear()

	ascii := renderQR(content)
	_, _ = fmt.Fprintf(av, "\n  Scan this QR code with WhatsApp:\n\n%s\n  [::d]Waiting for authentication...", ascii)
}

// ShowMessage displays a status message.
func (av *AuthView) ShowMessage(msg string) {
	av.Clear()
	_, _ = fmt.Fprintf(av, "\n\n%s", msg)
}

// renderQR converts a string to a compact ASCII QR code using Unicode
// half-block characters.
func renderQR(content string) string {
	qr, err := qrcode.New(content, qrcode.Low)
	if err != nil {
		return "  (QR generation failed: " + err.Error() + ")"
	}
	qr.DisableBorder = false

	bitmap := qr.Bitmap()
	rows := len(bitmap)
	cols := 0
	if rows > 0 {
		cols = len(bitmap[0])
	}

	var sb strings.Builder

	for y := 0; y < rows; y += 2 {
		sb.WriteString("  ")
		for x := 0; x < cols; x++ {
			top := bitmap[y][x]
			bot := false
			if y+1 < rows {
				bot = bitmap[y+1][x]
			}
			switch {
			case top && bot:
				sb.WriteRune('\u2588') // █
			case top && !bot:
				sb.WriteRune('\u2580') // ▀
			case !top && bot:
				sb.WriteRune('\u2584') // ▄
			default:
				sb.WriteRune(' ')
			}
		}
		sb.WriteRune('\n')
	}

	return sb.String()
}
