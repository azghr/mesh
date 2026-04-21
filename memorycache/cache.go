// Package memorycache provides a thread-safe in-memory cache with TTL and LRU eviction.
//
// This package implements a simple in-memory key-value cache with:
// - Thread-safe operations using sync.RWMutex
// - TTL (time-to-live) support for entries
// - LRU eviction when max size is reached
// - GetOrSet pattern for cache-aside caching
//
// Example:
//
//	cache := memorycache.New(
//	    memorycache.WithMaxSize(1000),
//	    memorycache.WithTTL(time.Hour),
//	)
//
//	user, err := cache.GetOrSet(ctx, "user:123", func() (any, error) {
//	    return db.FindUser(ctx, "123")
//	}, time.Minute*5)
package memorycache

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNotFound = errors.New("cache miss")
)

// Config holds the cache configuration.
type Config struct {
	MaxSize int
	TTL     time.Duration
}

// Option configures the cache.
type Option func(*Config)

// WithMaxSize sets the maximum number of entries.
// Default: 10000
func WithMaxSize(size int) Option {
	return func(c *Config) {
		c.MaxSize = size
	}
}

// WithTTL sets the default TTL for entries.
// Default: 5 minutes
func WithTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.TTL = ttl
	}
}

// entry holds a cached value with metadata.
type entry struct {
	key       string
	value     any
	expiresAt time.Time
	element   *list.Element
}

// Cache is a thread-safe in-memory cache with TTL and LRU eviction.
type Cache struct {
	mu     sync.RWMutex
	items  map[string]*entry
	lru    *list.List
	config Config
	stats  Stats
}

// New creates a new in-memory cache.
func New(opts ...Option) *Cache {
	cfg := Config{
		MaxSize: 10000,
		TTL:     5 * time.Minute,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Cache{
		items:  make(map[string]*entry),
		lru:    list.New(),
		config: cfg,
	}
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		c.RecordMiss()
		return nil, ErrNotFound
	}

	if time.Now().After(item.expiresAt) {
		c.RecordMiss()
		return nil, ErrNotFound
	}

	c.RecordHit()
	return item.value, nil
}

// Set stores a value in the cache with TTL.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.config.TTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.setLocked(key, value, ttl)
}

func (c *Cache) setLocked(key string, value any, ttl time.Duration) {
	expiresAt := time.Now().Add(ttl)

	if existing, ok := c.items[key]; ok {
		existing.value = value
		existing.expiresAt = expiresAt
		c.lru.MoveToFront(existing.element)
		return
	}

	if len(c.items) >= c.config.MaxSize {
		c.evictOne()
	}

	elem := c.lru.PushFront(key)
	c.items[key] = &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
		element:   elem,
	}
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		c.lru.Remove(item.element)
		delete(c.items, key)
	}
}

// GetOrSet retrieves a value from cache, or executes the fetch function and caches the result.
// This implements the cache-aside pattern.
func (c *Cache) GetOrSet(ctx context.Context, key string, fetchFn func() (any, error), ttl time.Duration) (any, error) {
	if val, err := c.Get(key); err == nil {
		return val, nil
	}

	val, err := fetchFn()
	if err != nil {
		return nil, err
	}

	c.Set(key, val, ttl)
	return val, nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*entry)
	c.lru.Init()
}

// evictOne removes the least recently used entry.
func (c *Cache) evictOne() {
	if c.lru.Len() == 0 {
		return
	}

	elem := c.lru.Back()
	key := elem.Value.(string)
	delete(c.items, key)
	c.lru.Remove(elem)
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Metrics returns current cache statistics.
type Metrics struct {
	Size   int
	Hits   int64
	Misses int64
}

// Stats holds atomic statistics counters.
type Stats struct {
	hits   int64
	misses int64
}

// GetMetrics returns cache statistics.
func (c *Cache) GetMetrics() Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Metrics{
		Size:   len(c.items),
		Hits:   atomic.LoadInt64(&c.stats.hits),
		Misses: atomic.LoadInt64(&c.stats.misses),
	}
}

// RecordHit records a cache hit.
func (c *Cache) RecordHit() {
	atomic.AddInt64(&c.stats.hits, 1)
}

// RecordMiss records a cache miss.
func (c *Cache) RecordMiss() {
	atomic.AddInt64(&c.stats.misses, 1)
}

// ResetMetrics resets the hit/miss counters.
func (c *Cache) ResetMetrics() {
	atomic.StoreInt64(&c.stats.hits, 0)
	atomic.StoreInt64(&c.stats.misses, 0)
}
