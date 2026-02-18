package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/rivo/tview"
)

// FlashLevel represents the severity of a flash message.
type FlashLevel int

const (
	FlashInfo FlashLevel = iota
	FlashWarn
	FlashErr
)

// FlashMessage is a flash notification with a level and expiry.
type FlashMessage struct {
	Text    string
	Level   FlashLevel
	Expires time.Time
}

// FlashModel holds transient notification messages with levels.
type FlashModel struct {
	mu      sync.RWMutex
	current FlashMessage
	watchCh chan FlashMessage
}

// NewFlashModel creates a new flash model.
func NewFlashModel() *FlashModel {
	return &FlashModel{
		watchCh: make(chan FlashMessage, 8),
	}
}

// Info sets an info-level flash message.
func (f *FlashModel) Info(msg string) {
	f.set(msg, FlashInfo, 5*time.Second)
}

// Warn sets a warn-level flash message.
func (f *FlashModel) Warn(msg string) {
	f.set(msg, FlashWarn, 8*time.Second)
}

// Err sets an error-level flash message.
func (f *FlashModel) Err(err error) {
	f.set(err.Error(), FlashErr, 10*time.Second)
}

// Set sets a flash message with a specific duration (info level).
func (f *FlashModel) Set(msg string, d time.Duration) {
	f.set(msg, FlashInfo, d)
}

func (f *FlashModel) set(msg string, level FlashLevel, d time.Duration) {
	fm := FlashMessage{
		Text:    msg,
		Level:   level,
		Expires: time.Now().Add(d),
	}
	f.mu.Lock()
	f.current = fm
	f.mu.Unlock()
	select {
	case f.watchCh <- fm:
	default:
	}
}

// Get returns the current flash message text, or empty if expired.
func (f *FlashModel) Get() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if time.Now().After(f.current.Expires) {
		return ""
	}
	return f.current.Text
}

// GetMessage returns the current flash message, or nil if expired.
func (f *FlashModel) GetMessage() *FlashMessage {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if time.Now().After(f.current.Expires) {
		return nil
	}
	m := f.current
	return &m
}

// Watch returns a channel that receives flash messages.
func (f *FlashModel) Watch() <-chan FlashMessage {
	return f.watchCh
}

// FlashBar is the UI component that displays flash notifications.
type FlashBar struct {
	*tview.TextView
	theme *Theme
}

// NewFlashBar creates a new flash notification bar.
func NewFlashBar(theme *Theme) *FlashBar {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgColor)

	return &FlashBar{
		TextView: tv,
		theme:    theme,
	}
}

// Update renders a flash message on the bar.
func (fb *FlashBar) Update(msg *FlashMessage) {
	fb.Clear()
	if msg == nil {
		return
	}

	var color string
	switch msg.Level {
	case FlashInfo:
		color = colorName(fb.theme.FlashInfoColor)
	case FlashWarn:
		color = colorName(fb.theme.FlashWarnColor)
	case FlashErr:
		color = colorName(fb.theme.FlashErrColor)
	}
	_, _ = fmt.Fprintf(fb, " [%s]%s[-]", color, msg.Text)
}
