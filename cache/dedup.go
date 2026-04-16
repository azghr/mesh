// Package cache provides Redis-backed caching utilities with stampede protection.
//
// This package implements a caching layer with automatic serialization,
// hit/miss metrics, and common caching patterns like cache-aside.
package cache

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// DedupMetrics tracks deduplication statistics.
type DedupMetrics struct {
	Calls      int64 // Total calls to Do()
	Duplicates int64 // Calls that waited for another caller
}

// Dedup provides request deduplication to prevent cache stampedes.
// When multiple goroutines request the same key simultaneously,
// only one executes the function; others wait for the result.
//
// Also known as "request coalescing" or "key consolidation".
//
// Example:
//
//	d := cache.NewDedup()
//	val, err := d.Do("expensive-key", func() (interface{}, error) {
//	    return expensiveComputation()
//	})
type Dedup struct {
	group   singleflight.Group
	metrics *DedupMetrics
}

// NewDedup creates a new deduplicator.
func NewDedup() *Dedup {
	return &Dedup{
		metrics: &DedupMetrics{},
	}
}

// Do executes fn for key, deduplicating concurrent calls.
// Only one execution happens; all callers receive the same result.
//
// Metrics track calls and duplicates:
// - Calls: total invocations
// - Duplicates: calls that waited for another goroutine
func (d *Dedup) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	atomic.AddInt64(&d.metrics.Calls, 1)
	v, err, shared := d.group.Do(key, fn)
	if shared {
		atomic.AddInt64(&d.metrics.Duplicates, 1)
	}
	return v, err
}

// DoCtx executes fn with context, deduplicating concurrent calls.
func (d *Dedup) DoCtx(ctx context.Context, key string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	atomic.AddInt64(&d.metrics.Calls, 1)

	wrapper := func() (interface{}, error) {
		return fn(ctx)
	}

	v, err, shared := d.group.Do(key, wrapper)
	if shared {
		atomic.AddInt64(&d.metrics.Duplicates, 1)
	}
	return v, err
}

// Metrics returns deduplication statistics.
func (d *Dedup) Metrics() *DedupMetrics {
	return d.metrics
}

// StoreGetFn retrieves a value from cache.
type StoreGetFn func(ctx context.Context, key string) (interface{}, error)

// StoreSetFn stores a value in cache.
type StoreSetFn func(ctx context.Context, key string, val interface{}, ttl time.Duration) error

// DedupStore combines caching with deduplication.
// It prevents cache stampedes by ensuring only one goroutine
// fetches data on cache miss.
//
// Example:
//
//	get := func(ctx context.Context, key string) (interface{}, error) {
//	    var val interface{}
//	    err := cache.Get(ctx, key, &val)
//	    return val, err
//	}
//	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
//	    return cache.Set(ctx, key, val, ttl)
//	}
//	store := cache.NewDedupStore(get, set)
//
//	val, err := store.Fetch(ctx, "user:123", time.Hour, func(ctx context.Context) (interface{}, error) {
//	    return db.FindUser(ctx, "123")
//	})
type DedupStore struct {
	get   StoreGetFn
	set   StoreSetFn
	dedup *Dedup
}

// NewDedupStore creates a deduplicating cache store.
func NewDedupStore(get StoreGetFn, set StoreSetFn) *DedupStore {
	return &DedupStore{
		get:   get,
		set:   set,
		dedup: NewDedup(),
	}
}

// Fetch retrieves a value, computing it on cache miss.
// Deduplicates concurrent requests for the same key.
func (ds *DedupStore) Fetch(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	val, err := ds.get(ctx, key)
	if err == nil {
		return val, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	val, err = ds.dedup.DoCtx(ctx, key, fn)
	if err != nil {
		return nil, err
	}

	if cacheErr := ds.set(ctx, key, val, ttl); cacheErr != nil {
		return nil, cacheErr
	}

	return val, nil
}

// FetchInto fetches a value and unmarshals it into dest.
func (ds *DedupStore) FetchInto(ctx context.Context, key string, dest interface{}, ttl time.Duration, fn func(context.Context) (interface{}, error)) error {
	val, err := ds.Fetch(ctx, key, ttl, fn)
	if err != nil {
		return err
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}
