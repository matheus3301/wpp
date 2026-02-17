package model

import (
	"sync"
	"time"
)

// Flash holds transient notification messages.
type Flash struct {
	mu      sync.RWMutex
	message string
	expires time.Time
}

// Set stores a flash message that expires after the given duration.
func (f *Flash) Set(msg string, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.message = msg
	f.expires = time.Now().Add(d)
}

// Get returns the current flash message, or empty if expired.
func (f *Flash) Get() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if time.Now().After(f.expires) {
		return ""
	}
	return f.message
}
