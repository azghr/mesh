# mesh

A production-ready Go package for building microservices with clean architecture and best practices.

## Overview

`mesh` provides a comprehensive set of reusable components for building scalable, maintainable Go microservices. It includes utilities for logging, error handling, database operations, HTTP/gRPC middleware, caching, distributed tracing, and more.

## Features

- **Logger**: Structured logging with slog, context support, and customizable output
- **Errors**: Application error handling with HTTP/gRPC status mapping
- **Database**: PostgreSQL connection pooling and health checks
- **HTTP**: Server utilities, middleware, and graceful shutdown
- **Config**: Environment-based configuration loading
- **Auth**: RBAC (Role-Based Access Control) utilities
- **Cache**: Redis-based caching with distributed locking
- **Health**: Health check endpoints for Kubernetes readiness/liveness probes
- **Telemetry**: OpenTelemetry integration for distributed tracing
- **Middleware**: HTTP and gRPC middleware for common concerns

## Installation

```bash
go get github.com/azghr/mesh
```

## Usage

### Logger

```go
import "github.com/azghr/mesh/logger"

log := logger.New("my-service", "info", false)
log.Info("Service starting up")
log.WithField("user_id", "123").Info("User logged in")
```

### Errors

```go
import "github.com/azghr/mesh/errors"

err := errors.NotFoundError("user", "123")
httpStatus := err.ToHTTPStatus()
grpcStatus := err.ToGRPCStatus()
```

### Database

```go
import "github.com/azghr/mesh/database"

db, err := database.NewConnection(database.Config{
    Host:     "localhost",
    Port:     5432,
    User:     "user",
    Password: "pass",
    Database: "mydb",
})
```

### Middleware

```go
import "github.com/azghr/mesh/middleware"

// Apply common middleware
app.Use(middleware.Recovery())
app.Use(middleware.RequestID())
app.Use(middleware.Logger(log))
```

## License

MIT License - see LICENSE file for details.
