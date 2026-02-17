package status

import (
	"testing"

	"github.com/matheus3301/wpp/internal/bus"
)

func TestInitialState(t *testing.T) {
	m := NewMachine(nil)
	if m.Current() != Booting {
		t.Errorf("initial state = %s, want BOOTING", m.Current())
	}
}

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from State
		to   State
	}{
		{Booting, AuthRequired},
		{Booting, Connecting},
		{Booting, Error},
		{AuthRequired, Connecting},
		{Connecting, Syncing},
		{Syncing, Ready},
		{Ready, Reconnecting},
		{Reconnecting, Connecting},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			m := NewMachine(nil)
			// Walk to the "from" state.
			walkTo(t, m, tt.from)
			if err := m.Transition(tt.to); err != nil {
				t.Errorf("Transition(%s -> %s) error = %v", tt.from, tt.to, err)
			}
			if m.Current() != tt.to {
				t.Errorf("state = %s, want %s", m.Current(), tt.to)
			}
		})
	}
}

func TestInvalidTransition(t *testing.T) {
	m := NewMachine(nil)
	if err := m.Transition(Ready); err == nil {
		t.Error("Transition(BOOTING -> READY) should fail")
	}
}

func TestTransitionEmitsEvent(t *testing.T) {
	b := bus.New()
	ch, unsub := b.Subscribe("session.", 10)
	defer unsub()

	m := NewMachine(b)
	if err := m.Transition(AuthRequired); err != nil {
		t.Fatal(err)
	}

	evt := <-ch
	if evt.Kind != "session.status_changed" {
		t.Errorf("event kind = %q, want session.status_changed", evt.Kind)
	}
	change, ok := evt.Payload.(StatusChange)
	if !ok {
		t.Fatalf("payload type = %T, want StatusChange", evt.Payload)
	}
	if change.From != Booting || change.To != AuthRequired {
		t.Errorf("change = %v -> %v, want BOOTING -> AUTH_REQUIRED", change.From, change.To)
	}
}

// TestAuthToSyncingRequiresConnecting verifies that AUTH_REQUIRED cannot jump
// directly to SYNCING. This was the root cause of the daemon staying stuck on
// AUTH_REQUIRED after QR auth — the Connected event handler tried
// AUTH_REQUIRED→SYNCING which silently failed.
func TestAuthToSyncingRequiresConnecting(t *testing.T) {
	m := NewMachine(nil)
	_ = m.Transition(AuthRequired)

	// Direct AUTH_REQUIRED→SYNCING must fail.
	if err := m.Transition(Syncing); err == nil {
		t.Fatal("Transition(AUTH_REQUIRED -> SYNCING) should fail; must go through CONNECTING first")
	}
	if m.Current() != AuthRequired {
		t.Errorf("state = %s, want AUTH_REQUIRED (should not have changed)", m.Current())
	}

	// Correct path: AUTH_REQUIRED→CONNECTING→SYNCING.
	if err := m.Transition(Connecting); err != nil {
		t.Fatalf("AUTH_REQUIRED -> CONNECTING: %v", err)
	}
	if err := m.Transition(Syncing); err != nil {
		t.Fatalf("CONNECTING -> SYNCING: %v", err)
	}
	if m.Current() != Syncing {
		t.Errorf("state = %s, want SYNCING", m.Current())
	}
}

// TestFullQRAuthLifecycle simulates the complete first-run lifecycle:
// BOOTING → AUTH_REQUIRED → CONNECTING → SYNCING → READY
func TestFullQRAuthLifecycle(t *testing.T) {
	m := NewMachine(nil)

	steps := []State{AuthRequired, Connecting, Syncing, Ready}
	for _, s := range steps {
		if err := m.Transition(s); err != nil {
			t.Fatalf("Transition to %s: %v (current: %s)", s, err, m.Current())
		}
	}
	if m.Current() != Ready {
		t.Errorf("final state = %s, want READY", m.Current())
	}
}

// TestReturningUserLifecycle simulates a returning user who already has credentials:
// BOOTING → CONNECTING → SYNCING → READY
func TestReturningUserLifecycle(t *testing.T) {
	m := NewMachine(nil)

	steps := []State{Connecting, Syncing, Ready}
	for _, s := range steps {
		if err := m.Transition(s); err != nil {
			t.Fatalf("Transition to %s: %v (current: %s)", s, err, m.Current())
		}
	}
	if m.Current() != Ready {
		t.Errorf("final state = %s, want READY", m.Current())
	}
}

// TestDisconnectReconnectCycle verifies the reconnect loop:
// READY → RECONNECTING → CONNECTING → SYNCING → READY
func TestDisconnectReconnectCycle(t *testing.T) {
	m := NewMachine(nil)
	walkTo(t, m, Ready)

	steps := []State{Reconnecting, Connecting, Syncing, Ready}
	for _, s := range steps {
		if err := m.Transition(s); err != nil {
			t.Fatalf("Transition to %s: %v (current: %s)", s, err, m.Current())
		}
	}
	if m.Current() != Ready {
		t.Errorf("final state = %s, want READY", m.Current())
	}
}

// TestLoggedOutFromReady verifies that a logout event from READY
// transitions to AUTH_REQUIRED correctly.
func TestLoggedOutFromReady(t *testing.T) {
	m := NewMachine(nil)
	walkTo(t, m, Ready)

	if err := m.Transition(AuthRequired); err != nil {
		t.Fatalf("READY -> AUTH_REQUIRED: %v", err)
	}
	if m.Current() != AuthRequired {
		t.Errorf("state = %s, want AUTH_REQUIRED", m.Current())
	}
}

// walkTo is a helper that transitions the machine to a target state.
func walkTo(t *testing.T, m *Machine, target State) {
	t.Helper()
	paths := map[State][]State{
		Booting:      {},
		AuthRequired: {AuthRequired},
		Connecting:   {AuthRequired, Connecting},
		Syncing:      {Connecting, Syncing},
		Ready:        {Connecting, Syncing, Ready},
		Reconnecting: {Connecting, Syncing, Ready, Reconnecting},
		Degraded:     {Connecting, Syncing, Degraded},
		Error:        {Error},
	}
	for _, s := range paths[target] {
		if err := m.Transition(s); err != nil {
			t.Fatalf("walkTo(%s): %v", target, err)
		}
	}
}
