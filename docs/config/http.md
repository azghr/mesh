# http

Resilient HTTP client with circuit breaker and retry patterns.

## What It Does

Combines circuit breaker and retry logic into a single HTTP client. Prevents cascading failures and handles transient errors gracefully.

## Circuit Breaker

Stops calling a failing service automatically. Has three states:

- **Closed**: Normal operation, calls go through
- **Open**: Too many failures, calls are rejected immediately
- **Half-Open**: Testing if service recovered, limited calls allowed

### Usage

```go
cb := http.NewCircuitBreaker(nil) // uses defaults

err := cb.Execute(func() error {
    return callExternalService()
})

if err != nil {
    // Service is unavailable (circuit is OPEN)
    // Don't keep hammering it
}
```

### Configuration

```go
config := &http.CircuitBreakerConfig{
    MaxFailures:      5,        // Failures before opening (default: 5)
    ResetTimeout:     60*time.Second,  // Time before half-open (default: 60s)
    HalfOpenMaxCalls: 3,        // Max calls in half-open (default: 3)
    SuccessThreshold: 2,        // Successes to close (default: 2)
}
cb := http.NewCircuitBreaker(config)
```

### State Handling

```go
// Check state
cb.State()       // returns StateClosed, StateOpen, or StateHalfOpen
cb.IsOpen()      // true if open
cb.IsClosed()    // true if closed
cb.IsHalfOpen()  // true if half-open

// Get stats
stats := cb.Stats()
// map with state, failure_count, success_count, next_attempt, etc.

// Reset manually (for admin tools)
cb.Reset()
```

## Retry Logic

Exponential backoff with jitter for handling transient failures.

```go
// Wrap any function with retry
err := http.Retry(func() error {
    return callService()
})

// With custom config
err := http.RetryWithConfig(func() error {
    return callService()
}, &http.RetryConfig{
    MaxRetries:    5,
    InitialDelay:  100*time.Millisecond,
    MaxDelay:      30*time.Second,
    BackoffFactor: 2.0,
    Jitter:        true,
})
```

### Retry Configuration

```go
type RetryConfig struct {
    MaxRetries    int           // Max retry attempts (default: 3)
    InitialDelay  time.Duration // First delay (default: 100ms)
    MaxDelay      time.Duration // Max delay cap (default: 10s)
    BackoffFactor float64       // Multiply each delay by this (default: 2.0)
    Jitter        bool          // Add randomness (default: true)
    JitterRange   float64       // Jitter range 0.0-1.0 (default: 0.1)
}
```

### Marking Errors as Retryable

```go
// Wrap errors that should be retried
return http.NewRetryableError(err)

// Check if error is retryable
if http.IsRetryable(err) {
    // Will be retried
}
```

## Resilient Client

Combines circuit breaker + retry + HTTP client into one:

```go
config := http.DefaultResilientClientConfig("my-service")
config.HTTPTimeout = 30*time.Second

client := http.NewResilientClient(config)

// GET request
resp, err := client.Get("https://api.example.com/users")

// POST with JSON
resp, err := client.Post("https://api.example.com/users", user)

// GET and decode JSON directly
var users []User
err := client.GetJSON("https://api.example.com/users", &users)

// POST and decode response
var created User
err := client.PostJSON("https://api.example.com/users", createReq, &created)

// PUT, DELETE similarly...
resp, err := client.Put(url, body)
resp, err := client.Delete(url)
```

### Access Underlying Components

```go
// Get circuit breaker for monitoring
cb := client.CircuitBreaker()
if cb.IsOpen() {
    log.Println("Circuit is open, service unavailable")
}
```

## Fallback Support

Graceful degradation when services fail. Returns cached fallback responses instead of errors when circuit is open.

### Basic Usage

```go
// Create fallback client
client := http.NewFallbackClient(nil)

// Register fallback for a service
client.SetFallback("users", http.FallbackResponse{
    Content:   []map[string]string{{"id": "1", "name": "Default User"}},
    StatusCode: 200,
    TTL:       5*time.Minute,
})

// Execute with fallback
result, err := client.Execute(ctx, "users", func() (interface{}, error) {
    return fetchUsers(ctx)
})

// No error returned if fallback exists - even if circuit is open
// result contains fallback data
```

### Fallback Components

```go
// FallbackResponse - the fallback data
type FallbackResponse struct {
    Content   interface{}   // Fallback data (any JSON-serializable type)
    StatusCode int        // HTTP status code to return
    TTL       time.Duration // Cache duration for fallback
}

// FallbackRegistry - stores multiple service fallbacks
registry := http.NewFallbackRegistry()
registry.Register("users", fallback)
fb, ok := registry.Get("users")

// FallbackClient - combines circuit breaker with fallbacks
client := http.NewFallbackClient(circuitBreaker)
client.SetFallback("service", fallbackResp)
```

### Caching Fallbacks

```go
// Fallbacks are cached locally to avoid repeated service calls
// TTL determines how long to use cached fallback
response := http.FallbackResponse{
    Content:   cachedData,
    StatusCode: 200,
    TTL:       5*time.Minute,  // Cache for 5 minutes
}

// Cache is automatic - same fallback returned during outage
```

### Best Practices

- Return partial data (empty arrays, cached lists) instead of errors
- Set TTL based on data freshness requirements
- Monitor fallback usage - high rate indicates problems
- Use for read-only operations primarily

## Network Error Helpers

```go
// Check if error is a retryable network error
if http.IsNetworkError(err) {
    // Temporary network issue
}

// Check if error is a timeout
if http.IsTimeoutError(err) {
    // Operation timed out
}
```