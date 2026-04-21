# Service Discovery

Redis-backed service discovery for microservices.

## Overview

This package provides service registry and discovery using Redis. It enables microservices to register themselves and be discovered by other services dynamically.

## Features

- Service registration with metadata
- Service discovery by name
- Health check support
- Automatic TTL-based cleanup
- Background heartbeats

## Installation

```go
import "github.com/azghr/mesh/discovery"
```

## Usage

### Register a Service

```go
sd := discovery.New(discovery.Config{
    Redis: redisClient,
    TTL: 30 * time.Second,
})

err := sd.Register(ctx, "user-service", "localhost:8080", map[string]string{
    "version": "1.0.0",
    "env":    "production",
})
if err != nil {
    log.Fatal(err)
}
```

### Discover Services

```go
services, err := sd.Discover(ctx, "user-service")
if err != nil {
    log.Fatal(err)
}

for _, svc := range services {
    fmt.Printf("Service: %s at %s\n", svc.Name, svc.Address)
}
```

### Discover One Service

For load balancing across instances:

```go
svc, err := sd.DiscoverOne(ctx, "user-service")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Using: %s\n", svc.Address)
```

### Deregister

```go
err := sd.Deregister(ctx, "user-service", "localhost:8080")
```

### List All Services

```go
all, err := sd.ListAll(ctx)
for name, svcs := range all {
    fmt.Printf("%s: %d instances\n", name, len(svcs))
}
```

### Health Checks

```go
// Register health check
sd.RegisterHealthCheck("user-service", func(ctx context.Context, addr string) error {
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        return err
    }
    conn.Close()
    return nil
})

// Check all services
results := sd.CheckHealth(ctx)
for addr, err := range results {
    fmt.Printf("Unhealthy: %s - %v\n", addr, err)
}
```

### Heartbeats

Keep service registration alive:

```go
// Start background heartbeat
sd.StartHeartbeat(ctx, "user-service", "localhost:8080")

// Or manually update
sd.Heartbeat(ctx, "user-service", "localhost:8080")
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `Redis` | Redis client | Required |
| `TTL` | Service expiry time | 30s |
| `Heartbeat` | Heartbeat interval | 10s |

## API Reference

### `New(cfg Config) *Registry`

Creates a new service registry.

### `r.Register(ctx context.Context, name, addr string, metadata map[string]string) error`

Registers a service with metadata.

### `r.Discover(ctx context.Context, name string) ([]Service, error)`

Discovers all services by name.

### `r.DiscoverOne(ctx context.Context, name string) (*Service, error)`

Discovers a single service (random selection).

### `r.Deregister(ctx context.Context, name, addr string) error`

Removes a service registration.

### `r.ListAll(ctx context.Context) (map[string][]Service, error)`

Lists all registered services.

### `r.ServiceInfo(ctx context.Context, name, addr string) (*Service, error)`

Returns service information.

### `r.CheckHealth(ctx context.Context) (map[string]error)`

Checks health of all registered services.

### `r.Heartbeat(ctx context.Context, name, addr string) error`

Updates service heartbeat.

### `r.StartHeartbeat(ctx context.Context, name, addr string)`

Starts background heartbeat.

### `sd.RegisterHealthCheck(name string, fn HealthCheckFunc)`

Registers a health check function.

## Example: Service Clients

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/discovery"
    "github.com/redis/go-redis/v9"
)

func main() {
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    sd := discovery.New(discovery.Config{
        Redis: client,
    })

    ctx := context.Background()

    // Find user service
    svc, err := sd.DiscoverOne(ctx, "user-service")
    if err != nil {
        log.Printf("User service not available: %v", err)
        return
    }

    log.Printf("Connecting to user service at %s", svc.Address)
}
```