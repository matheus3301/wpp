package ui

// MenuHint describes a keyboard shortcut for display in the menu bar.
type MenuHint struct {
	Key         string
	Description string
	Numeric     bool // true for 0-9 shortcuts (displayed in a different color)
}

// Component is the lifecycle interface for all TUI views.
type Component interface {
	Name() string
	Init()
	Start()
	Stop()
	Hints() []MenuHint
}
