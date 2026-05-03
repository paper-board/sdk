// Package clock provides an injectable time source for paper-board services.
//
// Real returns wall-clock time in UTC. Fake is settable for tests.
package clock

import (
	"sync"
	"time"
)

// Clock returns the current time. All implementations MUST return UTC.
type Clock interface {
	Now() time.Time
}

// Real is the production wall-clock implementation.
type Real struct{}

// Now returns time.Now() converted to UTC.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a controllable clock for tests. Safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a Fake initialised at t (normalised to UTC).
func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

// Now returns the currently set fake time (UTC).
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set replaces the fake time (normalised to UTC).
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}

// Advance moves the fake clock forward (or back, if d is negative) by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
