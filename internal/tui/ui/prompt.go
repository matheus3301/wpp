package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PromptMode indicates the type of prompt (command or filter).
type PromptMode int

const (
	PromptCommand PromptMode = iota
	PromptFilter
)

// Prompt is a command/filter input bar.
type Prompt struct {
	*tview.InputField
	theme    *Theme
	mode     PromptMode
	onSubmit func(mode PromptMode, text string)
	onCancel func()
}

// NewPrompt creates a new prompt input bar.
func NewPrompt(theme *Theme) *Prompt {
	input := tview.NewInputField()
	input.SetBorder(true)
	input.SetBorderColor(theme.PromptBorderColor)
	input.SetBackgroundColor(theme.BgColor)
	input.SetFieldBackgroundColor(theme.BgColor)
	input.SetFieldTextColor(theme.FgColor)
	input.SetLabelColor(theme.MenuKeyColor)

	p := &Prompt{
		InputField: input,
		theme:      theme,
	}

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			text := p.GetText()
			if p.onSubmit != nil && text != "" {
				p.onSubmit(p.mode, text)
			}
			p.SetText("")
		case tcell.KeyEscape:
			p.SetText("")
			if p.onCancel != nil {
				p.onCancel()
			}
		}
	})

	return p
}

// SetOnSubmit sets the callback when the prompt is submitted.
func (p *Prompt) SetOnSubmit(fn func(mode PromptMode, text string)) {
	p.onSubmit = fn
}

// SetOnCancel sets the callback when the prompt is cancelled.
func (p *Prompt) SetOnCancel(fn func()) {
	p.onCancel = fn
}

// Activate shows the prompt in the specified mode.
func (p *Prompt) Activate(mode PromptMode) {
	p.mode = mode
	p.SetText("")
	switch mode {
	case PromptCommand:
		p.SetLabel(":")
		p.SetTitle(" Command ")
	case PromptFilter:
		p.SetLabel("/")
		p.SetTitle(" Filter ")
	}
}

// Mode returns the current prompt mode.
func (p *Prompt) Mode() PromptMode {
	return p.mode
}
