// Package ratelimiter provides distributed rate limiting.
//
// This package adds per-user, per-IP, and per-API key rate limiting
// with multi-dimensional support.
package ratelimiter

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type (
	// DistributedLimiter provides per-user/IP/key rate limiting
	DistributedLimiter struct {
		client   *redis.Client
		config   *DistConfig
		mu       sync.RWMutex
		limiters map[string]*LimiterDim
	}

	// DistConfig holds configuration for distributed rate limiting
	DistConfig struct {
		// Default rate (requests per window)
		Rate int
		// Default window duration
		Window time.Duration
		// Key prefix for Redis
		KeyPrefix string
		// Per-user rate limit (0 = use default)
		PerUserRate int
		// Per-IP rate limit (0 = use default)
		PerIPRate int
		// Per-APIKey rate limit (0 = use default)
		PerAPIKeyRate int
		// Enable per-user limiting
		EnablePerUser bool
		// Enable per-IP limiting
		EnablePerIP bool
		// Enable per-APIKey limiting
		EnablePerAPIKey bool
	}

	// LimiterDim is a single dimension limiter
	LimiterDim struct {
		key    string
		rate   int
		window time.Duration
	}

	// MultiDimLimiter combines multiple rate limit dimensions
	MultiDimLimiter struct {
		client     *redis.Client
		dimensions []DimConfig
		keyPrefix  string
	}

	// DimConfig configures a single dimension
	DimConfig struct {
		Name   string
		Rate   int
		Window time.Duration
	}
)

const (
	distKeyPrefix = "dist:ratelimit:"
)

// DistOption configures distributed rate limiting
type DistOption func(*DistConfig)

// WithDistDefaultRate sets the default rate
func WithDistDefaultRate(rate int) DistOption {
	return func(c *DistConfig) {
		c.Rate = rate
	}
}

// WithDistDefaultWindow sets the default window
func WithDistDefaultWindow(window time.Duration) DistOption {
	return func(c *DistConfig) {
		c.Window = window
	}
}

// WithDistKeyPrefix sets the key prefix
func WithDistKeyPrefix(prefix string) DistOption {
	return func(c *DistConfig) {
		c.KeyPrefix = prefix
	}
}

// WithPerUserRate enables per-user rate limiting
func WithPerUserRate(rate int) DistOption {
	return func(c *DistConfig) {
		c.EnablePerUser = true
		c.PerUserRate = rate
	}
}

// WithPerIPRate enables per-IP rate limiting
func WithPerIPRate(rate int) DistOption {
	return func(c *DistConfig) {
		c.EnablePerIP = true
		c.PerIPRate = rate
	}
}

// WithPerAPIKeyRate enables per-APIKey rate limiting
func WithPerAPIKeyRate(rate int) DistOption {
	return func(c *DistConfig) {
		c.EnablePerAPIKey = true
		c.PerAPIKeyRate = rate
	}
}

// NewDistributedLimiter creates a distributed rate limiter
func NewDistributedLimiter(client *redis.Client, opts ...DistOption) *DistributedLimiter {
	cfg := &DistConfig{
		Rate:          100,
		Window:        time.Minute,
		KeyPrefix:     "ratelimit",
		PerUserRate:   1000,
		PerIPRate:     100,
		PerAPIKeyRate: 5000,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	dl := &DistributedLimiter{
		client:   client,
		config:   cfg,
		limiters: make(map[string]*LimiterDim),
	}

	if cfg.EnablePerUser {
		dl.limiters["user"] = &LimiterDim{
			key:    "user",
			rate:   cfg.PerUserRate,
			window: cfg.Window,
		}
	}

	if cfg.EnablePerIP {
		dl.limiters["ip"] = &LimiterDim{
			key:    "ip",
			rate:   cfg.PerIPRate,
			window: cfg.Window,
		}
	}

	if cfg.EnablePerAPIKey {
		dl.limiters["apikey"] = &LimiterDim{
			key:    "apikey",
			rate:   cfg.PerAPIKeyRate,
			window: cfg.Window,
		}
	}

	return dl
}

// Allow checks if a request is allowed (uses key as-is)
func (d *DistributedLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return d.AllowWithDims(ctx, key, "", "", "")
}

// AllowWithDims checks with dimensions for user, IP, and API key
func (d *DistributedLimiter) AllowWithDims(ctx context.Context, key, userID, ip, apiKey string) (bool, error) {
	var limits []bool
	var errs []error

	// Check each enabled dimension
	d.mu.RLock()
	if d.config.EnablePerUser && userID != "" {
		dim := d.limiters["user"]
		if dim != nil {
			allowed, err := d.checkLimit(ctx, fmt.Sprintf("%s:user:%s", key, userID), dim.rate, dim.window)
			limits = append(limits, allowed)
			errs = append(errs, err)
		}
	}

	if d.config.EnablePerIP && ip != "" {
		dim := d.limiters["ip"]
		if dim != nil {
			allowed, err := d.checkLimit(ctx, fmt.Sprintf("%s:ip:%s", key, ip), dim.rate, dim.window)
			limits = append(limits, allowed)
			errs = append(errs, err)
		}
	}

	if d.config.EnablePerAPIKey && apiKey != "" {
		dim := d.limiters["apikey"]
		if dim != nil {
			allowed, err := d.checkLimit(ctx, fmt.Sprintf("%s:apikey:%s", key, apiKey), dim.rate, dim.window)
			limits = append(limits, allowed)
			errs = append(errs, err)
		}
	}
	d.mu.RUnlock()

	// If no dimensions, use default rate limiting
	if len(limits) == 0 {
		return d.checkLimit(ctx, key, d.config.Rate, d.config.Window)
	}

	// All must pass
	for i, allowed := range limits {
		if !allowed {
			return false, nil
		}
		if errs[i] != nil {
			return false, errs[i]
		}
	}

	return true, nil
}

// AllowN checks if n requests are allowed
func (d *DistributedLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	return d.checkLimit(ctx, key, d.config.Rate*n, d.config.Window)
}

// checkLimit checks rate limit using sliding window algorithm
func (d *DistributedLimiter) checkLimit(ctx context.Context, key string, rate int, window time.Duration) (bool, error) {
	redisKey := distKeyPrefix + key

	now := time.Now()
	windowStart := now.Add(-window)

	pipe := d.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: fmt.Sprintf("%d:%d", now.UnixNano(), 1),
	})
	pipe.Expire(ctx, redisKey, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit: %w", err)
	}

	return countCmd.Val() <= int64(rate), nil
}

