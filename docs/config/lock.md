# lock

Distributed locking using Redis. Ensures only one instance executes a critical section.

## What It Does

Implements distributed locks using Redis with automatic expiration, retry logic, and safe release. Useful for:
- Preventing duplicate job processing
- Coordinating across multiple service instances
- Ensuring single execution of background tasks

## Basic Usage

```go
lock := lock.NewRedisLock(redisClient)

// Execute function while holding lock
err := lock.Execute(ctx, "my-resource", 30*time.Second, func() error {
    // Only one instance can be here at a time
    return doCriticalWork()
})
```

## Manual Lock Control

```go
// Try to acquire (non-blocking)
result, err := lock.TryAcquire(ctx, "resource", 30*time.Second)
if result.Acquired {
    // Got the lock
    defer lock.Release(ctx, "resource")
}

// Acquire with retry
result, err := lock.Acquire(ctx, "resource", 30*time.Second, 10*time.Second)
if err != nil {
    return err // Couldn't get lock in time
}
// result.LockID identifies this lock instance
defer lock.ReleaseWithID(ctx, "resource", result.LockID)
```

## Lock Result

```go
type LockResult struct {
    Acquired  bool          // Whether lock was obtained
    LockID    string        // Unique ID for this lock instance
    WaitTime  time.Duration // How long we waited
    ExpiresAt time.Time     // When lock expires
}
```

## Safe Release

```go
// Release only if we own the lock (matches LockID)
err := lock.ReleaseWithID(ctx, "resource", lockID)

// Simple release (may release someone else's lock - use carefully)
err := lock.Release(ctx, "resource")
```

## Extending Locks

```go
// Refresh TTL (must own the lock)
err := lock.Refresh(ctx, "resource", lockID, 30*time.Second)

// Auto-extend while running long tasks
go lock.ExtendLock(ctx, "resource", lockID, 30*time.Second, 10*time.Second)
// Continues extending until context cancelled or lock lost
```

## Retry with Backoff

```go
// Execute with automatic retry on failure
err := lock.ExecuteWithRetry(ctx, "resource", 30*time.Second, 3, func() error {
    return doWork()
})
// Retries up to 3 times if the function fails
```

## Checking Lock Status

```go
// Is lock held?
isLocked, err := lock.IsLocked(ctx, "resource")

// Get current lock holder (if available)
lockID, err := lock.GetLockID(ctx, "resource")
```

## In-Memory Lock (Testing)

```go
// For testing or single-instance scenarios
memLock := lock.NewMemoryLock()
memLock.TryAcquire(ctx, "resource", time.Second)
// Doesn't support same features as Redis lock
```

## Metrics

```go
// Enable metrics collection
lock := lock.NewRedisLock(redisClient, lock.WithMetrics(lock.MetricsConfig{
    EnableWaitTime:   true,   // track wait time histogram
    EnableContention: true,   // track contention events
}))

// When lock is acquired:
// mesh_lock_acquired_total{key="resource",owner="12345"} 1
// mesh_lock_wait_seconds{key="resource",owner="12345"} 0.234
// mesh_lock_contention_total{key="resource",owner="12345"} 3
```

## Error Handling

```go
var (
    ErrLockFailed        = errors.New("failed to acquire lock")
    ErrLockNotHeld       = errors.New("lock not held")
    ErrLockExpired       = errors.New("lock has expired")
    ErrInvalidLockDuration = errors.New("lock duration must be positive")
)
```

## Best Practices

1. **Always use `ReleaseWithID`** - safer than simple `Release`
2. **Set reasonable TTL** - not too short (task doesn't finish), not too long (slow recovery if crash)
3. **Use `Execute`** when possible - handles acquisition and release automatically
4. **Handle context cancellation** - if your operation is cancelled, the lock will auto-expire