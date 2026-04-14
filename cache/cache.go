// Package cache provides Redis-backed caching utilities.
//
// This package implements a caching layer with automatic serialization,
// hit/miss metrics, and common caching patterns like cache-aside.
//
// Quick example:
//
//	cache, _ := cache.New(redisClient, 5*time.Minute)
//	var user User
//	err := cache.GetOrSet(ctx, "user:123", &user, time.Hour, func() (interface{}, error) {
//	    return db.FindUser(ctx, "123")
//	})
//
// # Patterns
//
// - Get/Set: basic key-value operations with JSON serialization
// - GetOrSet: cache-aside pattern - fetch from source on miss
// - MultiGet/MultiSet: batch operations for performance
// - InvalidateByPrefix: clear all keys matching a prefix
//
// # Metrics
//
// The cache tracks hits, misses, sets, deletes, and errors automatically.
// Metrics are thread-safe and can be reset via ResetMetrics().
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss is returned when a key is not found in the cache
	ErrCacheMiss = errors.New("cache miss")
	// ErrInvalidType is returned when cached value cannot be unmarshaled
	ErrInvalidType = errors.New("invalid cached value type")
	// ErrRedisRequired is returned when Redis client is not available
	ErrRedisRequired = errors.New("redis client is required")
)

// Cache provides caching functionality with Redis backend
type Cache struct {
	client     RedisClient
	defaultTTL time.Duration
	keyPrefix  string
	metrics    *Metrics
	mu         sync.Mutex
}

// RedisClient interface defines the required Redis methods for caching
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
	Pipeline() redis.Pipeliner
}

// Metrics tracks cache performance
type Metrics struct {
	Hits    int64
	Misses  int64
	Sets    int64
	Deletes int64
	Errors  int64
}

// New creates a new cache with the given Redis client
func New(client RedisClient, defaultTTL time.Duration) (*Cache, error) {
	if client == nil {
		return nil, ErrRedisRequired
	}

	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}

	return &Cache{
		client:     client,
		defaultTTL: defaultTTL,
		keyPrefix:  "cache:",
		metrics:    &Metrics{},
	}, nil
}

// NewWithPrefix creates a new cache with a custom key prefix
func NewWithPrefix(client RedisClient, defaultTTL time.Duration, keyPrefix string) (*Cache, error) {
	if client == nil {
		return nil, ErrRedisRequired
	}

	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}

	return &Cache{
		client:     client,
		defaultTTL: defaultTTL,
		keyPrefix:  keyPrefix,
		metrics:    &Metrics{},
	}, nil
}

// formatKey adds the cache prefix to a key
func (c *Cache) formatKey(key string) string {
	return c.keyPrefix + key
}

func (c *Cache) incHits() {
	c.mu.Lock()
	c.metrics.Hits++
	c.mu.Unlock()
}

func (c *Cache) incMisses() {
	c.mu.Lock()
	c.metrics.Misses++
	c.mu.Unlock()
}

func (c *Cache) incErrors() {
	c.mu.Lock()
	c.metrics.Errors++
	c.mu.Unlock()
}

func (c *Cache) incSets(n int64) {
	c.mu.Lock()
	c.metrics.Sets += n
	c.mu.Unlock()
}

func (c *Cache) incDeletes() {
	c.mu.Lock()
	c.metrics.Deletes++
	c.mu.Unlock()
}

// Get retrieves a value from the cache and unmarshals it into dest
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	cacheKey := c.formatKey(key)

	val, err := c.client.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			c.incMisses()
			return ErrCacheMiss
		}
		c.incErrors()
		return fmt.Errorf("failed to get from cache: %w", err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		c.incErrors()
		return fmt.Errorf("%w: %v", ErrInvalidType, err)
	}

	c.incHits()
	return nil
}

// Set stores a value in the cache with the given TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	cacheKey := c.formatKey(key)

	data, err := json.Marshal(value)
	if err != nil {
		c.incErrors()
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := c.client.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	c.incSets(1)
	return nil
}

// Delete removes a value from the cache
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = c.formatKey(key)
	}

	if err := c.client.Del(ctx, cacheKeys...).Err(); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	c.incDeletes()
	return nil
}

// Exists checks if a key exists in the cache
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	cacheKey := c.formatKey(key)

	count, err := c.client.Exists(ctx, cacheKey).Result()
	if err != nil {
		c.incErrors()
		return false, fmt.Errorf("failed to check cache existence: %w", err)
	}

	return count > 0, nil
}

