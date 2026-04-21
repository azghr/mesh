# Service Discovery

Redis-backed service discovery for microservices.

## Overview

This package provides service registry and discovery using Redis.

## Installation

```go
import "github.com/azghr/mesh/discovery"
```

## Usage

```go
sd := discovery.New(discovery.Config{
    Redis: client,
})

// Register
sd.Register(ctx, "user-service", "localhost:8080", map[string]string{
    "version": "1.0.0",
})

// Discover
instances, _ := sd.Discover(ctx, "user-service")
```

## API

- `New(cfg Config) *Registry` - Creates registry
- `r.Register(ctx, name, addr, metadata)` - Registers service
- `r.Discover(ctx, name)` - Discovers services
- `r.Deregister(ctx, name, addr)` - Removes service
- `r.ListAll(ctx)` - Lists all services