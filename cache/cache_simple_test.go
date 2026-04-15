package cache

import (
	"context"
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

func newMockRedisClient(t *testing.T) *mockRedisClient {
	t.Helper()
	server, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	return &mockRedisClient{client: client, server: server}
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	return m.client.Get(ctx, key)
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return m.client.Set(ctx, key, value, expiration)
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return m.client.Del(ctx, keys...)
}

func (m *mockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return m.client.Exists(ctx, keys...)
}

func (m *mockRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	return m.client.Keys(ctx, pattern)
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return m.client.Expire(ctx, key, expiration)
}

func (m *mockRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return m.client.TTL(ctx, key)
}

func (m *mockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return m.client.Scan(ctx, cursor, match, count)
}

func (m *mockRedisClient) Pipeline() redis.Pipeliner {
	return m.client.Pipeline()
}

func (m *mockRedisClient) Close() error {
	m.server.Close()
	return m.client.Close()
}

func TestCacheBasic(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	cache, err := New(mockRedis, 5*time.Minute)
	assert.NoError(t, err)
	assert.NotNil(t, cache)

	ctx := context.Background()
	type testData struct {
		Value string `json:"value"`
	}

	// Test Set
	err = cache.Set(ctx, "test:key", testData{Value: "test"}, time.Minute)
	assert.NoError(t, err)

	// Test Get
	var result testData
	err = cache.Get(ctx, "test:key", &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result.Value)

	// Test Exists
	exists, err := cache.Exists(ctx, "test:key")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test Delete
	err = cache.Delete(ctx, "test:key")
	assert.NoError(t, err)

	exists, _ = cache.Exists(ctx, "test:key")
	assert.False(t, exists)
}

func TestCacheMetrics(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	cache, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	type testData struct{ Value string }

	// Perform operations
	_ = cache.Set(ctx, "key1", testData{Value: "1"}, time.Minute)
	var result testData
	_ = cache.Get(ctx, "key1", &result)    // hit
	_ = cache.Get(ctx, "missing", &result) // miss

	metrics := cache.Metrics()
	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses)
	assert.Equal(t, int64(1), metrics.Sets)

	hitRate := cache.HitRate()
	assert.Equal(t, float64(50), hitRate)
}

func BenchmarkCacheSet(b *testing.B) {
	server, err := miniredis.Run()
	require.NoError(b, err)
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	mockRedis := &mockRedisClient{client: client, server: server}

	cache, _ := New(mockRedis, 5*time.Minute)
	ctx := context.Background()
	type testData struct{ Value string }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "key", testData{Value: "test"}, time.Minute)
	}
}