// GetOrSet retrieves a value from cache or computes and caches it
func (c *Cache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, fn func() (interface{}, error)) error {
	// Try to get from cache
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil // Cache hit
	}

	if err != ErrCacheMiss {
		return err // Real error
	}

	// Cache miss - compute value
	value, err := fn()
	if err != nil {
		return err
	}

	// Store in cache (don't fail the request if caching fails)
	if cacheErr := c.Set(ctx, key, value, ttl); cacheErr != nil {
		c.incErrors()
	}

	// Set dest value
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// GetOrSetJSON retrieves a JSON value from cache or computes and caches it
func (c *Cache) GetOrSetJSON(ctx context.Context, key string, dest interface{}, ttl time.Duration, fn func() ([]byte, error)) error {
	// Try to get from cache
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil // Cache hit
	}

	if err != ErrCacheMiss {
		return err // Real error
	}

	// Cache miss - compute value
	data, err := fn()
	if err != nil {
		return err
	}

	// Store in cache (don't fail the request if caching fails)
	if cacheErr := c.Set(ctx, key, data, ttl); cacheErr != nil {
		c.incErrors()
	}

	// Set dest value
	return json.Unmarshal(data, dest)
}

// GetOrSetString retrieves a string value from cache or computes and caches it
func (c *Cache) GetOrSetString(ctx context.Context, key string, ttl time.Duration, fn func() (string, error)) (string, error) {
	// Try to get from cache
	val, err := c.client.Get(ctx, c.formatKey(key)).Result()
	if err == nil {
		c.incHits()
		return val, nil // Cache hit
	}

	if err != redis.Nil {
		c.incErrors()
		return "", fmt.Errorf("failed to get from cache: %w", err)
	}

	c.incMisses()

	// Cache miss - compute value
	value, err := fn()
	if err != nil {
		return "", err
	}

	// Store in cache (don't fail the request if caching fails)
	if cacheErr := c.Set(ctx, key, value, ttl); cacheErr != nil {
		c.incErrors()
	}

	return value, nil
}

// InvalidateByPrefix invalidates all keys with a given prefix
func (c *Cache) InvalidateByPrefix(ctx context.Context, prefix string) error {
	fullPrefix := c.keyPrefix + prefix

	keys, err := c.client.Keys(ctx, fullPrefix+"*").Result()
	if err != nil {
		c.incErrors()
		return fmt.Errorf("failed to find keys by prefix: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			c.incErrors()
			return fmt.Errorf("failed to delete keys by prefix: %w", err)
		}
		c.incDeletes()
	}

	return nil
}

// SetTTL updates the TTL for an existing key
func (c *Cache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	cacheKey := c.formatKey(key)

	if err := c.client.Expire(ctx, cacheKey, ttl).Err(); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to set TTL: %w", err)
	}

	return nil
}

// GetTTL returns the remaining time-to-live for a key
func (c *Cache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	cacheKey := c.formatKey(key)

	ttl, err := c.client.TTL(ctx, cacheKey).Result()
	if err != nil {
		c.incErrors()
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	return ttl, nil
}

// Clear removes all keys with the cache prefix
func (c *Cache) Clear(ctx context.Context) error {
	return c.InvalidateByPrefix(ctx, "")
}

// Metrics returns the cache metrics
func (c *Cache) Metrics() *Metrics {
	return c.metrics
}

// ResetMetrics resets the cache metrics
func (c *Cache) ResetMetrics() {
	c.mu.Lock()
	c.metrics = &Metrics{}
	c.mu.Unlock()
}

// HitRate returns the cache hit rate as a percentage
func (c *Cache) HitRate() float64 {
	total := c.metrics.Hits + c.metrics.Misses
	if total == 0 {
		return 0
	}
	return float64(c.metrics.Hits) / float64(total) * 100
}

// Warmup pre-loads the cache with the given data
func (c *Cache) Warmup(ctx context.Context, data map[string]interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	pipe := c.client.Pipeline()

	for key, value := range data {
		cacheKey := c.formatKey(key)

		jsonData, err := json.Marshal(value)
		if err != nil {
			c.incErrors()
			continue
		}

		pipe.Set(ctx, cacheKey, jsonData, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to warmup cache: %w", err)
	}

	c.incSets(int64(len(data)))
	return nil
}

// MultiGet retrieves multiple values from cache
func (c *Cache) MultiGet(ctx context.Context, keys []string, dest map[string]interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = c.formatKey(key)
		cmds[i] = pipe.Get(ctx, cacheKeys[i])
	}

	if _, err := pipe.Exec(ctx); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to multi-get from cache: %w", err)
	}

	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err != nil {
			if err != redis.Nil {
				c.incErrors()
			}
			c.incMisses()
			continue
		}

		if dest != nil {
			var destValue interface{}
			if err := json.Unmarshal([]byte(val), &destValue); err != nil {
				c.incErrors()
				continue
			}
			dest[keys[i]] = destValue
		}

		c.incHits()
	}

	return nil
}

// MultiSet stores multiple values in cache
func (c *Cache) MultiSet(ctx context.Context, data map[string]interface{}, ttl time.Duration) error {
	if len(data) == 0 {
		return nil
	}

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	pipe := c.client.Pipeline()

	for key, value := range data {
		cacheKey := c.formatKey(key)

		jsonData, err := json.Marshal(value)
		if err != nil {
			c.incErrors()
			continue
		}

		pipe.Set(ctx, cacheKey, jsonData, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		c.incErrors()
		return fmt.Errorf("failed to multi-set in cache: %w", err)
	}

	c.incSets(int64(len(data)))
	return nil
}
