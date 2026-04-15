# circuitbreaker

HTTP handler for monitoring circuit breakers in your application.

## What It Does

Provides an HTTP endpoint to view the status of all circuit breakers in your application. Useful for:
- Debugging why an external service is failing
- Monitoring dashboard integration
- Manual circuit reset

## Usage

### Register Circuit Breakers

```go
import (
    httpmesh "github.com/azghr/mesh/http"
    "github.com/azghr/mesh/circuitbreaker"
)

// Create and register circuit breakers
cb := httpmesh.NewCircuitBreaker(nil)
circuitbreaker.Register("external-api", cb)
circuitbreaker.Register("payment-service", paymentCB)
```

### HTTP Endpoint

```go
// Add to your router
router.Get("/debug/circuit-breakers", circuitbreaker.Handler())
// Or with named breakers
router.Get("/debug/circuit-breakers", circuitbreaker.HandlerWithNames())
```

### Response Format

```json
{
  "total": 2,
  "breakers": [
    {
      "id": "cb-1234567890",
      "state": "CLOSED",
      "failure_count": 0,
      "success_count": 10,
      "last_failure_time": "",
      "last_state_change": "2024-01-15T10:00:00Z",
      "next_attempt": ""
    },
    {
      "id": "cb-9876543210",
      "state": "OPEN",
      "failure_count": 5,
      "success_count": 0,
      "last_failure_time": "2024-01-15T10:05:00Z",
      "last_state_change": "2024-01-15T10:05:00Z",
      "next_attempt": "2024-01-15T10:06:00Z"
    }
  ]
}
```

### With Names

```json
{
  "total": 2,
  "breakers": {
    "external-api": {
      "id": "cb-1234567890",
      "state": "CLOSED",
      ...
    },
    "payment-service": {
      "state": "OPEN",
      ...
    }
  }
}
```

## State Meanings

- **CLOSED**: Normal operation, requests go through
- **OPEN**: Too many failures, requests blocked
- **HALF_OPEN**: Testing if service recovered

## Programmatic Access

```go
// Reset all circuits
circuitbreaker.ResetAll()

// Count registered breakers
count := circuitbreaker.Count()

// Unregister a specific breaker
circuitbreaker.Unregister("external-api")
```

## Integration with HTTP Package

The `http` package's circuit breaker can be used with this:

```go
client := http.NewResilientClient(config)
circuitbreaker.Register("external", client.CircuitBreaker())
```