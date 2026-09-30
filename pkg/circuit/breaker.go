// internal/platform/circuit/breaker.go
package circuit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"github.com/redis/go-redis/v9"
)

var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

type Breaker interface {
	Call(ctx context.Context, key string, fn func() error) error
	Execute(ctx context.Context, fn func() error) error
	GetState(ctx context.Context) (State, error)
}

// State represents the circuit breaker state
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// Statically assert that we implement the domain interface.
var _ Breaker = (*CircuitBreaker)(nil)

// CircuitBreaker implements the circuit breaker pattern using Redis
type CircuitBreaker struct {
	name       string
	client     *redis.Client
	config     config.CircuitBreakerConfig
	logger     log.Logger
	mu         sync.RWMutex
	localState State
	lastCheck  time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, client *redis.Client, cfg config.CircuitBreakerConfig, logger log.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:       name,
		client:     client,
		config:     cfg,
		logger:     logger,
		localState: StateClosed,
		lastCheck:  time.Now(),
	}
}

func (cb *CircuitBreaker) Call(ctx context.Context, key string, fn func() error) error {
	// Just call the existing Execute implementation, ignoring the key for now.
	return cb.Execute(ctx, fn)
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.config.Enabled {
		return fn()
	}

	state, err := cb.GetState(ctx)
	if err != nil {
		cb.logger.Warn("failed to get circuit breaker state", log.ErrorAttr(err))
		// Fail open - allow the request
		return fn()
	}

	if state == StateOpen {
		return ErrCircuitOpen
	}

	// Execute the function
	err = fn()

	// Record the result
	if err != nil {
		cb.recordFailure(ctx)
		return err
	}

	cb.recordSuccess(ctx)
	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState(ctx context.Context) (State, error) {
	// Check cache first
	cb.mu.RLock()
	if time.Since(cb.lastCheck) < time.Second {
		state := cb.localState
		cb.mu.RUnlock()
		return state, nil
	}
	cb.mu.RUnlock()

	stateKey := cb.getKey("state")
	stateStr, err := cb.client.Get(ctx, stateKey).Result()
	if err != nil {
		if err == redis.Nil {
			return StateClosed, nil
		}
		return "", err
	}

	state := State(stateStr)

	// Update cache
	cb.mu.Lock()
	cb.localState = state
	cb.lastCheck = time.Now()
	cb.mu.Unlock()

	return state, nil
}

// recordSuccess records a successful execution
func (cb *CircuitBreaker) recordSuccess(ctx context.Context) {
	state, _ := cb.GetState(ctx)

	if state == StateHalfOpen {
		successKey := cb.getKey("halfopen:success")
		count, _ := cb.client.Incr(ctx, successKey).Result()
		cb.client.Expire(ctx, successKey, cb.config.Interval)

		if uint32(count) >= cb.config.SuccessThreshold {
			cb.setState(ctx, StateClosed)
			cb.client.Del(ctx, successKey)
			cb.logger.Info("circuit breaker closed", log.String("name", cb.name))
		}
	}
}

// recordFailure records a failed execution
func (cb *CircuitBreaker) recordFailure(ctx context.Context) {
	failureKey := cb.getKey("failures")
	totalKey := cb.getKey("total")

	pipe := cb.client.Pipeline()
	pipe.Incr(ctx, failureKey)
	pipe.Incr(ctx, totalKey)
	pipe.Expire(ctx, failureKey, cb.config.Interval)
	pipe.Expire(ctx, totalKey, cb.config.Interval)

	results, err := pipe.Exec(ctx)
	if err != nil {
		cb.logger.Warn("failed to record failure", log.ErrorAttr(err))
		return
	}

	failures := results[0].(*redis.IntCmd).Val()
	total := results[1].(*redis.IntCmd).Val()

	if total == 0 {
		return
	}

	failureRate := float64(failures) / float64(total)

	if failureRate >= cb.config.FailureThreshold {
		cb.setState(ctx, StateOpen)
		cb.logger.Warn("circuit breaker opened",
			log.String("name", cb.name),
			log.Float64("failure_rate", failureRate),
			log.Int("failures", int(failures)),
			log.Int("total", int(total)),
		)

		// Schedule state check after timeout
		go cb.scheduleHalfOpen(ctx)
	}
}

// scheduleHalfOpen transitions to half-open state after timeout
func (cb *CircuitBreaker) scheduleHalfOpen(ctx context.Context) {
	time.Sleep(cb.config.Timeout)

	state, _ := cb.GetState(ctx)
	if state == StateOpen {
		cb.setState(ctx, StateHalfOpen)
		cb.logger.Info("circuit breaker half-open", log.String("name", cb.name))
	}
}

// setState sets the circuit breaker state
func (cb *CircuitBreaker) setState(ctx context.Context, state State) {
	stateKey := cb.getKey("state")
	cb.client.Set(ctx, stateKey, string(state), cb.config.Interval)

	// Update local cache
	cb.mu.Lock()
	cb.localState = state
	cb.lastCheck = time.Now()
	cb.mu.Unlock()

	// Clear counters when opening
	if state == StateOpen {
		cb.client.Del(ctx, cb.getKey("failures"), cb.getKey("total"))
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset(ctx context.Context) {
	cb.setState(ctx, StateClosed)
	cb.client.Del(ctx,
		cb.getKey("failures"),
		cb.getKey("total"),
		cb.getKey("halfopen:success"),
	)
	cb.logger.Info("circuit breaker reset", log.String("name", cb.name))
}

// getKey returns a Redis key for the circuit breaker
func (cb *CircuitBreaker) getKey(suffix string) string {
	return fmt.Sprintf("ngo:circuit:%s:%s", cb.name, suffix)
}