// GetClientIP extracts client IP from request headers or remote addr
func GetClientIP(xForwardedFor string, remoteAddr string) string {
	// Check X-Forwarded-For header first
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if IsValidIP(ip) {
				return ip
			}
		}
	}

	// Extract from remote address
	if remoteAddr != "" {
		ip, _, err := net.SplitHostPort(remoteAddr)
		if err == nil && IsValidIP(ip) {
			return ip
		}
	}

	return remoteAddr
}

// IsValidIP checks if string is a valid IP address
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GetLimits returns limits for all dimensions
func (d *DistributedLimiter) GetLimits(ctx context.Context, key, userID, ip, apiKey string) (map[string]int, error) {
	limits := make(map[string]int)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.config.EnablePerUser && userID != "" {
		dim := d.limiters["user"]
		if dim != nil {
			count, _, _ := d.getLimit(ctx, fmt.Sprintf("%s:user:%s", key, userID), dim.rate)
			limits["user"] = count
		}
	}

	if d.config.EnablePerIP && ip != "" {
		dim := d.limiters["ip"]
		if dim != nil {
			count, _, _ := d.getLimit(ctx, fmt.Sprintf("%s:ip:%s", key, ip), dim.rate)
			limits["ip"] = count
		}
	}

	if d.config.EnablePerAPIKey && apiKey != "" {
		dim := d.limiters["apikey"]
		if dim != nil {
			count, _, _ := d.getLimit(ctx, fmt.Sprintf("%s:apikey:%s", key, apiKey), dim.rate)
			limits["apikey"] = count
		}
	}

	return limits, nil
}

// getLimit gets the current limit count
func (d *DistributedLimiter) getLimit(ctx context.Context, key string, rate int) (int, int, error) {
	redisKey := distKeyPrefix + key
	windowStart := time.Now().Add(-d.config.Window)

	d.client.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))

	count, err := d.client.ZCard(ctx, redisKey).Result()
	if err != nil {
		return 0, 0, err
	}

	remaining := rate - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return int(count), remaining, nil
}

// Reset resets the rate limit for a key
func (d *DistributedLimiter) Reset(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		redisKey := distKeyPrefix + key
		if err := d.client.Del(ctx, redisKey).Err(); err != nil {
			return err
		}
	}
	return nil
}

// NewMultiDimLimiter creates a multi-dimensional rate limiter
func NewMultiDimLimiter(client *redis.Client, dims []DimConfig) *MultiDimLimiter {
	return &MultiDimLimiter{
		client:     client,
		dimensions: dims,
		keyPrefix:  "multidim:ratelimit:",
	}
}

// Allow checks if request is allowed across all dimensions
func (m *MultiDimLimiter) Allow(ctx context.Context, key string) (bool, map[string]bool, error) {
	results := make(map[string]bool)

	for _, dim := range m.dimensions {
		redisKey := m.keyPrefix + key + ":" + dim.Name

		now := time.Now()
		windowStart := now.Add(-dim.Window)

		pipe := m.client.Pipeline()
		pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
		countCmd := pipe.ZCard(ctx, redisKey)
		pipe.ZAdd(ctx, redisKey, redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: fmt.Sprintf("%d", now.UnixNano()),
		})
		pipe.Expire(ctx, redisKey, dim.Window)

		_, err := pipe.Exec(ctx)
		if err != nil {
			return false, results, err
		}

		allowed := countCmd.Val() <= int64(dim.Rate)
		results[dim.Name] = allowed
	}

	// All dimensions must pass
	for _, allowed := range results {
		if !allowed {
			return false, results, nil
		}
	}

	return true, results, nil
}

// GetDims returns the dimension configurations
func (m *MultiDimLimiter) GetDims() []DimConfig {
	return m.dimensions
}
