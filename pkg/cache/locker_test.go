package cache_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisLocker_Integration(t *testing.T) {
	client, teardown := testutil.NewTestRedis(t)
	defer teardown()

	locker := cache.NewRedisLocker(client, "test:lock:")
	ctx := context.Background()
	lockKey := "my-resource-123"
	lockTTL := 2 * time.Second

	t.Run("should acquire, block others, and release successfully", func(t *testing.T) {
		// --- 1. First actor acquires the lock ---
		lock1, err := locker.AcquireLock(ctx, lockKey, lockTTL)
		require.NoError(t, err)
		require.NotNil(t, lock1)

		// --- 2. Second actor fails to acquire the same lock ---
		_, err = locker.AcquireLock(ctx, lockKey, lockTTL)
		require.Error(t, err, "Second actor should fail to acquire the lock")
		assert.ErrorIs(t, err, cache.ErrLockNotAcquired)

		// --- 3. First actor releases the lock ---
		err = lock1.Unlock(ctx)
		require.NoError(t, err)

		// --- 4. Second actor can now acquire the lock ---
		lock2, err := locker.AcquireLock(ctx, lockKey, lockTTL)
		require.NoError(t, err)
		require.NotNil(t, lock2)

		// Cleanup
		_ = lock2.Unlock(ctx)
	})

	t.Run("lock should expire automatically after TTL", func(t *testing.T) {
		shortTTL := 100 * time.Millisecond
		lock, err := locker.AcquireLock(ctx, "auto-expire-key", shortTTL)
		require.NoError(t, err)
		require.NotNil(t, lock)

		// Wait for the lock to expire
		time.Sleep(shortTTL + 50*time.Millisecond)

		// A new actor should now be able to acquire the lock without an unlock
		lock2, err := locker.AcquireLock(ctx, "auto-expire-key", lockTTL)
		require.NoError(t, err, "Should be able to acquire an expired lock")
		require.NotNil(t, lock2)

		// Cleanup
		_ = lock2.Unlock(ctx)
	})

	t.Run("should correctly extend a lock's TTL", func(t *testing.T) {
		key := "extend-key"
		initialTTL := 1 * time.Second
		lock, err := locker.AcquireLock(ctx, key, initialTTL)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		time.Sleep(initialTTL/2 + 50*time.Millisecond)

		err = lock.Extend(ctx, 5*time.Second)
		require.NoError(t, err)

		time.Sleep(initialTTL)

		_, err = locker.AcquireLock(ctx, key, lockTTL)
		assert.ErrorIs(t, err, cache.ErrLockNotAcquired, "Lock should still be held after extension")
	})

	t.Run("unlock should be idempotent", func(t *testing.T) {
		lock, err := locker.AcquireLock(ctx, "idempotent-key", 5*time.Second)
		require.NoError(t, err)

		err = lock.Unlock(ctx)
		require.NoError(t, err)

		err = lock.Unlock(ctx)
		require.Error(t, err, "Second unlock should return an error as the lock is already gone")
	})

	t.Run("should handle concurrent attempts correctly", func(t *testing.T) {
		var wg sync.WaitGroup
		successCh := make(chan bool, 10)
		failureCh := make(chan bool, 10)
		concurrentKey := "concurrent-key"

		// --- Have 10 goroutines try to acquire the same lock simultaneously ---
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lock, err := locker.AcquireLock(ctx, concurrentKey, 5*time.Second)
				if err != nil {
					if errors.Is(err, cache.ErrLockNotAcquired) {
						failureCh <- true
					}
				} else {
					successCh <- true
					// ==========================================================
					// THE FIX: Hold the lock for a short period.
					// This simulates doing work and is essential for testing
					// that other goroutines are actually excluded.
					// ==========================================================
					time.Sleep(100 * time.Millisecond)
					_ = lock.Unlock(ctx)
				}
			}()
		}
		wg.Wait()
		close(successCh)
		close(failureCh)

		// --- Assert ---
		successCount := len(successCh)
		failureCount := len(failureCh)

		assert.Equal(t, 1, successCount, "Exactly one goroutine should have acquired the lock")
		assert.Equal(t, 9, failureCount, "The other nine goroutines should have failed to acquire it")
		t.Logf("Concurrent test finished: %d successes, %d failures", successCount, failureCount)
	})
}
