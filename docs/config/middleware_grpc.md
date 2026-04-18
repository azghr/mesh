# gRPC Middleware

gRPC interceptors for logging, recovery, rate limiting, and circuit breaking.

## What It Does

Provides gRPC server and client interceptors that add resilience and observability to gRPC services:

- **Logging**: Request/response logging with duration and status
- **Recovery**: Panic recovery to prevent server crashes
- **Rate Limiting**: Per-method or per-user rate limiting via Redis
- **Circuit Breaker**: Prevent cascading failures

## Quick Start

```go
import (
    "google.golang.org/grpc"
    "github.com/azghr/mesh/middleware"
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/redis"
)

// Create a rate limiter
redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
limiter := ratelimiter.NewRedisRateLimiter(redisClient.Client(), 100, time.Minute)

// Create server with all interceptors
server := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        middleware.GRPCLogging(),
        middleware.GRPCRecovery(),
        middleware.GRPCCircuitBreaker(),
        middleware.GRPCRateLimit(limiter),
    ),
)
```

## Server Interceptors

### Logging

Logs gRPC requests with method, duration, status, and error codes.

```go
import "github.com/azghr/mesh/middleware"

server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCLogging(
        middleware.WithGRPCLogRequests(true),
        middleware.WithGRPCLogDuration(true),
    )),
)
```

**Output:**
```
MESH |INFO| gRPC request started request_id=123456 method=/user.UserService/GetUser user_id=user1
MESH |INFO| gRPC request completed request_id=123456 method=/user.UserService/GetUser duration_ms=45
MESH |ERROR| gRPC request failed request_id=789 method=/payment.Process code=INTERNAL
```

**Options:**
| Option | Description | Default |
|--------|-------------|---------|
| `WithGRPCLogRequests` | Log request start | `false` |
| `WithGRPCLogResponses` | Log response | `false` |
| `WithGRPCLogDuration` | Log duration | `true` |
| `WithGRPCInterestingMethods` | Log only specific methods | all |
| `WithGRPCLogger` | Custom logger | global |

### Recovery

Catches panics in handlers and returns a proper gRPC error instead of crashing.

```go
server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCRecovery()),
)
```

When a panic occurs:
- Logs the panic details
- Returns `codes.Internal` error to client
- Server continues running

**Options:**
| Option | Description |
|--------|-------------|
| `WithGRPCRecoveryLogger` | Custom logger |
| `WithGRPCRecoveryHandler` | Custom panic handler function |

### Rate Limiting

Distributed rate limiting using Redis. Supports per-method and per-user limits.

```go
limiter := ratelimiter.NewRedisRateLimiter(redisClient, 100, time.Minute)

server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCRateLimit(limiter,
        middleware.WithGRPCRateLimitKeyPrefix("myapp"),
        middleware.WithGRPCRateLimitByMethod(),
    )),
)
```

**Options:**
| Option | Description |
|--------|-------------|
| `WithGRPCRateLimitKeyPrefix` | Redis key prefix (default: "grpc") |
| `WithGRPCRateLimitByMethod` | Separate limit per method |
| `WithGRPCRateLimitByUserID` | Separate limit per user (requires user ID in metadata) |

**Error:** Returns `codes.ResourceExhausted` when limit exceeded.

### Circuit Breaker

Prevents cascading failures by tracking errors per method.

```go
server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCCircuitBreaker(
        middleware.WithGRPCCircuitBreakerMaxFailures(5),
        middleware.WithGRPCCircuitBreakerResetTimeout(30*time.Second),
    )),
)
```

**States:**
1. **Closed**: Normal operation, requests allowed
2. **Open**: Too many failures, requests blocked
3. **HalfOpen**: Testing if service recovered

**Options:**
| Option | Description | Default |
|--------|-------------|---------|
| `WithGRPCCircuitBreakerMaxFailures` | Failures before opening | 5 |
| `WithGRPCCircuitBreakerResetTimeout` | Time before half-open | 30s |
| `WithGRPCCircuitBreakerByMethod` | Separate circuit per method | true |

**Error:** Returns `codes.Unavailable` when circuit is open.

## Stream Interceptors

Streaming RPC support with the same patterns.

```go
server := grpc.NewServer(
    grpc.StreamInterceptor(middleware.GRPCLoggingStream(
        middleware.WithGRPCLogRequests(true),
    )),
    grpc.StreamInterceptor(middleware.GRPCRecoveryStream()),
    grpc.StreamInterceptor(middleware.GRPCRateLimitStream(limiter)),
)
```

## Client Interceptors

### Logging

```go
conn, err := grpc.Dial(
    "localhost:50051",
    grpc.WithUnaryInterceptor(middleware.GRPCClientLogging(
        middleware.WithGRPCLogRequests(true),
        middleware.WithGRPCLogDuration(true),
    )),
)
```

### Rate Limiting

```go
conn, err := grpc.Dial(
    "localhost:50051",
    grpc.WithUnaryInterceptor(middleware.GRPCClientRateLimit(limiter)),
)
```

### Stream Interceptors

```go
conn, err := grpc.Dial(
    "localhost:50051",
    grpc.WithStreamInterceptor(middleware.GRPCClientLoggingStream()),
    grpc.WithStreamInterceptor(middleware.GRPCClientRateLimitStream(limiter)),
)
```

