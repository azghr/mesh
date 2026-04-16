# cache

Redis-backed caching with automatic metrics and common caching patterns.

## What It Does

Provides a clean caching layer on top of Redis with:
- Automatic JSON serialization
- Hit/miss metrics built-in
- Cache-aside pattern helper
- Batch operations
- Prefix-based invalidation
- Deduplication to prevent cache stampedes

## Usage

### Basic Setup

```go
cache, err := cache.New(redisClient, 5*time.Minute)
// redisClient is *redis.Client from the redis package
```

### Get/Set

```go
// Store a value
err := cache.Set(ctx, "user:123", user, 10*time.Minute)

// Retrieve a value
var user User
err := cache.Get(ctx, "user:123", &user)
if err == cache.ErrCacheMiss {
    // Not in cache
}
```

### Cache-Aside (GetOrSet)

The most common pattern - check cache first, fetch from source on miss:

```go
var user User
err := cache.GetOrSet(ctx, "user:123", &user, time.Hour, func() (any, error) {
    return db.FindUser(ctx, "123")
})
// If cached: returns immediately
// If not cached: calls fn(), caches result, returns it
```

### Deduplication (Stampede Protection)

Prevents the "thundering herd" problem when a popular cache key expires. Multiple concurrent requests for the same key are deduplicated - only one executes the fetch function, others wait for the result.

```go
// Simple deduplication for any function
d := cache.NewDedup()
val, err := d.Do("expensive-key", func() (interface{}, error) {
    return expensiveComputation()
})
// Multiple calls with same key only execute once

// Deduplication with context support
val, err := d.DoCtx(ctx, "key", func(ctx context.Context) (interface{}, error) {
    return fetchFromDB(ctx, key)
})

// Full cache-aside with dedup (recommended for production)
getFn := func(ctx context.Context, key string) (interface{}, error) {
    var val interface{}
    err := myCache.Get(ctx, key, &val)
    return val, err
}
setFn := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
    return myCache.Set(ctx, key, val, ttl)
}
store := cache.NewDedupStore(getFn, setFn)

val, err := store.Fetch(ctx, "user:123", time.Hour, func(ctx context.Context) (interface{}, error) {
    return db.FindUser(ctx, "123")
})
// - First request: cache miss → executes fn → caches result
// - Concurrent requests: wait for first result → return same value
// - Subsequent requests: cache hit → return immediately
```

### Metrics

```go
// Check cache performance
metrics := cache.Metrics()
log.Printf("hits: %d, misses: %d, hit_rate: %.2f%%", 
    metrics.Hits, metrics.Misses, cache.HitRate())

// Check deduplication stats
dedupMetrics := d.Metrics()
log.Printf("dedup calls: %d, duplicates: %d", 
    dedupMetrics.Calls, dedupMetrics.Duplicates)

// Reset for fresh metrics
cache.ResetMetrics()
```

### Invalidation

```go
// Delete specific keys
cache.Delete(ctx, "user:123", "user:456")

// Delete all keys with a prefix (e.g., "user:*")
cache.InvalidateByPrefix(ctx, "user:")
```

### Batch Operations

```go
// Get multiple keys
results := make(map[string]interface{})
cache.MultiGet(ctx, []string{"user:1", "user:2", "user:3"}, results)

// Set multiple keys
cache.MultiSet(ctx, map[string]interface{}{
    "user:1": user1,
    "user:2": user2,
}, 10*time.Minute)
```

## Custom Key Prefix

```go
// Add a prefix to all keys (default: "cache:")
cache, _ := cache.NewWithPrefix(redisClient, 5*time.Minute, "myapp:")
// Keys become "myapp:user:123"
```

## Why Deduplication?

Without dedup, when a cache key expires:
1. Request A misses cache → calls DB
2. Request B misses cache → calls DB (same data)
3. Request C misses cache → calls DB (same data)
4. ... 1000 more requests ...

This "cache stampede" can overwhelm your database. With dedup:
1. Request A misses cache → calls DB
2. Request B-H all wait for Request A's result
3. All get the same cached result

## Configuration

```go
type Metrics struct {
    Hits    int64 // Cache hits
    Misses  int64 // Cache misses
    Sets    int64 // Cache sets
    Deletes int64 // Cache deletes
    Errors  int64 // Operation errors
}

type DedupMetrics struct {
    Calls      int64 // Total deduplicated calls
    Duplicates int64 // Calls that waited for others
}
```

The cache tracks hits, misses, sets, deletes, and errors automatically. Call `HitRate()` to get percentage of successful lookups.