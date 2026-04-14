// Package ratelimiter provides distributed rate limiting using Redis.
package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter interface for rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	AllowN(ctx context.Context, key string, n int) (bool, error)
	Reset(ctx context.Context, key string) error
	GetLimit(ctx context.Context, key string) (int, int, error)
}

// RedisRateLimiter implements distributed rate limiting using Redis
type RedisRateLimiter struct {
	client    *redis.Client
	rate      int
	window    time.Duration
	keyPrefix string
}

// Option configures the rate limiter
type Option func(*RedisRateLimiter)

// WithKeyPrefix sets a prefix for all Redis keys
func WithKeyPrefix(prefix string) Option {
	return func(l *RedisRateLimiter) {
		l.keyPrefix = prefix
	}
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client, rate int, window time.Duration, opts ...Option) *RedisRateLimiter {
	l := &RedisRateLimiter{
		client:    client,
		rate:      rate,
		window:    window,
		keyPrefix: "ratelimit",
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Allow checks if a request is allowed
func (l *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed
func (l *RedisRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)
	now := time.Now()
	windowStart := now.Add(-l.window)

	pipe := l.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now.UnixMilli()), Member: fmt.Sprintf("%d:%d", now.UnixNano(), n)})
	pipe.Expire(ctx, redisKey, l.window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit: %w", err)
	}

	return countCmd.Val()+int64(n) <= int64(l.rate), nil
}

// Reset resets the rate limit for a key
func (l *RedisRateLimiter) Reset(ctx context.Context, key string) error {
	return l.client.Del(ctx, fmt.Sprintf("%s:%s", l.keyPrefix, key)).Err()
}

// GetLimit returns the current count and remaining requests
func (l *RedisRateLimiter) GetLimit(ctx context.Context, key string) (int, int, error) {
	redisKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)
	windowStart := time.Now().Add(-l.window)

	l.client.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))

	count, err := l.client.ZCard(ctx, redisKey).Result()
	if err != nil {
		return 0, 0, err
	}

	current := int(count)
	remaining := l.rate - current
	if remaining < 0 {
		remaining = 0
	}
	return current, remaining, nil
}

// SimpleRateLimiter provides a simple in-memory rate limiter for single instance
type SimpleRateLimiter struct {
	mu       sync.Mutex
	requests map[string]int
	rate     int
	window   time.Duration
}

// NewSimpleRateLimiter creates a simple in-memory rate limiter
func NewSimpleRateLimiter(rate int, window time.Duration) *SimpleRateLimiter {
	return &SimpleRateLimiter{
		requests: make(map[string]int),
		rate:     rate,
		window:   window,
	}
}

// Allow checks if a request is allowed
func (l *SimpleRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN checks if n requests are allowed
func (l *SimpleRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := l.requests[key]
	if count+n > l.rate {
		return false, nil
	}
	l.requests[key] = count + n
	return true, nil
}

// Reset resets the rate limit for a key
func (l *SimpleRateLimiter) Reset(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.requests, key)
	return nil
}

// GetLimit returns current count and remaining
func (l *SimpleRateLimiter) GetLimit(ctx context.Context, key string) (int, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.requests[key]
	remaining := l.rate - current
	if remaining < 0 {
		remaining = 0
	}
	return current, remaining, nil
}
