// Package ratelimiter provides distributed rate limiting using Redis.
//
// This package implements rate limiting with two algorithms:
// - Sliding Window (default via RedisRateLimiter)
// - Token Bucket (via TokenBucketLimiter)
package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBucketConfig configures the token bucket rate limiter
type TokenBucketConfig struct {
	Rate      float64 // Tokens per second (refill rate)
	Burst     int     // Maximum bucket size (burst capacity)
	KeyPrefix string  // Redis key prefix
}

// TokenBucketLimiter implements token bucket algorithm for rate limiting
// Token bucket allows bursts (up to Burst size) while enforcing average rate
type TokenBucketLimiter struct {
	client    *redis.Client
	config    TokenBucketConfig
	keyPrefix string
}

// TokenBucketOption configures the token bucket limiter
type TokenBucketOption func(*TokenBucketLimiter)

// WithTokenBucketKeyPrefix sets a prefix for all Redis keys
func WithTokenBucketKeyPrefix(prefix string) TokenBucketOption {
	return func(l *TokenBucketLimiter) {
		l.keyPrefix = prefix
	}
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
//
// Token bucket allows burst traffic up to the bucket size (Burst),
// then limits to the average rate (Rate). This is useful for APIs
// that need to handle occasional bursts (e.g., file uploads).
//
// Example:
//
//	limiter := ratelimiter.NewTokenBucketLimiter(redisClient, ratelimiter.TokenBucketConfig{
//	    Rate:      100.0,  // 100 tokens per second (average rate)
//	    Burst:     10,     // Allow bursts of up to 10
//	    KeyPrefix: "ratelimit:",
//	})
//
//	// Allow single request
//	allowed, err := limiter.Allow(ctx, "user:123")
//
//	// Allow multiple requests
//	allowed, err := limiter.AllowN(ctx, "user:123", 5)
func NewTokenBucketLimiter(client *redis.Client, config TokenBucketConfig, opts ...TokenBucketOption) *TokenBucketLimiter {
	if config.KeyPrefix == "" {
		config.KeyPrefix = "ratelimit"
	}
	if config.Burst <= 0 {
		config.Burst = 1
	}
	if config.Rate <= 0 {
		config.Rate = 1
	}

	l := &TokenBucketLimiter{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Allow checks if a single request is allowed
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed
// Uses Lua script for atomic bucket operations
func (l *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s:bucket", l.keyPrefix, key)

	// Lua script for atomic token bucket operations
	// Returns: [allowed (0/1), current_tokens]
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local burst = tonumber(ARGV[3])
		local n = tonumber(ARGV[4])

		-- Get current state
		local state = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(state[1])
		local last_refill = tonumber(state[2])

		-- Initialize if not exists
		if tokens == nil then
			tokens = burst
			last_refill = now
		end

		-- Calculate tokens to add since last refill
		local elapsed = (now - last_refill) / 1000.0  -- convert to seconds
		local new_tokens = math.min(burst, tokens + (elapsed * rate))

		-- Check if we have enough tokens
		if new_tokens >= n then
			-- Consume tokens
			new_tokens = new_tokens - n
			redis.call('HMSET', key, 'tokens', new_tokens, 'last_refill', now)
			redis.call('EXPIRE', key, math.ceil(burst / rate) + 10)
			return {1, new_tokens}
		else
			-- Not enough tokens
			return {0, new_tokens}
		end
	`

	result, err := l.client.Eval(ctx, script, []string{redisKey},
		int64(time.Now().UnixMilli()),
		l.config.Rate,
		l.config.Burst,
		n,
	).Slice()

	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit: %w", err)
	}

	allowed, _ := result[0].(int64)
	return allowed == 1, nil
}

// Reset resets the rate limit for a key
func (l *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("%s:%s:bucket", l.keyPrefix, key)
	return l.client.Del(ctx, redisKey).Err()
}

// GetLimit returns current token count and remaining tokens
func (l *TokenBucketLimiter) GetLimit(ctx context.Context, key string) (int, int, error) {
	redisKey := fmt.Sprintf("%s:%s:bucket", l.keyPrefix, key)

	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local burst = tonumber(ARGV[3])

		local state = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(state[1])
		local last_refill = tonumber(state[2])

		if tokens == nil then
			return {burst, burst}
		end

		local elapsed = (now - last_refill) / 1000.0
		local current = math.min(burst, tokens + (elapsed * rate))

		local remaining = math.max(0, math.floor(current))
		return {math.floor(current), remaining}
	`

	result, err := l.client.Eval(ctx, script, []string{redisKey},
		int64(time.Now().UnixMilli()),
		l.config.Rate,
		l.config.Burst,
	).Slice()

	if err != nil {
		return 0, 0, err
	}

	current, _ := result[0].(int64)
	remaining, _ := result[1].(int64)
	return int(current), int(remaining), nil
}

// Config returns the current configuration
func (l *TokenBucketLimiter) Config() TokenBucketConfig {
	return l.config
}
