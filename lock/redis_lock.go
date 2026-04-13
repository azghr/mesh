// Package lock provides distributed locking utilities for all Lumex services.
//
// This package implements distributed locking using Redis with proper
// error handling, automatic retry, and lock expiration.
//
// Example:
//
//	lock := lock.NewRedisLock(redisClient)
//	err := lock.Execute(ctx, "my-resource", 30*time.Second, func() error {
//	    // Critical section - only one instance will execute this
//	    return doWork()
//	})
package lock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockFailed is returned when lock cannot be acquired
	ErrLockFailed = errors.New("failed to acquire lock")
	// ErrLockNotHeld is returned when trying to release a lock not held
	ErrLockNotHeld = errors.New("lock not held")
	// ErrLockExpired is returned when a lock has expired
	ErrLockExpired = errors.New("lock has expired")
	// ErrInvalidLockDuration is returned for invalid lock durations
	ErrInvalidLockDuration = errors.New("lock duration must be positive")
)

// RedisClient interface defines the required Redis methods for locking
type RedisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// RedisLock implements distributed locking using Redis
type RedisLock struct {
	client RedisClient
}

// NewRedisLock creates a new Redis-based lock manager
func NewRedisLock(client RedisClient) *RedisLock {
	return &RedisLock{
		client: client,
	}
}

// LockResult contains information about a lock operation
type LockResult struct {
	Acquired  bool          // Whether the lock was acquired
	LockID    string        // Unique identifier for this lock instance
	WaitTime  time.Duration // Time waited to acquire the lock
	ExpiresAt time.Time     // When the lock will expire
}

// TryAcquire attempts to acquire a lock without retry
func (r *RedisLock) TryAcquire(ctx context.Context, key string, ttl time.Duration) (*LockResult, error) {
	if ttl <= 0 {
		return nil, ErrInvalidLockDuration
	}

	lockID := generateLockID()
	lockKey := formatLockKey(key)
	now := time.Now()

	// Use SET NX EX for atomic lock acquisition
	acquired, err := r.client.SetNX(ctx, lockKey, lockID, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return &LockResult{
		Acquired:  acquired,
		LockID:    lockID,
		WaitTime:  time.Since(now),
		ExpiresAt: now.Add(ttl),
	}, nil
}

// Acquire acquires a lock with automatic retry
func (r *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration, maxWait time.Duration) (*LockResult, error) {
	if ttl <= 0 {
		return nil, ErrInvalidLockDuration
	}

	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	startTime := time.Now()

	for {
		// Try to acquire the lock
		result, err := r.TryAcquire(ctx, key, ttl)
		if err != nil {
			lastErr = err
		} else if result.Acquired {
			result.WaitTime = time.Since(startTime)
			return result, nil
		}

		// Check if we've exceeded the deadline
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: %v", ErrLockFailed, lastErr)
		}

		// Wait before retrying
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Release releases a lock by key
func (r *RedisLock) Release(ctx context.Context, key string) error {
	lockKey := formatLockKey(key)
	result, err := r.client.Del(ctx, lockKey).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == 0 {
		return ErrLockNotHeld
	}

	return nil
}

// ReleaseWithID releases a lock only if it matches the given lock ID
func (r *RedisLock) ReleaseWithID(ctx context.Context, key, lockID string) error {
	lockKey := formatLockKey(key)

	// Use Lua script to ensure atomic release only if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := r.client.Eval(ctx, script, []string{lockKey}, lockID).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == int64(0) {
		return ErrLockNotHeld
	}

	return nil
}

// Execute runs a function while holding a lock
func (r *RedisLock) Execute(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	// Acquire the lock
	result, err := r.Acquire(ctx, key, ttl, 5*time.Second)
	if err != nil {
		return err
	}

	// Ensure lock is released
	defer func() {
		_ = r.ReleaseWithID(context.Background(), key, result.LockID)
	}()

	// Execute the function
	return fn()
}

// ExecuteWithRetry runs a function while holding a lock, with retry if the function fails
func (r *RedisLock) ExecuteWithRetry(ctx context.Context, key string, ttl time.Duration, maxRetries int, fn func() error) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := r.Execute(ctx, key, ttl, fn)
		if err == nil {
			return nil
		}

		lastErr = err

		// Wait before retrying (exponential backoff)
		waitTime := time.Duration(i+1) * 100 * time.Millisecond
		select {
		case <-time.After(waitTime):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// Refresh extends the lock expiration time
func (r *RedisLock) Refresh(ctx context.Context, key, lockID string, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidLockDuration
	}

	lockKey := formatLockKey(key)

	// Use Lua script to ensure atomic refresh only if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := r.client.Eval(ctx, script, []string{lockKey}, lockID, int(ttl.Seconds())).Result()
	if err != nil {
		return fmt.Errorf("failed to refresh lock: %w", err)
	}

	if result == int64(0) {
		return ErrLockNotHeld
	}

	return nil
}

// IsLocked checks if a lock is currently held
func (r *RedisLock) IsLocked(ctx context.Context, key string) (bool, error) {
	lockKey := formatLockKey(key)
	exists, err := r.client.SetNX(ctx, lockKey, "", 1*time.Millisecond).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check lock: %w", err)
	}

	// If SetNX returned false, the key exists (lock is held)
	return !exists, nil
}

// GetLockID returns the current lock ID for a key
func (r *RedisLock) GetLockID(ctx context.Context, key string) (string, error) {
	lockKey := formatLockKey(key)

	client, ok := r.client.(interface {
		Get(ctx context.Context, key string) *redis.StringCmd
	})
	if !ok {
		return "", errors.New("client does not support Get operation")
	}

	lockID, err := client.Get(ctx, lockKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // No lock
		}
		return "", fmt.Errorf("failed to get lock ID: %w", err)
	}

	return lockID, nil
}

// ExtendLock automatically extends a lock at regular intervals
func (r *RedisLock) ExtendLock(ctx context.Context, key, lockID string, ttl, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.Refresh(ctx, key, lockID, ttl); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil // Context cancelled, stop extending
		}
	}
}

// formatLockKey creates a consistent key format for locks
func formatLockKey(key string) string {
	return "lock:" + key
}

// generateLockID creates a unique lock ID
func generateLockID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// MemoryLock is an in-memory implementation for testing or single-instance deployments
type MemoryLock struct {
	mu    sync.RWMutex
	locks map[string]string
}

// NewMemoryLock creates a new in-memory lock manager
func NewMemoryLock() *MemoryLock {
	return &MemoryLock{
		locks: make(map[string]string),
	}
}

// TryAcquire attempts to acquire an in-memory lock
func (m *MemoryLock) TryAcquire(ctx context.Context, key string, ttl time.Duration) (*LockResult, error) {
	lockKey := formatLockKey(key)
	lockID := generateLockID()

	m.mu.Lock()
	m.locks[lockKey] = lockID
	m.mu.Unlock()

	return &LockResult{
		Acquired:  true,
		LockID:    lockID,
		WaitTime:  0,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// Release releases an in-memory lock
func (m *MemoryLock) Release(ctx context.Context, key string) error {
	lockKey := formatLockKey(key)

	m.mu.Lock()
	delete(m.locks, lockKey)
	m.mu.Unlock()

	return nil
}

// Execute runs a function while holding an in-memory lock
func (m *MemoryLock) Execute(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	_, err := m.TryAcquire(ctx, key, ttl)
	if err != nil {
		return err
	}

	defer m.Release(ctx, key)

	return fn()
}
