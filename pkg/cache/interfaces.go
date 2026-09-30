package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// --- Domain-Specific Errors ---
var (
	// ErrLockNotAcquired is returned by a Locker when a lock for a specific key
	// cannot be acquired, usually because it's already held by another process.
	ErrLockNotAcquired = errors.New("could not acquire lock")
)

// Pipeliner defines the interface for executing multiple cache commands in a single round-trip.
type Pipeliner interface {
	LPush(ctx context.Context, key string, values ...interface{}) error
	LTrim(ctx context.Context, key string, start, stop int64) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Exec(ctx context.Context) error
}

// Service defines the contract for a generic caching layer.
type Service interface {
	Get(ctx context.Context, key string) ([]byte, error)
	GetObject(ctx context.Context, key string, value interface{}) (bool, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, key string) error

	// Pipelining and List operations needed for hot/warm cache logic.
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	Pipeline() Pipeliner
}

// --- Distributed Locking ---

// Lock represents an acquired distributed lock. The holder of this object is
// responsible for unlocking it.
type Lock interface {
	// Unlock releases the lock. It is safe to call multiple times.
	// Implementations should be idempotent.
	Unlock(ctx context.Context) error

	// Extend attempts to refresh the lock's time-to-live (TTL).
	// This is useful for long-running tasks to prevent the lock from expiring prematurely.
	Extend(ctx context.Context, ttl time.Duration) error
}

// Locker defines the contract for a distributed locking mechanism.
type Locker interface {
	// AcquireLock attempts to obtain a lock for a given key with a specified TTL.
	// If the lock is already held, it must return ErrLockNotAcquired.
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

// --- Idempotency Store ---

// IdempotencyStore defines the contract for storing and retrieving the results
// of idempotent operations.
type IdempotencyStore interface {
	// StoreResult saves the outcome of an operation (either a successful result or an error)
	// associated with a unique idempotency key.
	StoreResult(ctx context.Context, key string, result json.RawMessage, execErr error, ttl time.Duration) error

	// GetResult attempts to retrieve a previously stored result.
	// It returns:
	// - result: The JSON result if the operation was successful.
	// - execErr: The error if the operation failed.
	// - found: A boolean indicating if a result was found for the key.
	// - err: An error related to the store itself (e.g., connection issues).
	GetResult(ctx context.Context, key string) (result json.RawMessage, execErr error, found bool, err error)
}
