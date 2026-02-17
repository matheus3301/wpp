package keys

import "github.com/gdamore/tcell/v2"

// Action represents a keybinding action.
type Action struct {
	Key         tcell.Key
	Rune        rune
	Description string
	Handler     func()
	Visible     bool
}

// Matches returns true if the event matches this action.
func (a *Action) Matches(ev *tcell.EventKey) bool {
	if a.Key != tcell.KeyRune {
		return ev.Key() == a.Key
	}
	return ev.Key() == tcell.KeyRune && ev.Rune() == a.Rune
}

// Registry holds keybindings organized by scope.
type Registry struct {
	Global map[string]*Action
	Views  map[string]map[string]*Action
}

// NewRegistry creates a new keybinding registry.
func NewRegistry() *Registry {
	return &Registry{
		Global: make(map[string]*Action),
		Views:  make(map[string]map[string]*Action),
	}
}

// AddGlobal registers a global keybinding.
func (r *Registry) AddGlobal(name string, action *Action) {
	r.Global[name] = action
}

// AddView registers a view-specific keybinding.
func (r *Registry) AddView(view, name string, action *Action) {
	if r.Views[view] == nil {
		r.Views[view] = make(map[string]*Action)
	}
	r.Views[view][name] = action
}

// Hints returns visible keybinding descriptions for a given view.
func (r *Registry) Hints(view string) []string {
	var hints []string
	for _, a := range r.Global {
		if a.Visible {
			hints = append(hints, a.Description)
		}
	}
	if viewBindings, ok := r.Views[view]; ok {
		for _, a := range viewBindings {
			if a.Visible {
				hints = append(hints, a.Description)
			}
		}
	}
	return hints
}

// HandleEvent dispatches a key event to matching action in the given view.
// Returns true if a handler matched.
func (r *Registry) HandleEvent(view string, ev *tcell.EventKey) bool {
	// Check view-specific bindings first.
	if viewBindings, ok := r.Views[view]; ok {
		for _, a := range viewBindings {
			if a.Matches(ev) {
				a.Handler()
				return true
			}
		}
	}
	// Check global bindings.
	for _, a := range r.Global {
		if a.Matches(ev) {
			a.Handler()
			return true
		}
	}
	return false
}
