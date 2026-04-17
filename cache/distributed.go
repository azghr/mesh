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
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	invalidationChannel = "cache:invalidate"
	invalidationPrefix  = "cache:invalidate:"
)

type PubSub interface {
	Subscribe(ctx context.Context, channel string) *redis.PubSub
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
}

// DistributedConfig holds configuration for the distributed cache
type DistributedConfig struct {
	Client     RedisClient
	PubSub     PubSub
	DefaultTTL time.Duration
	KeyPrefix  string
	Channel    string
}

// DistributedCache provides cache with cross-instance invalidation via Redis pub/sub
type DistributedCache struct {
	local   *Cache
	pubsub  PubSub
	channel string
	mu      sync.Mutex
	subs    map[string]func(prefix string) error
}

// NewDistributedCache creates a new distributed cache with pub/sub invalidation
func NewDistributedCache(config DistributedConfig) (*DistributedCache, error) {
	local, err := NewWithPrefix(config.Client, config.DefaultTTL, config.KeyPrefix)
	if err != nil {
		return nil, err
	}

	channel := config.Channel
	if channel == "" {
		channel = invalidationChannel
	}

	dc := &DistributedCache{
		local:   local,
		pubsub:  config.PubSub,
		channel: channel,
		subs:    make(map[string]func(prefix string) error),
	}

	return dc, nil
}

// SubscribeInvalidation registers a handler to be called when a prefix is invalidated
func (d *DistributedCache) SubscribeInvalidation(ctx context.Context, prefix string, handler func(prefix string) error) error {
	d.mu.Lock()
	d.subs[prefix] = handler
	d.mu.Unlock()

	return nil
}

// StartListening begins listening for invalidation messages from other instances
func (d *DistributedCache) StartListening(ctx context.Context) error {
	if d.pubsub == nil {
		return fmt.Errorf("pubsub not configured")
	}

	pubsub := d.pubsub.Subscribe(ctx, d.channel)
	go func() {
		ch := pubsub.Channel()
		for msg := range ch {
			d.handleInvalidation(msg.Payload)
		}
	}()

	return nil
}

func (d *DistributedCache) handleInvalidation(payload string) {
	var msg invalidationMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for prefix, handler := range d.subs {
		if matchPattern(msg.Prefix, prefix) {
			handler(msg.Prefix)
		}
	}
}

func matchPattern(key, pattern string) bool {
	if pattern == "" {
		return true
	}
	if len(pattern) > len(key) {
		return false
	}
	return key[:len(pattern)] == pattern
}

type invalidationMessage struct {
	Prefix string    `json:"prefix"`
	Keys   []string  `json:"keys,omitempty"`
	Time   time.Time `json:"time"`
}

func (d *DistributedCache) Invalidate(ctx context.Context, keys ...string) error {
	if err := d.local.Delete(ctx, keys...); err != nil {
		return err
	}

	if d.pubsub != nil {
		msg := invalidationMessage{
			Keys: keys,
			Time: time.Now(),
		}
		data, _ := json.Marshal(msg)
		d.pubsub.Publish(ctx, d.channel, data)
	}

	return nil
}

func (d *DistributedCache) InvalidateByPrefix(ctx context.Context, prefix string) error {
	if err := d.local.InvalidateByPrefix(ctx, prefix); err != nil {
		return err
	}

	if d.pubsub != nil {
		msg := invalidationMessage{
			Prefix: prefix,
			Time:   time.Now(),
		}
		data, _ := json.Marshal(msg)
		d.pubsub.Publish(ctx, d.channel, data)
	}

	return nil
}

func (d *DistributedCache) Get(ctx context.Context, key string, dest interface{}) error {
	return d.local.Get(ctx, key, dest)
}

func (d *DistributedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return d.local.Set(ctx, key, value, ttl)
}

func (d *DistributedCache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, fn func() (interface{}, error)) error {
	return d.local.GetOrSet(ctx, key, dest, ttl, fn)
}

func (d *DistributedCache) Metrics() *Metrics {
	return d.local.Metrics()
}

func (d *DistributedCache) ResetMetrics() {
	d.local.ResetMetrics()
}

func (d *DistributedCache) HitRate() float64 {
	return d.local.HitRate()
}
