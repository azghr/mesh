// Package ratelimiter provides distributed rate limiting with adaptive capabilities.
//
// The adaptive rate limiter automatically adjusts the request rate based on observed
// latency to optimize throughput while keeping response times under control.
//
// Quick Example:
//
//	limiter := ratelimiter.NewAdaptiveLimiter(redisClient,
//	    ratelimiter.WithBaseRate(1000),       // Start at 1000 rps
//	    ratelimiter.WithMinRate(100),          // Minimum 100 rps
//	    ratelimiter.WithMaxRate(10000),        // Maximum 10000 rps
//	    ratelimiter.WithTargetLatency(100*time.Millisecond),
//	)
//
//	// In request handler
//	start := time.Now()
//	resp, err := client.Do(req)
//	limiter.RecordLatency(ctx, "api", time.Since(start))
//	allowed, _ := limiter.Allow(ctx, "api")
//
// # How It Works
//
// 1. Starts at BaseRate (1000 requests/second)
// 2. Records latencies for each request
// 3. Calculates average latency over the window
// 4. If latency < TargetLatency: increases rate by AdjustmentStep
// 5. If latency > TargetLatency: decreases rate by AdjustmentStep*2
// 6. Clamps rate to MinRate/MaxRate bounds
//
// This creates a feedback loop that automatically finds the optimal rate for your system.
package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type (
	// AdaptiveConfig holds configuration for the adaptive rate limiter.
	AdaptiveConfig struct {
		// BaseRate is the initial requests per second.
		BaseRate float64
		// MinRate is the minimum allowed rate.
		MinRate float64
		// MaxRate is the maximum allowed rate.
		MaxRate float64
		// TargetLatency is the desired p99 latency.
		TargetLatency time.Duration
		// AdjustmentStep is how much to adjust rate per check.
		AdjustmentStep float64
		// Window is the time window for latency measurements.
		Window time.Duration
		// KeyPrefix is the Redis key prefix.
		KeyPrefix string
	}

	AdaptiveLimiter struct {
		client      *redis.Client
		config      *AdaptiveConfig
		mu          sync.RWMutex
		currentRate float64
		lastAdjust  time.Time
	}

	AdaptiveOption func(*AdaptiveConfig)
)

const (
	latencyKeySuffix = ":latency"
)

func WithBaseRate(rate float64) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.BaseRate = rate
	}
}

func WithMinRate(rate float64) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.MinRate = rate
	}
}

func WithMaxRate(rate float64) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.MaxRate = rate
	}
}

func WithTargetLatency(d time.Duration) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.TargetLatency = d
	}
}

func WithAdjustmentStep(step float64) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.AdjustmentStep = step
	}
}

func WithAdaptiveWindow(d time.Duration) AdaptiveOption {
	return func(c *AdaptiveConfig) {
		c.Window = d
	}
}

func NewAdaptiveLimiter(client *redis.Client, opts ...AdaptiveOption) *AdaptiveLimiter {
	cfg := &AdaptiveConfig{
		BaseRate:       1000,
		MinRate:        100,
		MaxRate:        10000,
		TargetLatency:  100 * time.Millisecond,
		AdjustmentStep: 100,
		Window:         time.Minute,
		KeyPrefix:      "adaptive:ratelimit",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &AdaptiveLimiter{
		client:      client,
		config:      cfg,
		currentRate: cfg.BaseRate,
		lastAdjust:  time.Now(),
	}
}

func (a *AdaptiveLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return a.AllowN(ctx, key, 1)
}

func (a *AdaptiveLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	rate := a.getCurrentRate()

	redisKey := fmt.Sprintf("%s:%s", a.config.KeyPrefix, key)
	now := time.Now()
	windowStart := now.Add(-a.config.Window)

	pipe := a.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now.UnixMilli()), Member: fmt.Sprintf("%d:%d", now.UnixNano(), n)})
	pipe.Expire(ctx, redisKey, a.config.Window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit: %w", err)
	}

	return countCmd.Val()+int64(n) <= int64(rate), nil
}

