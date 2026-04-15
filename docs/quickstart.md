# Quick Start

This guide walks through setting up mesh in a new project.

## 1. Install

```bash
go get github.com/azghr/mesh
```

## 2. Create Config File

Create `config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  environment: development

database:
  host: localhost
  port: 5432
  port_int: 5432
  user: myapp
  name: myapp_db
  ssl_mode: disable

redis:
  host: localhost
  port: 6379

log:
  level: info
  json_format: false
```

## 3. Basic Service Setup

```go
package main

import (
    "log"

    "github.com/azghr/mesh/config"
    "github.com/azghr/mesh/database"
    "github.com/azghr/mesh/logger"
)

func main() {
    // Load config (YAML + env overrides)
    cfg, err := config.Load("config.yaml", config.WithDefaultConfig())
    if err != nil {
        log.Fatal("config error: ", err)
    }

    // Validate for production
    if err := config.ValidateProduction(cfg); err != nil {
        log.Fatal("validation: ", err)
    }

    // Setup logging
    log := logger.New("myservice", cfg.Log.Level, cfg.Log.JSONFormat)
    log.Info("starting service", "port", cfg.Server.Port)

    // Setup database
    db, err := database.NewPool(database.Config{
        Host: cfg.Database.Host,
        Port: cfg.Database.PortInt,
        User: cfg.Database.User,
        Password: cfg.Database.Password,
        Name: cfg.Database.Name,
        MaxOpenConns: cfg.Database.MaxOpenConns,
    })
    if err != nil {
        log.Fatal("database: ", err)
    }
    defer db.Close()

    // ... continue with HTTP server, etc.
}
```

## 4. Adding Cache

```go
import (
    "github.com/azghr/mesh/cache"
    "github.com/azghr/mesh/redis"
)

// After database setup
rclient, err := redis.NewClient(redis.Config{
    Host: cfg.Redis.Host,
    Port: cfg.Redis.Port,
})
if err != nil {
    log.Fatal("redis: ", err)
}
defer rclient.Close()

c, err := cache.New(rclient.Client(), 5*time.Minute)
if err != nil {
    log.Fatal("cache: ", err)
}

// Use cache-aside pattern
var user User
err = c.GetOrSet(ctx, "user:123", &user, time.Hour, func() (any, error) {
    return findUser(ctx, db.DB(), "123")
})
```

## 5. Resilient HTTP Client

```go
import "github.com/azghr/mesh/http"

// For calling external APIs
client := http.NewResilientClient(http.DefaultResilientClientConfig("external-api"))

// Make requests
resp, err := client.Get("https://api.example.com/data")
```

## 6. Health Checks

```go
import "github.com/azghr/mesh/health"

checker := health.NewChecker()
checker.Register("database", func(ctx context.Context) error {
    return db.Ping(ctx)
})
checker.Register("redis", func(ctx context.Context) error {
    return rclient.Ping(ctx)
})

// Expose /health endpoint
results := checker.Check(ctx)
```

## Environment Variables

Override YAML config:

```bash
export DB_HOST=prod-db.example.com
export DB_PASSWORD=secret
export REDIS_HOST=redis.example.com
```

## Next Steps

- [Config Package](config/config.md) - Full config docs
- [Database Package](config/database.md) - PostgreSQL setup
- [Cache Package](config/cache.md) - Redis caching patterns