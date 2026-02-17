package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Composer is the text input for sending messages.
type Composer struct {
	*tview.InputField
	onSend func(text string)
}

// NewComposer creates a new message composer.
func NewComposer() *Composer {
	input := tview.NewInputField().
		SetLabel(" > ").
		SetFieldWidth(0)

	c := &Composer{InputField: input}

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && c.onSend != nil {
			text := c.GetText()
			if text != "" {
				c.onSend(text)
				c.SetText("")
			}
		}
	})

	return c
}

// SetOnSend sets the callback when a message is sent.
func (c *Composer) SetOnSend(fn func(text string)) {
	c.onSend = fn
}
