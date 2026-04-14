# mesh

A Go toolkit for building production-grade microservices. Provides the common infrastructure code you need so you can focus on business logic.

## What This Is

Think of mesh as the foundation beneath your service. It handles the undifferentiated heavy lifting—the stuff every service needs but nobody enjoys writing:

- **Database connections** with pooling, transactions, and query helpers
- **Error handling** that maps to HTTP/gRPC status codes automatically  
- **Resilient HTTP clients** with circuit breakers and retry logic
- **Redis caching** with built-in metrics and cache-aside pattern
- **Structured logging** for development (pretty terminal) and production (JSON)
- **Configuration** from YAML files with environment variable overrides

## Why Use It

You're building a microservice. You need a database, caching, some HTTP clients, and observability. You could:

1. Write all this yourself (time-consuming, easy to get wrong)
2. Use a full framework (adds a lot of baggage you might not need)
3. Use mesh (lean, composable, you pick what you need)

Mesh is deliberately small and focused. Import only what you need. The packages are independent—no giant dependency tree to drag in.

## Installation

```bash
go get github.com/azghr/mesh
```

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/azghr/mesh/config"
    "github.com/azghr/mesh/database"
    "github.com/azghr/mesh/cache"
    "github.com/azghr/mesh/logger"
    "github.com/azghr/mesh/http"
    "github.com/azghr/mesh/redis"
)

func main() {
    // Load configuration from YAML + environment variables
    cfg, err := config.Load("config.yaml", config.WithDefaultConfig())
    if err != nil {
        log.Fatal(err)
    }

    // Structured logging
    log := logger.New("my-service", "debug", false)
    log.Info("service starting", "port", cfg.Server.Port)

    // Database connection pool
    pool, err := database.NewPool(database.Config{
        Host: cfg.Database.Host,
        Port: cfg.Database.PortInt,
        User: cfg.Database.User,
        Password: cfg.Database.Password,
        Name: cfg.Database.Name,
        MaxOpenConns: 25,
    })
    if err != nil {
        log.Fatal("database connection failed", "error", err)
    }
    defer pool.Close()

    // Redis client
    redisClient, err := redis.NewClient(redis.Config{
        Host: cfg.Redis.Host,
        Port: cfg.Redis.Port,
    })
    if err != nil {
        log.Fatal("redis connection failed", "error", err)
    }
    defer redisClient.Close()

    // Cache layer
    myCache, _ := cache.New(redisClient.Client(), 5*time.Minute)

    // Resilient HTTP client with circuit breaker + retry
    client := http.NewResilientClient(http.DefaultResilientClientConfig("external-api"))

    // Use them...
    var user User
    err = myCache.GetOrSet(ctx, "user:123", &user, time.Hour, func() (any, error) {
        return findUser(ctx, pool.DB(), "123")
    })
}
```

## Packages Overview

| Package | Purpose | Key Types/Functions |
|---------|---------|---------------------|
| `config` | Load and validate configuration | `Load()`, `GetEnv()`, `ValidateProduction()` |
| `database` | PostgreSQL connection pool | `NewPool()`, `WithTransaction()`, `ScanRows()` |
| `cache` | Redis caching with metrics | `GetOrSet()`, `InvalidateByPrefix()`, `HitRate()` |
| `errors` | Structured errors → HTTP/gRPC | `NotFoundError()`, `ToHTTPStatus()`, `ToGRPCStatus()` |
| `logger` | Structured logging | `New()`, `With()`, `FormatServiceName()` |
| `http` | Resilient HTTP patterns | `ResilientClient`, `CircuitBreaker`, `Retry()` |
| `redis` | Redis client wrapper | `NewClient()`, `Ping()`, `Keys()` |
| `health` | Health checks for k8s | `Checker`, `Register()`, `Status()` |
| `middleware` | HTTP middleware | `Logging()`, `Recovery()`, `RateLimit()` |
| `auth` | JWT + RBAC | `RBAC`, `RequirePermission()`, `HasPermission()` |
| `telemetry` | Observability | `InitTracing()`, `InitMetrics()`, `RecordHTTPRequest()` |
| `lock` | Distributed locking | `RedisLock`, `Execute()`, `Acquire()` |
| `workerpool` | Goroutine pool | `New()`, `Submit()`, `Shutdown()` |
| `shutdown` | Graceful shutdown | `Manager`, `Register()`, `WaitForSignal()` |
| `eventbus` | Pub/sub events | `Bus`, `Subscribe()`, `Publish()` |

## Key Patterns

### Circuit Breaker

Prevents cascading failures when downstream services are down. Automatically stops calling a failing service until it recovers.

```go
cb := http.NewCircuitBreaker(nil)
err := cb.Execute(func() error {
    return callExternalService()
})
if err != nil {
    // Service is unavailable, don't even try
}
```

### Cache-Aside

Check cache first, fetch from database on miss, store in cache.

```go
var user User
err := cache.GetOrSet(ctx, "user:"+id, &user, time.Hour, func() (any, error) {
    return db.FindUser(ctx, id)
})
```

### Error Handling

Return structured errors that automatically map to HTTP status codes.

```go
// In handlers
if user == nil {
    return errors.NotFoundError("user", id)
}

// Maps to HTTP 404 automatically
http.Status = err.ToHTTPStatus()
```

### RBAC Permissions

Check if a user has permission before allowing an action.

```go
rbac := auth.NewRBAC(roleStore) // or nil for default

// HTTP middleware
rbac.RequirePermission(perm.PermWalletWrite)(next)

// Direct check
err := rbac.CheckPermission(ctx, userID, perm.PermWalletRead)
```

## Configuration

YAML base with environment variable overrides:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  environment: development

database:
  host: localhost
  port: 5432
  port_int: 5432
  user: app
  name: myapp
  ssl_mode: disable
  max_open_conns: 25
  max_idle_conns: 5

redis:
  host: localhost
  port: 6379
  db: 0

log:
  level: info
  json_format: false
```

Override with environment variables: `DB_HOST`, `DB_PORT`, `DB_NAME`, `REDIS_HOST`, etc.

## Project Structure

```
mesh/
├── config/         # Configuration loading
├── database/       # PostgreSQL utilities
├── cache/          # Redis caching
├── errors/         # Structured errors
├── logger/         # Structured logging
├── http/           # Circuit breaker + retry
├── redis/          # Redis client
├── health/         # Health checks
├── middleware/     # HTTP middleware
├── auth/           # JWT + RBAC
├── telemetry/      # Metrics + tracing
├── lock/           # Distributed locks
├── workerpool/     # Goroutine pool
├── shutdown/       # Graceful shutdown
├── eventbus/       # Pub/sub events
└── circuitbreaker/ # CB monitoring
```

## Testing

```bash
go test ./...
```

## License

MIT