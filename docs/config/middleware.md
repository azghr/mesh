# middleware

HTTP middleware for common server needs: logging, recovery, rate limiting, etc.

## What It Does

A collection of HTTP middleware functions that wrap handlers to add functionality.

## Logging Middleware

Logs HTTP requests with method, path, status, and duration.

```go
// Basic
app.Use(middleware.Logging())

// With options
app.Use(middleware.Logging(
    middleware.WithLogger(log),
    middleware.WithBodyCapture(true),
    middleware.WithMaxBodySize(1024*1024),  // 1MB
    middleware.WithExcludePaths("/health", "/metrics"),
))
```

Output:
```
[15:04:05] HTTP request started request_id=123 method=GET path=/users remote_addr=127.0.0.1
[15:04:05] HTTP request completed request_id=123 method=GET path=/users status=200 duration_ms=45
```

## Recovery Middleware

Panic recovery - catches panics and returns a 500 instead of crashing.

```go
app.Use(middleware.Recovery())
```

## Rate Limiting

Request rate limiting with distributed support via Redis (Fiber integration).

### Basic Usage

```go
import (
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/middleware"
)

// Redis-backed distributed rate limiter
redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
limiter := ratelimiter.NewRedisRateLimiter(redisClient.Client(), 100, time.Minute)

mw := middleware.NewLimit(limiter)
app.Use(mw.Handler())
```

### Custom Key Functions

```go
// Per-user rate limiting
mw := middleware.NewLimit(limiter)
mw.KeyFunc(middleware.UserKeyFunc)

// Per-endpoint rate limiting
mw.KeyFunc(middleware.EndpointKeyFunc)

// Custom logic
mw.KeyFunc(func(c *fiber.Ctx) string {
    return "tenant:" + c.Locals("tenant_id").(string)
})
```

### Metrics

```go
mw := middleware.NewLimit(limiter)
app.Use(mw.Handler())

// Access metrics
metrics := mw.Metrics()
fmt.Printf("Allowed: %d, Rejected: %d\n", 
    metrics.AllowedTotal(), metrics.RejectedTotal())
```

### Custom Key Functions

```go
// Per-user rate limiting
mw := middleware.NewLimit(limiter)
mw.KeyFunc(middleware.UserKeyFunc)

// Per-endpoint rate limiting
mw.KeyFunc(middleware.EndpointKeyFunc)

// Custom logic
mw.KeyFunc(func(c *fiber.Ctx) string {
    return "tenant:" + c.Locals("tenant_id").(string)
})
```

### In-Memory (Single Instance)

```go
import "github.com/azghr/mesh/middleware"

// 100 requests per minute per IP
limiter := ratelimiter.NewSimpleRateLimiter(100, time.Minute)
mw := middleware.NewLimit(limiter)
app.Use(mw.Handler())
```

### Distributed (Redis-Backed)

```go
import (
    "github.com/azghr/mesh/ratelimiter"
    "github.com/azghr/mesh/middleware"
    "github.com/azghr/mesh/redis"
)

// Redis-backed for multi-instance deployments
redisClient, _ := redis.NewClient(redis.Config{Host: "localhost", Port: 6379})
limiter := ratelimiter.NewRedisRateLimiter(redisClient.Client(), 100, time.Minute)
mw := middleware.NewLimit(limiter)
app.Use(mw.Handler())
```

### Custom Key Functions

```go
// Per-user rate limiting (requires JWT/auth middleware to set user_id)
mw := middleware.NewLimit(limiter)
mw.KeyFunc(middleware.UserKeyFunc)

// Per-endpoint rate limiting
mw.KeyFunc(middleware.EndpointKeyFunc)

// Custom logic
mw.KeyFunc(func(c *fiber.Ctx) string {
    return "tenant:" + c.Locals("tenant_id").(string)
})
```

### Metrics

```go
mw := middleware.NewLimit(limiter)
app.Use(mw.Handler())

// Access metrics
metrics := mw.Metrics()
fmt.Printf("Allowed: %d, Rejected: %d\n", 
    metrics.AllowedTotal(), metrics.RejectedTotal())
```

### Response Headers

All responses include rate limit information:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed |
| `X-RateLimit-Remaining` | Requests remaining in window |
| `X-RateLimit-Reset` | Unix timestamp when window resets |
| `Retry-After` | Seconds to wait (only when limited) |

### Rate Limited Response

```json
{
  "error": "rate limit exceeded",
  "code": "RATE_LIMITED"
}
```

## Correlation ID

Adds and forwards a request ID through the request chain.

```go
app.Use(middleware.Correlation())
// Adds X-Correlation-ID header to requests
```

## Security Headers

Adds common security headers.

```go
app.Use(middleware.SecurityHeaders())
// Adds: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, etc.
```

## Validation Middleware

Validate request body against a schema.

```go
app.Post("/users", middleware.Validate(userSchema), handler)
// Returns 400 if validation fails
```

### Field Validators

Custom field-level validators for form/JSON input validation.

```go
import "github.com/azghr/mesh/middleware"

// Create validator with rules
v := middleware.NewFieldValidator()
v.AddRule("email", middleware.EmailValidator("invalid email"))
v.AddRule("password", middleware.PasswordValidator("password too weak"))

// Validate in handler
func handleRegister(w http.ResponseWriter, r *http.Request) {
    var data map[string]interface{}
    json.NewDecoder(r.Body).Decode(&data)
    
    errs := v.Validate(data)
    if len(errs) > 0 {
        json.NewEncoder(w).Encode(errs)
        return
    }
    // Process registration...
}
```

### Built-in Validators

| Validator | Description | Example |
|-----------|-------------|---------|
| `EmailValidator` | Validates email format | `test@example.com` |
| `PasswordValidator` | Min 8 chars, upper, lower, digit | `Password1` |

### Custom Validators

Create custom validation rules with ValidatorFunc:

```go
rule := middleware.ValidationRule{
    Name:    "phone",
    Fn:     middleware.ValidatorFunc(func(value interface{}) error {
        phone, ok := value.(string)
        if !ok {
            return fmt.Errorf("must be string")
        }
        if !regexp.MustCompile(`^\+?[0-9]{10,15}$`).MatchString(phone) {
            return fmt.Errorf("invalid phone format")
        }
        return nil
    }),
    ErrorMsg: "invalid phone number",
}
v.AddRule("phone", rule)
```

### Request Validation Middleware

HTTP-level request validation (body size, content-type):

```go
validation := middleware.NewValidationMiddleware()
validation.SetMaxBodySize(2 << 20) // 2MB
app.Use(validation.ValidateRequest())
```

## All Middleware Available

| Middleware | Purpose |
|------------|---------|
| `Logging` | Request/response logging |
| `Recovery` | Panic recovery |
| `Limit` | Request rate limiting (Redis-backed) |
| `Correlation` | Request ID tracking |
| `SecurityHeaders` | Add security headers |
| `Validation` | Request body validation |