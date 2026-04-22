# ratelimiter

Distributed rate limiting using Redis with sliding window algorithm.

## What It Does

Provides rate limiting that works across multiple service instances:
- **Redis-backed** - State shared across all instances
- **Sliding window** - More accurate than fixed windows
- **Per-key limits** - Limit by IP, user, endpoint, or custom key

## Usage

### SimpleRateLimiter (Single Instance)

For development or single-instance deployments:

```go
import "github.com/azghr/mesh/ratelimiter"

limiter := ratelimiter.NewSimpleRateLimiter(100, time.Minute)

allowed, err := limiter.Allow(ctx, "192.168.1.1")
if !allowed {
    // Rate limited
}
```

### RedisRateLimiter (Distributed)

For multi-instance deployments:

```go
import (
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/redis"
)

redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
limiter := ratelimiter.NewRedisRateLimiter(redisClient.Client(), 100, time.Minute)

allowed, err := limiter.Allow(ctx, "user:123")
if !allowed {
    // Rate limited
}
```

### Check Multiple Requests

```go
// Allow 5 requests at once
allowed, err := limiter.AllowN(ctx, "key", 5)
```

### Get Current Usage

```go
current, remaining, err := limiter.GetLimit(ctx, "key")
fmt.Printf("Used: %d, Remaining: %d\n", current, remaining)
```

### Reset a Key

```go
err := limiter.Reset(ctx, "user:123")
```

## Algorithms

### Sliding Window (Redis)

Uses Redis sorted sets to track requests with precise timestamps:

1. Remove expired entries (outside window)
2. Count current entries
3. Add new entry with current timestamp
4. Check if under limit

```bash
# Example Redis structure
ZADD ratelimit:192.168.1.1 1714567890000 "req-1"
ZADD ratelimit:192.168.1.1 1714567891000 "req-2"
ZREMRANGEBYSCORE ratelimit:192.168.1.1 0 1714567230000
ZCARD ratelimit:192.168.1.1  # = 2
```

## Configuration

```go
// Sliding window: 100 requests per minute
limiter := ratelimiter.NewRedisRateLimiter(client, 100, time.Minute)

// Higher throughput: 1000 requests per second
limiter := ratelimiter.NewRedisRateLimiter(client, 1000, time.Second)

// Custom key prefix
limiter := ratelimiter.NewRedisRateLimiter(client, 100, time.Minute,
    ratelimiter.WithKeyPrefix("myapp"))
```

## Token Bucket Algorithm

The token bucket algorithm allows bursts while enforcing an average rate:

- **Burst**: Can handle sudden spikes up to bucket size
- **Rate**: Refills tokens at this speed per second
- **Use cases**: APIs with occasional bursts (file uploads, data imports)

### TokenBucketLimiter

```go
// Token bucket: 100 requests/second, burst up to 10
limiter := ratelimiter.NewTokenBucketLimiter(client, ratelimiter.TokenBucketConfig{
    Rate:      100.0,  // tokens per second
    Burst:     10,     // max bucket size
    KeyPrefix: "ratelimit:",
})

allowed, err := limiter.Allow(ctx, "user:123")
// - First 10 requests allowed (burst)
// - Then limited to 100/second average
```

### When to Use Token Bucket

| Scenario | Algorithm | Why |
|----------|------------|-----|
| API with bursts | Token Bucket | Handle spikes gracefully |
| Consistent traffic | Sliding Window | Precise rate limiting |
| Strict limits | Sliding Window | No burst tolerance |
| File uploads | Token Bucket | Short bursts allowed |

### Configuration

```go
// Burst-heavy workload (e.g., batch processing)
limiter := ratelimiter.NewTokenBucketLimiter(client, TokenBucketConfig{
    Rate:      50.0,
    Burst:     100,   // large burst for batch jobs
})

// Tight limits (e.g., public API)
limiter := ratelimiter.NewTokenBucketLimiter(client, TokenBucketConfig{
    Rate:      10.0,
    Burst:     1,     // no burst tolerance
})
```

## Interface

```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) (bool, error)
    AllowN(ctx context.Context, key string, n int) (bool, error)
    Reset(ctx context.Context, key string) error
    GetLimit(ctx context.Context, key string) (int, int, error)
}
```

## Best Practices

1. **Use Redis in production** - In-memory limiter doesn't work with multiple instances
2. **Set reasonable limits** - Start conservative, adjust based on traffic patterns
3. **Include helpful headers** - Tell clients when they can retry
4. **Monitor metrics** - Track allowed vs rejected requests
5. **Consider per-user limits** - More fair than per-IP (handles shared IPs)

## Example: Full Setup

```go
import (
    "github.com/azghr/mesh/redis"
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/middleware"
)

func setupRateLimiting() (*middleware.LimitMiddleware, error) {
    // Redis client
    redisClient, err := redis.NewClient(redis.Config{
        Host: "localhost",
        Port: 6379,
    })
    if err != nil {
        return nil, err
    }

    // Distributed rate limiter: 100 requests per minute per IP
    limiter := ratelimiter.NewRedisRateLimiter(
        redisClient.Client(),
        100,
        time.Minute,
    )

    // Create middleware with custom key function
    mw := middleware.NewLimit(limiter)
    mw.KeyFunc(func(c *fiber.Ctx) string {
        // Try user-based limiting first
        if userID := c.Locals("user_id"); userID != nil {
            return "user:" + userID.(string)
        }
        // Fall back to IP
        return "ip:" + c.IP()
    })

    return mw, nil
}

## Adaptive Rate Limiter

Rate limiter that automatically adjusts based on latency.

### Basic Usage

```go
import "github.com/azghr/mesh/ratelimiter"

