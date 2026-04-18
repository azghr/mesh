// Package cache provides Redis-backed caching utilities.
//
// This package implements multi-level caching with L1 (in-memory) and L2 (Redis) layers.
// Benefits include reduced Redis load and faster access to frequently used data.
//
// Example:
//
//	// Create multi-level cache
//	ml := cache.NewMultiLevel(redisClient, cache.MultiLevelConfig{
//	    L1TTL:      1*time.Minute,   // in-memory TTL
//	    L2TTL:      1*time.Hour,    // Redis TTL
//	    L1MaxSize:  10000,          // max entries in memory
//	    SyncWrites: true,          // write both L1 and L2
//	})
//
//	// Get from cache (L1 -> L2 -> DB fallback)
//	val, err := ml.Get(ctx, "user:123")
//
//	// Cache-aside pattern
//	val, err := ml.GetOrSet(ctx, "user:123", time.Hour, func() (any, error) {
//	    return db.FindUser(ctx, "123")
//	})
package cache

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// MultiLevelConfig holds configuration for multi-level cache.
// All durations default to sensible values if not set.
type MultiLevelConfig struct {
	L1TTL      time.Duration // L1 cache TTL (in-memory), default 1min
	L2TTL      time.Duration // L2 cache TTL (Redis), default 1hr
	L1MaxSize  int           // Max entries in L1, default 10000
	SyncWrites bool          // Write to both L1 and L2 on Set
}

// MultiLevelCache provides L1 (memory) + L2 (Redis) caching.
// L1 is a thread-safe in-memory cache that reduces Redis load.
// L2 is Redis for persistence across instances.
type MultiLevelCache struct {
	client    RedisClient
	config    MultiLevelConfig
	l1        *l1Cache
	keyPrefix string
}

// l1Cache is an in-memory L1 cache with TTL and size limits.
type l1Cache struct {
	mu    sync.RWMutex
	items map[string]l1Item
	max   int
}

type l1Item struct {
	Value   any
	Expires time.Time
}

// NewMultiLevel creates a new multi-level cache with the given configuration.
func NewMultiLevel(client RedisClient, cfg MultiLevelConfig) *MultiLevelCache {
	if cfg.L1TTL == 0 {
		cfg.L1TTL = 1 * time.Minute
	}
	if cfg.L2TTL == 0 {
		cfg.L2TTL = 1 * time.Hour
	}
	if cfg.L1MaxSize == 0 {
		cfg.L1MaxSize = 10000
	}

	return &MultiLevelCache{
		client:    client,
		config:    cfg,
		l1:        &l1Cache{items: make(map[string]l1Item), max: cfg.L1MaxSize},
		keyPrefix: "ml:",
	}
}

// Get retrieves a value from the cache hierarchy.
// It first checks L1 (memory), then L2 (Redis).
// Returns ErrCacheMiss if not found in either layer.
func (m *MultiLevelCache) Get(ctx context.Context, key string) (any, error) {
	// Try L1 first (fastest)
	if val := m.getL1(key); val != nil {
		return val, nil
	}

	// Fall back to L2 (Redis)
	val, err := m.client.Get(ctx, m.keyPrefix+key).Result()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}

	// Promote to L1 for faster subsequent access
	m.setL1(key, val, m.config.L1TTL)

	return val, nil
}

// Set stores a value in L1 and optionally L2.
// If SyncWrites is true, also writes to Redis (L2).
// Uses L1TTL for L1 and provided ttl (or L2TTL) for L2.
func (m *MultiLevelCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	// Always write L1
	m.setL1(key, value, m.config.L1TTL)

	// Write L2 if sync enabled
	if m.config.SyncWrites {
		l2TTL := ttl
		if l2TTL == 0 {
			l2TTL = m.config.L2TTL
		}
		return m.client.Set(ctx, m.keyPrefix+key, value, l2TTL).Err()
	}

	return nil
}

// GetOrSet implements the cache-aside pattern.
// First tries to get from cache hierarchy.
// On miss, calls the provided function to fetch and caches the result.
func (m *MultiLevelCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	// Try cache first
	val, err := m.Get(ctx, key)
	if err == nil {
		return val, nil
	}

	// Fetch from source (DB, API, etc.)
	val, err = fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := m.Set(ctx, key, val, ttl); err != nil {
		return nil, err
	}

	return val, nil
}

// Invalidate removes a key from both L1 and L2.
func (m *MultiLevelCache) Invalidate(ctx context.Context, key string) error {
	// Remove from L1
	m.l1.mu.Lock()
	delete(m.l1.items, key)
	m.l1.mu.Unlock()

	// Remove from L2
	return m.client.Del(ctx, m.keyPrefix+key).Err()
}

// ClearL1 clears only the in-memory (L1) cache.
// Useful for testing or forced refresh.
func (m *MultiLevelCache) ClearL1() {
	m.l1.mu.Lock()
	m.l1.items = make(map[string]l1Item)
	m.l1.mu.Unlock()
}

// Stats returns current cache sizes.
// L1Size is the in-memory count, L2Size would require Redis call.
func (m *MultiLevelCache) Stats() (l1Size, l2Size int) {
	m.l1.mu.RLock()
	l1Size = len(m.l1.items)
	m.l1.mu.RUnlock()
	return l1Size, 0
}

// getL1 retrieves from L1 cache with expiration check.
func (m *MultiLevelCache) getL1(key string) any {
	m.l1.mu.RLock()
	item, ok := m.l1.items[key]
	m.l1.mu.RUnlock()

	if !ok {
		return nil
	}

	// Check TTL expiration
	if time.Now().After(item.Expires) {
		m.l1.mu.Lock()
		delete(m.l1.items, key)
		m.l1.mu.Unlock()
		return nil
	}

	return item.Value
}

// setL1 stores in L1 with eviction if full.
func (m *MultiLevelCache) setL1(key string, value any, ttl time.Duration) {
	m.l1.mu.Lock()
	defer m.l1.mu.Unlock()

	// Evict expired entries if at capacity
	if len(m.l1.items) >= m.l1.max {
		m.l1.evict()
	}

	m.l1.items[key] = l1Item{
		Value:   value,
		Expires: time.Now().Add(ttl),
	}
}

// evict removes expired entries (25% at a time) or clears if still full.
func (m *l1Cache) evict() {
	now := time.Now()
	evictCount := len(m.items) / 4

	// Remove expired first
	for key, item := range m.items {
		if now.After(item.Expires) {
			delete(m.items, key)
			evictCount--
		}
		if evictCount <= 0 {
			break
		}
	}

	// If still at capacity, clear all
	if len(m.items) >= m.max {
		m.items = make(map[string]l1Item)
	}
}
