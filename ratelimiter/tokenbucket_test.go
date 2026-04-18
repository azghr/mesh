package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	return server, client
}

func TestTokenBucketConfig_Defaults(t *testing.T) {
	config := TokenBucketConfig{
		Rate:  100.0,
		Burst: 10,
	}

	assert.Equal(t, 100.0, config.Rate)
	assert.Equal(t, 10, config.Burst)
}

func TestTokenBucketLimiter_New(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      100.0,
		Burst:     10,
		KeyPrefix: "test",
	})

	assert.NotNil(t, limiter)
	assert.Equal(t, 100.0, limiter.Config().Rate)
	assert.Equal(t, 10, limiter.Config().Burst)
}

func TestTokenBucketLimiter_BurstBehavior(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      100.0, // 100 tokens per second
		Burst:     10,    // burst up to 10
		KeyPrefix: "burst:",
	})

	ctx := context.Background()

	// First 10 requests should be allowed (burst)
	for i := 0; i < 10; i++ {
		allowed, err := limiter.Allow(ctx, "user:1")
		assert.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// 11th request should be rate limited
	allowed, err := limiter.Allow(ctx, "user:1")
	assert.NoError(t, err)
	assert.False(t, allowed, "11th request should be rate limited")
}

func TestTokenBucketLimiter_RefillOverTime(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	// Very slow rate: 1 token per second, burst of 1
	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      1.0, // 1 token per second
		Burst:     1,   // burst of 1
		KeyPrefix: "refill:",
	})

	ctx := context.Background()

	// First request allowed (burst)
	allowed, err := limiter.Allow(ctx, "user:1")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Second request should be denied immediately
	allowed, err = limiter.Allow(ctx, "user:1")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Wait for refill
	time.Sleep(1100 * time.Millisecond)

	// Now should be allowed again
	allowed, err = limiter.Allow(ctx, "user:1")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestTokenBucketLimiter_AllowN(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      100.0,
		Burst:     10,
		KeyPrefix: "multitest:",
	})

	ctx := context.Background()

	// Allow 5 requests at once
	allowed, err := limiter.AllowN(ctx, "user:1", 5)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Try 6 more (total 11 > burst of 10)
	allowed, err = limiter.AllowN(ctx, "user:1", 6)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestTokenBucketLimiter_GetLimit(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      100.0,
		Burst:     10,
		KeyPrefix: "getlimit:",
	})

	ctx := context.Background()

	// First check - should show full bucket
	current, remaining, err := limiter.GetLimit(ctx, "user:1")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, current, 0)
	assert.GreaterOrEqual(t, remaining, 0)

	// Make some requests
	limiter.Allow(ctx, "user:1")
	limiter.Allow(ctx, "user:1")

	current, remaining, err = limiter.GetLimit(ctx, "user:1")
	assert.NoError(t, err)
	assert.LessOrEqual(t, current, 10)
}

func TestTokenBucketLimiter_Reset(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      100.0,
		Burst:     2,
		KeyPrefix: "reset:",
	})

	ctx := context.Background()

	// Use up the burst
	limiter.Allow(ctx, "user:1")
	limiter.Allow(ctx, "user:1")

	// Should be rate limited
	allowed, _ := limiter.Allow(ctx, "user:1")
	assert.False(t, allowed)

	// Reset
	err := limiter.Reset(ctx, "user:1")
	assert.NoError(t, err)

	// Should be allowed again
	allowed, err = limiter.Allow(ctx, "user:1")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestTokenBucketLimiter_DifferentKeys(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      1.0, // 1 per second
		Burst:     1,   // burst of 1
		KeyPrefix: "keys:",
	})

	ctx := context.Background()

	// Different keys should be independent
	allowed1, _ := limiter.Allow(ctx, "user:1")
	allowed2, _ := limiter.Allow(ctx, "user:2")

	assert.True(t, allowed1, "user:1 should be allowed")
	assert.True(t, allowed2, "user:2 should be allowed")
}

func TestTokenBucketLimiter_Config(t *testing.T) {
	server, client := setupMiniRedis(t)
	defer server.Close()
	defer client.Close()

	config := TokenBucketConfig{
		Rate:      50.0,
		Burst:     5,
		KeyPrefix: "configtest:",
	}

	limiter := NewTokenBucketLimiter(client, config)
	assert.Equal(t, config, limiter.Config())
}

func BenchmarkTokenBucketLimiter_Allow(b *testing.B) {
	server, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()

	limiter := NewTokenBucketLimiter(client, TokenBucketConfig{
		Rate:      10000.0,
		Burst:     10000,
		KeyPrefix: "bench:",
	})

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = limiter.Allow(ctx, "bench:key")
	}
}
