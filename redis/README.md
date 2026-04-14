# redis

Redis client with connection pooling and health checks.

## What It Does

A thin wrapper around the `redis/go-redis` library that handles connection pooling, health checks, and provides a clean API. Designed to work seamlessly with the `cache` package.

## Usage

### Creating a Client

```go
client, err := redis.NewClient(redis.Config{
    Host:     "localhost",
    Port:     6379,
    Password: "",           // No password
    DB:       0,           // Database 0
    PoolSize: 10,           // Max connections
    MinIdleConns: 5,        // Min idle connections
})
if err != nil {
    return err
}
defer client.Close()
```

### Access the Raw Client

```go
// Get the underlying *redis.Client for direct commands
r := client.Client()
r.Set(ctx, "key", "value", time.Hour)
val, err := r.Get(ctx, "key").Result()
```

### Health Checks

```go
// Simple ping
err := client.Ping(ctx)

// Full health check
err := client.HealthCheck(ctx)
```

### Basic Operations

```go
// String operations
err := client.Set(ctx, "key", "value", time.Hour)
val, err := client.Get(ctx, "key")

// Delete
err := client.Del(ctx, "key1", "key2")

// Check existence
count, err := client.Exists(ctx, "key")

// Set expiration
err := client.Expire(ctx, "key", 30*time.Minute)

// Get TTL
ttl, err := client.TTL(ctx, "key")
```

### Key Operations

```go
// Find keys matching pattern
keys, err := client.Keys(ctx, "user:*")

// Scan through keys (better for large datasets)
var cursor uint64
for {
    keys, cursor, err = client.Scan(ctx, cursor, "user:*", 100)
    if err != nil {
        break
    }
    // Process keys
    if cursor == 0 {
        break
    }
}
```

### Server Info

```go
// Get server info
info, err := client.Info(ctx)
// Or specific section
info, err := client.Info(ctx, "memory")

// Get number of keys
size, err := client.DBSize(ctx)

// Clear current database (use with caution!)
err := client.FlushDB(ctx)
```

### Monitoring

```go
// Get connection pool stats
stats := client.PoolStats()
log.Printf("Conns: total=%d, idle=%d, in_use=%d", 
    stats.TotalConns, 
    stats.IdleConns, 
    stats.InUseConns)

// Get config
cfg := client.Config()
log.Printf("Connected to %s:%d", cfg.Host, cfg.Port)
```

## Configuration

```go
type Config struct {
    Host         string        // Redis host (default: "localhost")
    Port         int          // Redis port (default: 6379)
    Password     string        // Redis password (default: "")
    DB           int          // Database number (default: 0)
    PoolSize     int          // Max connections per pool (default: 10)
    MinIdleConns int          // Min idle connections (default: 5)
    MaxRetries   int          // Max retries on error (default: 3)
    DialTimeout  time.Duration // Connection timeout (default: 5s)
    ReadTimeout  time.Duration // Read timeout (default: 3s)
    WriteTimeout time.Duration // Write timeout (default: 3s)
    PoolTimeout  time.Duration // Pool connection timeout (default: 4s)
    IdleTimeout  time.Duration // Idle connection timeout (default: 5m)
}
```