## Usage Examples

### Complete Server Setup

```go
import (
    "time"
    "google.golang.org/grpc"
    "github.com/azghr/mesh/middleware"
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/redis"
)

func newServer() *grpc.Server {
    // Redis client
    redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
    
    // Rate limiter: 1000 requests/minute per method
    limiter := ratelimiter.NewRedisRateLimiter(
        redisClient.Client(),
        1000,
        time.Minute,
    )
    
    return grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            middleware.GRPCLogging(
                middleware.WithGRPCLogDuration(true),
            ),
            middleware.GRPCRecovery(),
            middleware.GRPCCircuitBreaker(
                middleware.WithGRPCCircuitBreakerMaxFailures(5),
                middleware.WithGRPCCircuitBreakerResetTimeout(30*time.Second),
            ),
            middleware.GRPCRateLimit(limiter,
                middleware.WithGRPCRateLimitByMethod(),
            ),
        ),
    )
}
```

### Complete Client Setup

```go
func newClient() (*grpc.ClientConn, error) {
    redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
    limiter := ratelimiter.NewRedisRateLimiter(redisClient.Client(), 500, time.Minute)
    
    return grpc.Dial(
        "localhost:50051",
        grpc.WithUnaryInterceptor(middleware.GRPCClientLogging()),
        grpc.WithUnaryInterceptor(middleware.GRPCClientRateLimit(limiter)),
        grpc.WithStreamInterceptor(middleware.GRPCClientLoggingStream()),
    )
}
```

### Per-User Rate Limiting

```go
// Server: rate limit per user
server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.GRPCRateLimit(limiter,
        middleware.WithGRPCRateLimitKeyPrefix("api"),
        middleware.WithGRPCRateLimitByUserID(
            func(ctx context.Context) string {
                return middleware.GRPCGetUserID(ctx)
            },
        ),
    )),
)

// Client: include user ID in metadata
func callWithUserID(ctx context.Context, conn *grpc.ClientConn, userID string) error {
    md := metadata.Pairs("x-user-id", userID)
    ctx = metadata.NewOutgoingContext(ctx, md)
    // ... make gRPC call
    return nil
}
```

### Chain Helpers

Combine multiple interceptors efficiently:

```go
interceptors := middleware.GRPCChainUnaryInterceptor(
    middleware.GRPCLogging(),
    middleware.GRPCRecovery(),
    nil,  // skip rate limiting
    middleware.GRPCCircuitBreaker(),
)

server := grpc.NewServer(
    grpc.ChainUnaryInterceptor(interceptors...),
)
```

Available chain helpers:
- `GRPCChainUnaryInterceptor`
- `GRPCChainStreamInterceptor`
- `GRPCChainUnaryClientInterceptor`
- `GRPCChainStreamClientInterceptor`

## Context Helpers

### Get Metadata from Context

```go
// In server handler
func (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    requestID := middleware.GRPCGetRequestID(ctx)
    userID := middleware.GRPCGetUserID(ctx)
    traceID := middleware.GRPCGetTraceID(ctx)
    
    log.Info("processing request",
        "request_id", requestID,
        "user_id", userID,
        "trace_id", traceID,
    )
    
    // ... handler logic
}
```

### Add Metadata to Context

```go
// In client before making call
func withMetadata(ctx context.Context, requestID, userID string) context.Context {
    ctx = middleware.GRPCWithRequestID(ctx, requestID)
    ctx = middleware.GRPCWithUserID(ctx, userID)
    return ctx
}
```

## Error Codes

Middleware returns standard gRPC error codes:

| Code | Used By | Description |
|------|--------|-------------|
| `Internal` | Recovery | Panic recovered |
| `ResourceExhausted` | Rate Limit | Limit exceeded |
| `Unavailable` | Circuit Breaker | Circuit open |

## Metrics

The interceptors integrate with the telemetry package for metrics:

```go
// Server metrics (automatically recorded)
// grpc_server_request_duration_seconds{method="/user.Get"}
// grpc_server_errors_total{method="/user.Get",code="INTERNAL"}

// Client metrics
// grpc_client_request_duration_seconds{method="/user.Get"}
// grpc_client_errors_total{method="/user.Get",code="UNAVAILABLE"}
```

## All Options

### Logging Options
- `WithGRPCLogger(logger.Logger)`
- `WithGRPCLogRequests(bool)`
- `WithGRPCLogResponses(bool)`
- `WithGRPCLogDuration(bool)`
- `WithGRPCInterestingMethods(...string)`

### Recovery Options
- `WithGRPCRecoveryLogger(logger.Logger)`
- `WithGRPCRecoveryHandler(func(p any) (any, error))`

### Rate Limit Options
- `WithGRPCRateLimiter(ratelimiter.RateLimiter)`
- `WithGRPCRateLimitKeyPrefix(string)`
- `WithGRPCRateLimitByMethod()`
- `WithGRPCRateLimitByUserID(func(ctx) string)`

### Circuit Breaker Options
- `WithGRPCCircuitBreakerMaxFailures(int)`
- `WithGRPCCircuitBreakerResetTimeout(time.Duration)`
- `WithGRPCCircuitBreakerByMethod()`