func (a *AdaptiveLimiter) RecordLatency(ctx context.Context, key string, latency time.Duration) error {
	latencyKey := fmt.Sprintf("%s:%s%s", a.config.KeyPrefix, key, latencyKeySuffix)

	now := time.Now()
	windowStart := now.Add(-a.config.Window)

	pipe := a.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, latencyKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	pipe.ZAdd(ctx, latencyKey, redis.Z{Score: float64(now.UnixNano()), Member: fmt.Sprintf("%d", latency.Milliseconds())})
	pipe.Expire(ctx, latencyKey, a.config.Window)

	_, err := pipe.Exec(ctx)
	return err
}

func (a *AdaptiveLimiter) getCurrentRate() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentRate
}

func (a *AdaptiveLimiter) GetRate(ctx context.Context, key string) (float64, error) {
	key = fmt.Sprintf("%s:%s", a.config.KeyPrefix, key)

	avgLatency, err := a.getAverageLatency(ctx, key)
	if err != nil {
		return a.config.BaseRate, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	newRate := a.currentRate

	if avgLatency < a.config.TargetLatency {
		newRate = a.currentRate + a.config.AdjustmentStep
		if newRate > a.config.MaxRate {
			newRate = a.config.MaxRate
		}
	} else if avgLatency > a.config.TargetLatency {
		newRate = a.currentRate - a.config.AdjustmentStep*2
		if newRate < a.config.MinRate {
			newRate = a.config.MinRate
		}
	}

	if newRate != a.currentRate {
		a.currentRate = newRate
		a.lastAdjust = time.Now()
	}

	return a.currentRate, nil
}

func (a *AdaptiveLimiter) getAverageLatency(ctx context.Context, key string) (time.Duration, error) {
	latencyKey := fmt.Sprintf("%s:%s%s", a.config.KeyPrefix, key, latencyKeySuffix)

	now := time.Now()
	windowStart := now.Add(-a.config.Window)

	results, err := a.client.ZRangeByScore(ctx, latencyKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", windowStart.UnixMilli()),
		Max: fmt.Sprintf("%d", now.UnixMilli()),
	}).Result()

	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, nil
	}

	var total int64
	for _, member := range results {
		var v int64
		fmt.Sscanf(member, "%d", &v)
		total += v
	}

	return time.Duration(total/int64(len(results))) * time.Millisecond, nil
}

func (a *AdaptiveLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("%s:%s", a.config.KeyPrefix, key)
	return a.client.Del(ctx, redisKey).Err()
}

func (a *AdaptiveLimiter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	avgLatency, _ := a.getAverageLatency(ctx, "global")

	return map[string]interface{}{
		"current_rate": a.currentRate,
		"base_rate":    a.config.BaseRate,
		"min_rate":     a.config.MinRate,
		"max_rate":     a.config.MaxRate,
		"last_adjust":  a.lastAdjust,
		"avg_latency":  avgLatency,
	}, nil
}

func (a *AdaptiveLimiter) ForceRate(rate float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if rate < a.config.MinRate {
		rate = a.config.MinRate
	}
	if rate > a.config.MaxRate {
		rate = a.config.MaxRate
	}

	a.currentRate = rate
}

func (a *AdaptiveLimiter) SetConfig(config *AdaptiveConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if config.BaseRate > 0 {
		a.config.BaseRate = config.BaseRate
	}
	if config.MinRate > 0 {
		a.config.MinRate = config.MinRate
	}
	if config.MaxRate > 0 {
		a.config.MaxRate = config.MaxRate
	}
	if config.TargetLatency > 0 {
		a.config.TargetLatency = config.TargetLatency
	}
	if config.AdjustmentStep > 0 {
		a.config.AdjustmentStep = config.AdjustmentStep
	}
}

func (a *AdaptiveLimiter) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return fmt.Sprintf("AdaptiveLimiter{rate=%v, min=%v, max=%v}", a.currentRate, a.config.MinRate, a.config.MaxRate)
}
