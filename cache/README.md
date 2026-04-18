# cache

Redis-backed caching with automatic metrics and common caching patterns.

## What It Does

Provides a clean caching layer on top of Redis with:
- Automatic JSON serialization
- Hit/miss metrics built-in
- Cache-aside pattern helper
- Batch operations
- Prefix-based invalidation

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

### Metrics

```go
// Check cache performance
metrics := cache.Metrics()
log.Printf("hits: %d, misses: %d, hit_rate: %.2f%%", 
    metrics.Hits, metrics.Misses, cache.HitRate())

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

## Configuration

```go
// Configuration for new cache
cache, err := cache.NewWithConfig(redisClient, cache.Config{
    DefaultTTL:      5*time.Minute,
    KeyPrefix:       "cache:",
    StampedeEnabled: true,         // Enable stampede protection
    StampedeTTL:     100*time.Millisecond,  // Lock timeout
    StampedeRetries: 3,            // Max lock acquisition retries
})
```

### Stampede Protection

When multiple requests hit a cache miss for the same key simultaneously, only one request fetches the data while others wait:

```go
cache, _ := cache.NewWithConfig(redisClient, cache.Config{
    StampedeEnabled: true,
    StampedeTTL:     100*time.Millisecond,
})

// Request 1: cache miss, acquires lock, fetches from DB
// Request 2: cache miss, lock exists, waits for result
// Request 3: cache miss, lock exists, waits for result
// ... once Request 1 completes, all get the cached result
```

### Metrics

```go
type Metrics struct {
    Hits    int64 // Cache hits
    Misses  int64 // Cache misses
    Sets    int64 // Cache sets
    Deletes int64 // Cache deletes
    Errors  int64 // Operation errors
}
```

The cache tracks hits, misses, sets, deletes, and errors automatically. Call `HitRate()` to get percentage of successful lookups.