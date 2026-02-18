package ui

import "github.com/rivo/tview"

// Pages is a stack-based page manager wrapping tview.Pages.
// It provides push/pop semantics and notifies on stack changes.
type Pages struct {
	*tview.Pages
	stack    []string
	onChange func(stack []string)
}

// NewPages creates a new stack-based page manager.
func NewPages() *Pages {
	return &Pages{
		Pages: tview.NewPages(),
	}
}

// SetOnChange sets a callback that fires when the stack changes.
func (p *Pages) SetOnChange(fn func(stack []string)) {
	p.onChange = fn
}

// Push adds a page to the top of the stack and shows it.
func (p *Pages) Push(name string) {
	// Hide current page.
	if len(p.stack) > 0 {
		p.HidePage(p.stack[len(p.stack)-1])
	}
	p.stack = append(p.stack, name)
	p.ShowPage(name)
	p.SendToFront(name)
	p.notify()
}

// Pop removes the top page and shows the previous one.
// Returns the name of the popped page, or empty if stack is empty.
func (p *Pages) Pop() string {
	if len(p.stack) == 0 {
		return ""
	}
	top := p.stack[len(p.stack)-1]
	p.HidePage(top)
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 {
		current := p.stack[len(p.stack)-1]
		p.ShowPage(current)
		p.SendToFront(current)
	}
	p.notify()
	return top
}

// Current returns the name of the current (top) page.
func (p *Pages) Current() string {
	if len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

// Stack returns a copy of the current page stack.
func (p *Pages) Stack() []string {
	s := make([]string, len(p.stack))
	copy(s, p.stack)
	return s
}

// Depth returns the current stack depth.
func (p *Pages) Depth() int {
	return len(p.stack)
}

// Reset clears the stack and shows only the given page.
func (p *Pages) Reset(name string) {
	for _, n := range p.stack {
		p.HidePage(n)
	}
	p.stack = []string{name}
	p.ShowPage(name)
	p.SendToFront(name)
	p.notify()
}

func (p *Pages) notify() {
	if p.onChange != nil {
		p.onChange(p.Stack())
	}
}
