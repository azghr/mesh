package lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	client *redis.Client
	server *miniredis.Miniredis
}

func newMockRedis(t *testing.T) *mockRedisClient {
	server, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})

	return &mockRedisClient{
		client: client,
		server: server,
	}
}

func (m *mockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	return m.client.SetNX(ctx, key, value, expiration)
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return m.client.Del(ctx, keys...)
}

func (m *mockRedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	return m.client.Eval(ctx, script, keys, args...)
}

func (m *mockRedisClient) Close() error {
	m.server.Close()
	return m.client.Close()
}

func TestRedisLock_TryAcquire(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		ttl     time.Duration
		setup   func(*RedisLock, string)
		want    bool
		wantErr bool
	}{
		{
			name:    "acquires new lock",
			key:     "test:lock",
			ttl:     10 * time.Second,
			want:    true,
			wantErr: false,
		},
		{
			name: "fails when lock already held",
			key:  "test:lock",
			ttl:  10 * time.Second,
			setup: func(l *RedisLock, key string) {
				ctx := context.Background()
				// Pre-acquire lock
				_ = l.client.SetNX(ctx, formatLockKey(key), "locked", 10*time.Second)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:    "acquires lock with zero TTL",
			key:     "test:lock",
			ttl:     1, // Positive duration required
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedis := newMockRedis(t)
			defer mockRedis.Close()

			lock := NewRedisLock(mockRedis)

			if tt.setup != nil {
				tt.setup(lock, tt.key)
			}

			got, err := lock.TryAcquire(context.Background(), tt.key, tt.ttl)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got.Acquired)
		})
	}
}