limiter := ratelimiter.NewAdaptiveLimiter(redisClient,
    ratelimiter.WithBaseRate(1000),      // Start at 1000 rps
    ratelimiter.WithMinRate(100),         // Minimum 100 rps
    ratelimiter.WithMaxRate(10000),       // Maximum 10000 rps
    ratelimiter.WithTargetLatency(100*time.Millisecond),
    ratelimiter.WithAdjustmentStep(100),   // Adjust by 100 each time
)

// Record latency after each request
start := time.Now()
resp, _ := client.Do(req)
latency := time.Since(start)
limiter.RecordLatency(ctx, "api", latency)

// Check allowed
allowed, err := limiter.Allow(ctx, "api")
```

### How It Works

1. Starts at BaseRate
2. Monitors average latency
3. Increases rate if latency < TargetLatency
4. Decreases rate if latency > TargetLatency
5. Clamps to MinRate/MaxRate

### Options

| Option | Description | Default |
|-------|-------------|---------|
| `WithBaseRate` | Starting rate | 1000 |
| `WithMinRate` | Minimum rate | 100 |
| `WithMaxRate` | Maximum rate | 10000 |
| `WithTargetLatency` | Target latency | 100ms |
| `WithAdjustmentStep` | Rate adjustment | 100 |
| `WithAdaptiveWindow` | Measurement window | 1 min |

### API

| Method | Description |
|-------|-------------|
| `Allow(ctx, key)` | Check if request allowed |
| `RecordLatency(ctx, key, latency)` | Record request latency |
| `GetRate(ctx, key)` | Get current adjusted rate |
| `GetStats(ctx)` | Get limiter statistics |
| `ForceRate(rate)` | Manually set rate |

## Distributed Rate Limiter

Per-user, per-IP, and per-API key rate limiting with multi-dimensional support.

### Basic Usage

```go
limiter := ratelimiter.NewDistributedLimiter(redisClient,
    ratelimiter.WithDistDefaultRate(100),      // Default: 100/min
    ratelimiter.WithDistDefaultWindow(time.Minute),
    ratelimiter.WithPerUserRate(1000),        // Per-user: 1000/min
    ratelimiter.WithPerIPRate(100),            // Per-IP: 100/min
    ratelimiter.WithPerAPIKeyRate(5000),       // Per-API key: 5000/min
)

// Simple check (uses key as-is)
allowed, err := limiter.Allow(ctx, "api:login")

// Check with dimensions (user, IP, API key)
allowed, err := limiter.AllowWithDims(ctx, "api:login", userID, clientIP, apiKey)
```

### Client IP Extraction

```go
// Get client IP from headers or remote address
clientIP := ratelimiter.GetClientIP(
    c.Get("X-Forwarded-For"),  // X-Forwarded-For header
    c.IP(),                   // Fiber's IP() (remote addr)
)

// Validate IP
if ratelimiter.IsValidIP(ip) {
    // Valid IP address
}
```

### Multi-Dimensional Limiting

Combine multiple rate limit dimensions:

```go
dims := []ratelimiter.DimConfig{
    {Name: "requests", Rate: 100, Window: time.Minute},
    {Name: "bandwidth", Rate: 1000, Window: time.Minute},
}

multiLimiter := ratelimiter.NewMultiDimLimiter(redisClient, dims)

// Check all dimensions
allowed, results, err := multiLimiter.Allow(ctx, "upload:user123")
// results["requests"] = true/false
// results["bandwidth"] = true/false
```

### Options

| Option | Description | Default |
|-------|-------------|---------|
| `WithDistDefaultRate` | Default rate | 100 |
| `WithDistDefaultWindow` | Default window | 1 min |
| `WithDistKeyPrefix` | Redis key prefix | "ratelimit" |
| `WithPerUserRate` | Per-user rate limit | 1000 |
| `WithPerIPRate` | Per-IP rate limit | 100 |
| `WithPerAPIKeyRate` | Per-API key rate limit | 5000 |

### API

| Method | Description |
|-------|-------------|
| `Allow(ctx, key)` | Check with default limits |
| `AllowWithDims(ctx, key, user, ip, apiKey)` | Check with all dimensions |
| `AllowN(ctx, key, n)` | Check n requests |
| `GetLimits(ctx, key, user, ip, apiKey)` | Get limits for all dimensions |
| `Reset(ctx, keys...)` | Reset rate limits |

### Best Practices

1. **Layer limits**: Per-APIKey > Per-User > Per-IP > Default
2. **Set appropriate rates**: API keys can have higher limits
3. **Extract client IP correctly**: Check X-Forwarded-For, X-Real-IP, then remote
4. **Monitor all dimensions**: Track per-user and per-IP separately
```
