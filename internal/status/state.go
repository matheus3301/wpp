package status

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
)

// State represents a daemon runtime state.
type State string

const (
	Booting      State = "BOOTING"
	AuthRequired State = "AUTH_REQUIRED"
	Connecting   State = "CONNECTING"
	Syncing      State = "SYNCING"
	Ready        State = "READY"
	Reconnecting State = "RECONNECTING"
	Degraded     State = "DEGRADED"
	Error        State = "ERROR"
)

// validTransitions defines allowed state transitions.
var validTransitions = map[State][]State{
	Booting:      {AuthRequired, Connecting, Error},
	AuthRequired: {Connecting, Error},
	Connecting:   {Syncing, AuthRequired, Reconnecting, Error},
	Syncing:      {Ready, Reconnecting, Degraded, Error},
	Ready:        {Reconnecting, Degraded, AuthRequired, Error},
	Reconnecting: {Connecting, Degraded, Error},
	Degraded:     {Connecting, Reconnecting, Ready, Error},
	Error:        {Booting},
}

// Machine tracks and enforces daemon runtime state transitions.
type Machine struct {
	mu      sync.RWMutex
	current State
	bus     *bus.Bus
}

// NewMachine creates a new state machine starting in Booting state.
func NewMachine(b *bus.Bus) *Machine {
	return &Machine{
		current: Booting,
		bus:     b,
	}
}

// Current returns the current state.
func (m *Machine) Current() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Transition attempts to move to a new state. Returns error if transition is invalid.
func (m *Machine) Transition(to State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	allowed := validTransitions[m.current]
	if !slices.Contains(allowed, to) {
		return fmt.Errorf("invalid transition from %s to %s", m.current, to)
	}
	from := m.current
	m.current = to
	if m.bus != nil {
		m.bus.Publish(bus.Event{
			Kind:      "session.status_changed",
			Timestamp: time.Now(),
			Payload: StatusChange{
				From: from,
				To:   to,
			},
		})
	}
	return nil
}

// StatusChange is the payload for status change events.
type StatusChange struct {
	From State
	To   State
}
