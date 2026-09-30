package health

import (
	"context"
	"sync"
)

// CheckFunc is a function that returns an error if the component is unhealthy.
type CheckFunc func(ctx context.Context) error

// Checker manages health checks for various components.
type Checker struct {
	checks map[string]CheckFunc
	mu     sync.RWMutex
}

func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

// Register adds a component to be health-checked.
// If 'fn' is nil, it registers a dummy check that always passes (useful for static dependencies).
func (c *Checker) Register(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fn == nil {
		fn = func(ctx context.Context) error { return nil }
	}
	c.checks[name] = fn
}

// Check runs all registered health checks in parallel.
// It returns true if all checks pass, and a map of component names to their status/error.
func (c *Checker) Check(ctx context.Context) (bool, map[string]string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	healthy := true

	for name, checkFn := range c.checks {
		wg.Add(1)
		go func(n string, fn CheckFunc) {
			defer wg.Done()
			if err := fn(ctx); err != nil {
				mu.Lock()
				results[n] = "unhealthy: " + err.Error()
				healthy = false
				mu.Unlock()
			} else {
				mu.Lock()
				results[n] = "healthy"
				mu.Unlock()
			}
		}(name, checkFn)
	}

	wg.Wait()
	return healthy, results
}
