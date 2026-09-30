package clock

import "time"

// Provider abstracts the system clock.
type Provider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
}

type realClock struct{}

func New() Provider                                  { return &realClock{} }
func (c *realClock) Now() time.Time                  { return time.Now().UTC() }
func (c *realClock) Since(t time.Time) time.Duration { return time.Since(t) }
func (c *realClock) Until(t time.Time) time.Duration { return time.Until(t) }
