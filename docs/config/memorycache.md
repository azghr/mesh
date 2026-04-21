# memorycache

Thread-safe in-memory cache with TTL and LRU eviction.

## What It Does

Provides a simple in-memory key-value cache ideal for:
- Caching within a single process
- L1 cache layer in front of Redis
- Configuration and lookup tables
- High-frequency data access

Features:
- Thread-safe operations
- TTL (time-to-live) for entries
- LRU eviction when max size is reached
- GetOrSet cache-aside pattern
- Hit/miss metrics

## Usage

### Basic Setup

```go
cache := memorycache.New(
    memorycache.WithMaxSize(1000),
    memorycache.WithTTL(time.Hour),
)
```

### Get/Set

```go
// Store a value
cache.Set("user:123", user, 10*time.Minute)

// Retrieve a value
user, err := cache.Get("user:123")
if err == memorycache.ErrNotFound {
    // Not in cache
}
```

### GetOrSet (Cache-Aside)

The most common pattern - check cache first, fetch from source on miss:

```go
var user User
val, err := cache.GetOrSet(ctx, "user:123", func() (any, error) {
    return db.FindUser(ctx, "123")
}, time.Hour)
// If cached: returns immediately
// If not cached: calls fn(), caches result, returns it
```

### Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxSize` | 10000 | Maximum entries |
| `WithTTL` | 5min | Default TTL |

### Metrics

```go
metrics := cache.GetMetrics()
log.Printf("size=%d hits=%d misses=%d", 
    metrics.Size, metrics.Hits, metrics.Misses)

// Reset counters
cache.ResetMetrics()
```

### Deletion

```go
cache.Delete("user:123")

// Clear all
cache.Clear()
```

## When to Use

- **High-frequency data**: User profiles, config, lookup tables
- **L1 cache**: In front of Redis for faster access
- **Single instance**: Not for distributed caching

## Example with Fiber

```go
cache := memorycache.New(
    memorycache.WithMaxSize(10000),
    memorycache.WithTTL(5*time.Minute),
)

app.Get("/users/:id", func(c *fiber.Ctx) error {
    id := c.Params("id")
    
    var user User
    val, err := cache.GetOrSet(c.UserContext(), "user:"+id, func() (any, error) {
        return db.FindUser(c.UserContext(), id)
    }, 5*time.Minute)
    
    if err != nil {
        return err
    }
    
    return c.JSON(val)
})
```

## LRU Eviction

When the cache reaches `MaxSize`, the least recently used entry is automatically evicted to make room for new entries.

## TTL Expiration

Entries automatically expire after their TTL. Expired entries return `ErrNotFound` on access.