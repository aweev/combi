/*
Package cache provides a unified interface for Redis operations, distributed locking, and idempotency.

Usage of Locker:

	locker := app.Core.Locker
	// Lock key "my-resource" for 5 seconds
	lock, err := locker.AcquireLock(ctx, "my-resource", 5*time.Second)
	if err != nil {
	    return err // Could not acquire lock
	}
	defer lock.Unlock(ctx)

	// Do critical work...
*/
package cache
