package ui

import "github.com/gdamore/tcell/v2"

// Theme holds color constants for the TUI.
type Theme struct {
	BgColor           tcell.Color
	FgColor           tcell.Color
	BorderColor       tcell.Color
	BorderFocusColor  tcell.Color
	TableHeaderFg     tcell.Color
	TableHeaderBg     tcell.Color
	TableCursorFg     tcell.Color
	TableCursorBg     tcell.Color
	CrumbActiveFg     tcell.Color
	CrumbActiveBg     tcell.Color
	CrumbInactiveFg   tcell.Color
	CrumbInactiveBg   tcell.Color
	MenuKeyColor      tcell.Color
	NumericKeyColor   tcell.Color
	TitleColor        tcell.Color
	CounterColor      tcell.Color
	FlashInfoColor    tcell.Color
	FlashWarnColor    tcell.Color
	FlashErrColor     tcell.Color
	PromptBorderColor tcell.Color
}

// DefaultTheme returns a k9s-inspired dark theme.
func DefaultTheme() *Theme {
	return &Theme{
		BgColor:           tcell.ColorBlack,
		FgColor:           tcell.ColorCadetBlue,
		BorderColor:       tcell.ColorDodgerBlue,
		BorderFocusColor:  tcell.ColorLightSkyBlue,
		TableHeaderFg:     tcell.ColorWhite,
		TableHeaderBg:     tcell.ColorBlack,
		TableCursorFg:     tcell.ColorBlack,
		TableCursorBg:     tcell.ColorAqua,
		CrumbActiveFg:     tcell.ColorBlack,
		CrumbActiveBg:     tcell.ColorOrange,
		CrumbInactiveFg:   tcell.ColorBlack,
		CrumbInactiveBg:   tcell.ColorAqua,
		MenuKeyColor:      tcell.ColorDodgerBlue,
		NumericKeyColor:   tcell.ColorFuchsia,
		TitleColor:        tcell.ColorFuchsia,
		CounterColor:      tcell.ColorPapayaWhip,
		FlashInfoColor:    tcell.ColorNavajoWhite,
		FlashWarnColor:    tcell.ColorOrange,
		FlashErrColor:     tcell.ColorOrangeRed,
		PromptBorderColor: tcell.ColorDodgerBlue,
	}
}
