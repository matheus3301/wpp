package bus

import (
	"strings"
	"sync"
)

// Bus is an in-process publish/subscribe event bus with namespace filtering.
type Bus struct {
	mu   sync.RWMutex
	subs map[int]*subscription
	next int
}

type subscription struct {
	namespace string
	ch        chan Event
}

// New creates a new event bus.
func New() *Bus {
	return &Bus{
		subs: make(map[int]*subscription),
	}
}

// Publish sends an event to all subscribers whose namespace is a prefix of event.Kind.
func (b *Bus) Publish(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subs {
		if strings.HasPrefix(evt.Kind, sub.namespace) {
			select {
			case sub.ch <- evt:
			default:
				// Drop event if subscriber is full (non-blocking).
			}
		}
	}
}

// Subscribe returns a channel that receives events matching the given namespace prefix.
// bufSize controls the channel buffer. Returns the channel and an unsubscribe function.
func (b *Bus) Subscribe(namespace string, bufSize int) (<-chan Event, func()) {
	ch := make(chan Event, bufSize)
	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = &subscription{namespace: namespace, ch: ch}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}
