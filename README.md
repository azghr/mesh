# mesh

A Go toolkit for building production-grade services. Provides the common infrastructure code you need so you can focus on business logic.

## What This Is

Think of mesh as the foundation beneath your application. It handles the undifferentiated heavy lifting—the stuff every service needs but nobody enjoys writing:

- **Database connections** with pooling, transactions, and query helpers
- **Error handling** that maps to HTTP/gRPC status codes automatically  
- **Resilient HTTP clients** with circuit breakers and retry logic
- **Redis caching** with built-in metrics, cache-aside pattern, and stampede protection
- **In-memory caching** for fast local access
- **Distributed rate limiting** with sliding window algorithm (per-IP, per-user)
- **Structured logging** for development (pretty terminal) and production (JSON)
- **Configuration** from YAML files with environment variable overrides

## Why Use It

You're building a service. You need a database, caching, some HTTP clients, and observability. You could:

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

### Core
| Package | Purpose | Docs |
|---------|---------|------|
| `config` | YAML config + env overrides | [config.md](docs/config/config.md) |
| `database` | PostgreSQL pool + queries | [database.md](docs/config/database.md) |
| `errors` | Structured errors → HTTP/gRPC | [errors.md](docs/config/errors.md) |
| `logger` | Structured logging | [logger.md](docs/config/logger.md) |
| `json` | Fast JSON serialization | [json.md](docs/config/json.md) |

### Caching
| Package | Purpose | Docs |
|---------|---------|------|
| `cache` | Redis caching + metrics | [cache.md](docs/config/cache.md) |
| `memorycache` | In-memory LRU cache | [memorycache.md](docs/config/memorycache.md) |

### Networking
| Package | Purpose | Docs |
|---------|---------|------|
| `http` | Circuit breaker + retry | [http.md](docs/config/http.md) |
| `redis` | Redis client | [redis.md](docs/config/redis.md) |
| `health` | Health checks | [health.md](docs/config/health.md) |
| `ratelimiter` | Rate limiting | [ratelimiter.md](docs/config/ratelimiter.md) |
| `apiversion` | API versioning | [apiversion.md](docs/config/apiversion.md) |

### Async & Workers
| Package | Purpose | Docs |
|---------|---------|------|
| `queue` | In-memory job queue | [queue.md](docs/config/queue.md) |
| `taskqueue` | Redis-based queue | [taskqueue.md](docs/config/taskqueue.md) |
| `workerpool` | Goroutine pool | [workerpool.md](docs/config/workerpool.md) |
| `cron` | Cron scheduler | [cron.md](docs/config/cron.md) |

### Utilities
| Package | Purpose | Docs |
|---------|---------|------|
| `paginator` | HTTP pagination | [paginator.md](docs/config/paginator.md) |
| `response` | HTTP responses | [response.md](docs/config/response.md) |
| `retry` | Retry logic | [retry.md](docs/config/retry.md) |
| `bulkops` | Bulk DB operations | [bulkops.md](docs/config/bulkops.md) |
| `testing` | Test helpers | [testing.md](docs/config/testing.md) |
| `idgen` | Snowflake IDs | [idgen.md](docs/config/idgen.md) |

### More
| Package | Purpose | Docs |
|---------|---------|------|
| `auth` | JWT + RBAC | [auth.md](docs/config/auth.md) |
| `middleware` | HTTP middleware | [middleware.md](docs/config/middleware.md) |
| `telemetry` | Metrics + tracing | [telemetry.md](docs/config/telemetry.md) |
| `lock` | Distributed locks | [lock.md](docs/config/lock.md) |
| `shutdown` | Graceful shutdown | [shutdown.md](docs/config/shutdown.md) |
| `eventbus` | Pub/sub events | [eventbus.md](docs/config/eventbus.md) |

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

### In-Memory Cache (LRU)

For fast local caching without Redis:

```go
localCache := memorycache.New(
    memorycache.WithMaxSize(1000),
    memorycache.WithTTL(5*time.Minute),
)

val, _ := localCache.GetOrSet(ctx, "config:123", func() (any, error) {
    return db.GetConfig(ctx, "123")
}, time.Minute)
```

### Pagination

Standardized list pagination:

```go
params, _ := paginator.FromRequest(r, 20)
users, _ := db.ListUsers(ctx, params.Offset(), params.Limit())
total, _ := db.CountUsers(ctx)

response.SuccessWithMeta(w, users, total, params.Page(), params.Limit())
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

### Bulk Operations

Batch database operations efficiently:

```go
err := bulkops.Insert(ctx, users, 100, func(batch []User) error {
    return db.InsertUsers(ctx, batch)
})
```

### Queue (In-Memory)

Lightweight async tasks:

```go
q := queue.New(queue.WithWorkers(4))
q.Enqueue(ctx, queue.Job{Type: "email", Payload: data})

worker := q.Worker("email")
worker.Start(ctx, func(ctx context.Context, job queue.Job) error {
    return sendEmail(ctx, job.Payload)
})
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
  user: app
  name: myapp
  ssl_mode: disable

redis:
  host: localhost
  port: 6379

log:
  level: info
  json_format: false
```

Override with environment variables: `DB_HOST`, `DB_PORT`, `DB_NAME`, `REDIS_HOST`, etc.

## Project Structure

```
mesh/
├── cache/            # Redis caching
├── memorycache/      # In-memory LRU cache
├── config/           # Configuration + feature flags
├── database/         # PostgreSQL utilities
├── errors/          # Structured errors
├── logger/          # Structured logging
├── json/            # Fast JSON
├── http/            # Circuit breaker + retry
├── redis/           # Redis client
├── health/          # Health checks
├── ratelimiter/     # Rate limiting
├── apiversion/      # API versioning
├── queue/           # In-memory queue
├── taskqueue/       # Redis queue
├── workerpool/      # Goroutine pool
├── cron/            # Cron scheduler
├── middleware/      # HTTP middleware
├── auth/            # JWT + RBAC
├── telemetry/       # Metrics + tracing
├── lock/            # Distributed locks
├── shutdown/        # Graceful shutdown
├── eventbus/        # Pub/sub events
├── paginator/       # Pagination
├── response/        # HTTP responses
├── retry/           # Retry logic
├── bulkops/         # Bulk DB operations
├── testing/         # Test helpers
├── idgen/           # Snowflake IDs
└── ...
```

## Testing

```bash
go test ./...
```

## License

MIT
