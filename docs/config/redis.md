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

## Cluster Support

Redis Cluster provides automatic sharding and high availability for production deployments.

### Creating a Cluster Client

```go
cluster, err := redis.NewCluster(redis.ClusterConfig{
    Addrs:    []string{
        "localhost:7000",
        "localhost:7001",
        "localhost:7002",
    },
    PoolSize:     10,
    MinIdleConns:  5,
    Password:     "cluster-pass",
    MaxRetries:    3,
})
if err != nil {
    return err
}
defer cluster.Close()
```

### Cluster Configuration

```go
type ClusterConfig struct {
    Addrs         []string       // Cluster node addresses
    PoolSize     int           // Max connections per node (default: 10)
    MinIdleConns int           // Min idle connections per node (default: 5)
    Password    string        // Password for authentication
    MaxRetries  int           // Maximum number of retries (default: 3)
    DialTimeout time.Duration // Connection timeout (default: 5s)
    ReadTimeout time.Duration // Read timeout (default: 3s)
    WriteTimeout time.Duration // Write timeout (default: 3s)
    PoolTimeout time.Duration // Pool connection timeout (default: 4s)
}
```

### Cluster Operations

```go
// Same API as regular client
err := cluster.Set(ctx, "key", "value", time.Hour)
val, err := cluster.Get(ctx, "key")

// Cluster-specific operations
err := cluster.ForEachNode(ctx, func(ctx context.Context, node *redis.Client) error {
    // Execute on each master node
    return nil
})

slots, err := cluster.GetClusterSlots(ctx)
```

## Sentinel Support

Redis Sentinel provides automatic failover for high availability.

### Creating a Sentinel Client

```go
sentinel, err := redis.NewSentinel(redis.SentinelConfig{
    MasterName:      "mymaster",
    SentinelAddrs:   []string{
        "localhost:26379",
        "localhost:26380",
    },
    SentinelPassword: "sentinel-pass",
    Password:       "master-pass",
    PoolSize:       10,
})
if err != nil {
    return err
}
defer sentinel.Close()
```

### Sentinel Configuration

```go
type SentinelConfig struct {
    MasterName       string        // Master name to monitor
    SentinelAddrs   []string     // Sentinel addresses
    SentinelPassword string     // Sentinel password
    Password       string       // Master password
    DB             int          // Default DB (default: 0)
    PoolSize       int          // Max connections (default: 10)
    MinIdleConns   int          // Min idle connections (default: 5)
    DialTimeout   time.Duration // Connection timeout (default: 5s)
    ReadTimeout   time.Duration // Read timeout (default: 3s)
    WriteTimeout time.Duration // Write timeout (default: 3s)
    PoolTimeout   time.Duration // Pool connection timeout (default: 4s)
}
```

### Sentinel Operations

```go
// Same API as regular client - automatically routes to master
err := sentinel.Set(ctx, "key", "value", time.Hour)
val, err := sentinel.Get(ctx, "key")

// Get master info
masterName := sentinel.GetMasterAddr(ctx)
```
```