# mesh

<div align="center">

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-4AAC61?style=flat)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/azghr/mesh.svg)](https://pkg.go.dev/github.com/azghr/mesh)
[![Go Report](https://goreportcard.com/badge/github.com/azghr/mesh)](https://goreportcard.com/report/github.com/azghr/mesh)

**A production-grade Go toolkit for building services**

</div>

---

## Why mesh?

- ⚡ **Lightweight** - Import only what you need
- 🛡️ **Production-tested** - Built from real-world services  
- 🔧 **Composable** - Each package works independently
- 📚 **Well-documented** - Godoc + static docs + examples

## Quick Install

```bash
go get github.com/azghr/mesh
```

## Packages Overview

| Package | Purpose |
|---------|---------|
| `config` | YAML + env config loading |
| `database` | PostgreSQL connection pool |
| `cache` | Redis caching with metrics |
| `errors` | Structured errors → HTTP/gRPC |
| `logger` | Structured logging |
| `http` | Circuit breaker + retry |
| `redis` | Redis client wrapper |
| `health` | Kubernetes health checks |
| `middleware` | HTTP middleware |
| `auth` | JWT + RBAC |
| `telemetry` | Metrics + tracing |
| `lock` | Distributed locks |
| `workerpool` | Goroutine pool |
| `shutdown` | Graceful shutdown |
| `eventbus` | Pub/sub events |

## Code Example

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
    // Configuration - YAML + env overrides
    cfg, _ := config.Load("config.yaml", config.WithDefaultConfig())

    // Structured logging
    log := logger.New("my-service", "debug", false)
    log.Info("starting service", "port", cfg.Server.Port)

    // Database connection pool
    pool, _ := database.NewPool(database.Config{
        Host: cfg.Database.Host,
        Port: cfg.Database.PortInt,
        User: cfg.Database.User,
        Name: cfg.Database.Name,
    })
    defer pool.Close()

    // Redis + Cache
    rclient, _ := redis.NewClient(redis.Config{
        Host: cfg.Redis.Host,
        Port: cfg.Redis.Port,
    })
    defer rclient.Close()

    c, _ := cache.New(rclient.Client(), 5*time.Minute)

    // Resilient HTTP client with circuit breaker + retry
    client := http.NewResilientClient(
        http.DefaultResilientClientConfig("external-api"),
    )

    // Cache-aside pattern
    var user User
    c.GetOrSet(ctx, "user:123", &user, time.Hour, func() (any, error) {
        return findUser(ctx, pool.DB(), "123")
    })
}
```

## Documentation

| Type | Link |
|------|------|
| **Godoc** | [pkg.go.dev/mesh](https://pkg.go.dev/github.com/azghr/mesh) |
| **Static Docs** | Browse sidebar |
| **GitHub** | [github.com/azghr/mesh](https://github.com/azghr/mesh) |

## Contributing

Contributions welcome! Please read the [contributing guidelines](https://github.com/azghr/mesh/blob/main/CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](https://github.com/azghr/mesh/blob/main/LICENSE) for details.