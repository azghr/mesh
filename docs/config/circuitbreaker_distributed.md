# Distributed Circuit Breaker

Circuit breaker with Redis-backed state for multi-instance deployments.

## What It Does

Shares circuit breaker state across multiple service instances using Redis. Ensures consistent failure detection in horizontally scaled deployments.

## Why Use It

In single-instance deployments, circuit breaker tracks local failures only. In multi-pod deployments, each pod has its own state, leading to:
- Inconsistent circuit states across instances
- Continued traffic to failing instances
- Poor failure detection

## Installation

Requires Redis client (already included in mesh dependencies).

## Quick Start

```go
import "github.com/azghr/mesh/http"

// Create distributed circuit breaker
config := http.DefaultDistributedConfig(redisClient)

dcb := http.NewDistributedCircuitState(nil, config)

// Use with fallback client
client := http.NewDistributedClient(nil, config)
client.SetFallback("users", http.FallbackResponse{
    Content:   cachedUsers,
    StatusCode: 200,
    TTL:       5*time.Minute,
})

result, err := client.Execute(ctx, "users", fetchUsers)
```

## How It Works

1. **Local Tracking**: Each instance tracks its own failures
2. **Redis Sharing**: State is stored in Redis
3. **Cross-Instance Rate**: Calculates global failure rate
4. **Consistent Decision**: All instances agree on circuit state

### State Flow

```
Request → Check Local State → Check Redis State → Calculate Rate → Decision
         (fast path)        (if local unclear)  (if needed)
```

## Configuration

```go
type DistributedConfig struct {
    Redis       *redis.Client  // Required: Redis client
    KeyPrefix   string         // Default: "circuit_breaker"
    Threshold   float64        // Default: 0.5 (50% failure rate)
    StateTTL    time.Duration // Default: 5 minutes
}
```

### Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| Redis | (required) | Redis client for state storage |
| KeyPrefix | "circuit_breaker" | Prefix for Redis keys |
| Threshold | 0.5 | Failure rate to open circuit (0.0-1.0) |
| StateTTL | 5m | How long to keep state in Redis |

## Usage with Fallback

Combine with fallback for graceful degradation:

```go
client := http.NewDistributedClient(nil, http.DistributedConfig{
    Redis:     redisClient,
    Threshold: 0.5,
})

// Set fallback for when circuit is open
client.SetFallback("users", http.FallbackResponse{
    Content:   []map[string]string{{"id": "0", "name": "Unavailable"}},
    StatusCode: 200,
    TTL:       5*time.Minute,
})

// Execute with automatic fallback
result, err := client.Execute(ctx, "users", func() (interface{}, error) {
    return fetchUsersFromAPI()
})

// If API fails and circuit opens, returns fallback automatically
```

## Monitoring

Get distributed state for monitoring:

```go
state, err := dcb.GetDistributedState(ctx, "users-service")
// Returns: {"successes": "100", "failures": "5", "last_failure": "1234567890"}
```

## Best Practices

- Use consistent threshold across all instances
- Set StateTTL based on expected recovery time
- Monitor failure rate in metrics
- Use with fallback for graceful degradation
- Set up alerts for circuit open events

## When to Use

- Horizontal scaling with multiple instances
- Load balancer doesn't track service health
- Need consistent failure detection
- Multi-region deployments

## Example: Multiple Services

```go
client := http.NewDistributedClient(nil, http.DistributedConfig{
    Redis:     redisClient,
    KeyPrefix: "myapp",
})

// Configure fallbacks for each service
client.SetFallback("users", http.FallbackResponse{Content: cachedUsers, StatusCode: 200})
client.SetFallback("orders", http.FallbackResponse{Content: cachedOrders, StatusCode: 200})
client.SetFallback("payments", http.FallbackResponse{Content: cachedPayments, StatusCode: 200})

// Use for each service
users, _ := client.Execute(ctx, "users", fetchUsers)
orders, _ := client.Execute(ctx, "orders", fetchOrders)
payments, _ := client.Execute(ctx, "payments", fetchPayments)
```