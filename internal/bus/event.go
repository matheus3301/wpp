package bus

import "time"

// Event represents a domain event published on the bus.
type Event struct {
	Kind      string
	Timestamp time.Time
	Payload   any
}