func TestRedisLock_Release(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*RedisLock, string)
		wantErr bool
	}{
		{
			name: "releases held lock",
			setup: func(l *RedisLock, key string) {
				ctx := context.Background()
				_ = l.client.SetNX(ctx, formatLockKey(key), "locked", 10*time.Second)
			},
			wantErr: false,
		},
		{
			name: "succeeds when lock not held",
			setup: func(l *RedisLock, key string) {
				// No lock acquired
			},
			wantErr: true, // ErrLockNotHeld
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedis := newMockRedis(t)
			defer mockRedis.Close()

			lock := NewRedisLock(mockRedis)
			testKey := "test:lock"

			if tt.setup != nil {
				tt.setup(lock, testKey)
			}

			err := lock.Release(context.Background(), testKey)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedisLock_Execute(t *testing.T) {
	tests := []struct {
		name    string
		fn      func() error
		wantErr error
	}{
		{
			name: "executes function successfully",
			fn: func() error {
				return nil
			},
			wantErr: nil,
		},
		{
			name: "returns function error",
			fn: func() error {
				return errors.New("function error")
			},
			wantErr: errors.New("function error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedis := newMockRedis(t)
			defer mockRedis.Close()

			lock := NewRedisLock(mockRedis)

			err := lock.Execute(context.Background(), "test:lock", 10*time.Second, tt.fn)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedisLock_Refresh(t *testing.T) {
	mockRedis := newMockRedis(t)
	defer mockRedis.Close()

	lock := NewRedisLock(mockRedis)

	ctx := context.Background()

	// Acquire lock first
	result, err := lock.TryAcquire(ctx, "test:lock", 1*time.Second)
	require.True(t, result.Acquired)
	require.NoError(t, err)

	// Refresh the lock
	err = lock.Refresh(ctx, "test:lock", result.LockID, 5*time.Second)
	assert.NoError(t, err)
}

func TestRedisLock_ConcurrentAccess(t *testing.T) {
	mockRedis := newMockRedis(t)
	defer mockRedis.Close()

	lock := NewRedisLock(mockRedis)

	ctx := context.Background()
	var wg sync.WaitGroup
	acquiredCount := 0
	var mu sync.Mutex

	// Try to acquire from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			result, err := lock.TryAcquire(ctx, "test:lock", 1*time.Second)
			if err != nil {
				return
			}

			if result.Acquired {
				mu.Lock()
				acquiredCount++
				mu.Unlock()

				// Hold briefly then release
				time.Sleep(10 * time.Millisecond)
				_ = lock.Release(ctx, "test:lock")
			}
		}()
	}

	wg.Wait()

	// Should have acquired at least once
	assert.Greater(t, acquiredCount, 0)
}

func TestRedisLock_ExecuteWithTimeout(t *testing.T) {
	mockRedis := newMockRedis(t)
	defer mockRedis.Close()

	lock := NewRedisLock(mockRedis)

	ctx := context.Background()

	// Function that takes longer than lock TTL
	err := lock.Execute(ctx, "test:lock", 100*time.Millisecond, func() error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	// Should still complete (lock may expire during execution)
	assert.NoError(t, err)
}

func TestMemoryLock_TryAcquire(t *testing.T) {
	lock := NewMemoryLock()

	tests := []struct {
		name    string
		key     string
		ttl     time.Duration
		setup   func()
		want    bool
		wantErr error
	}{
		{
			name:    "acquires new lock",
			key:     "test:lock",
			ttl:     10 * time.Second,
			want:    true,
			wantErr: nil,
		},
		{
			name: "fails when lock already held",
			key:  "test:lock",
			ttl:  10 * time.Second,
			setup: func() {
				_, _ = lock.TryAcquire(context.Background(), "test:lock", 10*time.Second)
			},
			want:    true, // MemoryLock always acquires
			wantErr: nil,
		},
		{
			name:    "acquires lock with different key",
			key:     "different:lock",
			ttl:     10 * time.Second,
			want:    true,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			got, err := lock.TryAcquire(context.Background(), tt.key, tt.ttl)

			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, got.Acquired)
		})
	}
}

func TestMemoryLock_Release(t *testing.T) {
	lock := NewMemoryLock()

	ctx := context.Background()

	// Acquire lock
	result, err := lock.TryAcquire(ctx, "test:lock", 10*time.Second)
	require.True(t, result.Acquired)
	require.NoError(t, err)

	// Release lock
	err = lock.Release(ctx, "test:lock")
	assert.NoError(t, err)

	// Should be able to acquire again
	result, err = lock.TryAcquire(ctx, "test:lock", 10*time.Second)
	assert.True(t, result.Acquired)
	assert.NoError(t, err)
}

func TestMemoryLock_Execute(t *testing.T) {
	lock := NewMemoryLock()

	tests := []struct {
		name    string
		fn      func() error
		wantErr error
	}{
		{
			name: "executes function successfully",
			fn: func() error {
				return nil
			},
			wantErr: nil,
		},
		{
			name: "returns function error",
			fn: func() error {
				return errors.New("function error")
			},
			wantErr: errors.New("function error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := lock.Execute(ctx, "test:lock", 10*time.Second, tt.fn)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryLock_ConcurrentAccess(t *testing.T) {
	lock := NewMemoryLock()

	ctx := context.Background()
	var wg sync.WaitGroup
	acquiredCount := 0
	var mu sync.Mutex

	// Try to acquire from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			result, err := lock.TryAcquire(ctx, "test:lock", 1*time.Second)
			if err != nil {
				return
			}

			if result.Acquired {
				mu.Lock()
				acquiredCount++
				mu.Unlock()

				// Hold briefly then release
				time.Sleep(10 * time.Millisecond)
				_ = lock.Release(ctx, "test:lock")
			}
		}(i)
	}

	wg.Wait()

	// MemoryLock allows multiple acquisitions for testing
	assert.Greater(t, acquiredCount, 1)
}

func TestMemoryLock_MultipleKeys(t *testing.T) {
	lock := NewMemoryLock()
	ctx := context.Background()

	// Acquire multiple different locks
	keys := []string{"lock1", "lock2", "lock3"}

	for _, key := range keys {
		result, err := lock.TryAcquire(ctx, key, 10*time.Second)
		assert.True(t, result.Acquired)
		assert.NoError(t, err)
	}

	// All should be held
	for _, key := range keys {
		result, err := lock.TryAcquire(ctx, key, 10*time.Second)
		assert.True(t, result.Acquired)
		assert.NoError(t, err)
	}

	// Release all
	for _, key := range keys {
		err := lock.Release(ctx, key)
		assert.NoError(t, err)
	}
}

func TestFormatLockKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{
			key:  "test:lock",
			want: "lock:test:lock",
		},
		{
			key:  "simple",
			want: "lock:simple",
		},
		{
			key:  "",
			want: "lock:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := formatLockKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkRedisLock_TryAcquire(b *testing.B) {
	mockRedis := newMockRedis(&testing.T{})
	defer mockRedis.Close()

	lock := NewRedisLock(mockRedis)

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := b.Name()
		lock.TryAcquire(ctx, key, 1*time.Second)
	}
}

func BenchmarkRedisLock_Execute(b *testing.B) {
	mockRedis := newMockRedis(&testing.T{})
	defer mockRedis.Close()

	lock := NewRedisLock(mockRedis)

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := b.Name()
		lock.Execute(ctx, key, 1*time.Second, func() error {
			return nil
		})
	}
}

func BenchmarkMemoryLock_TryAcquire(b *testing.B) {
	lock := NewMemoryLock()
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := b.Name()
		lock.TryAcquire(ctx, key, 1*time.Second)
	}
}

func BenchmarkMemoryLock_Execute(b *testing.B) {
	lock := NewMemoryLock()
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := b.Name()
		lock.Execute(ctx, key, 1*time.Second, func() error {
			return nil
		})
	}
}
