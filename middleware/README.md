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

Request rate limiting per IP or per user.

```go
// Basic (100 requests per minute per IP)
app.Use(middleware.RateLimit())

// Custom
app.Use(middleware.RateLimit(
    middleware.WithLimit(200),
    middleware.WithWindow(time.Minute),
    middleware.WithKeyFunc(func(c *fiber.Ctx) string {
        return c.IP()  // or c.Locals("user_id") for per-user
    }),
))
```

Response when rate limited:
```json
{"error": "rate limit exceeded", "retry_after": 30}
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

## CORS Middleware

Cross-Origin Resource Sharing (CORS) middleware for handling cross-origin requests.

```go
// Basic - allow all origins (default)
app.Use(middleware.CORS())

// Restricted origins
app.Use(middleware.CORS(
    middleware.WithAllowedOrigins("https://example.com"),
))

// With credentials (requires specific origin, no wildcard)
app.Use(middleware.CORS(
    middleware.WithAllowedOrigins("https://app.example.com"),
    middleware.WithAllowCredentials(true),
    middleware.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
    middleware.WithAllowedHeaders("Content-Type", "Authorization"),
    middleware.WithExposedHeaders("X-Request-ID"),
    middleware.WithMaxAge(1*time.Hour),
))

// Custom origin validation
app.Use(middleware.CORS(
    middleware.WithAllowOriginFunc(func(origin string) bool {
        return strings.Contains(origin, "example.com")
    }),
))
```

Response headers added:
- `Access-Control-Allow-Origin` - Origin allowed to access the resource
- `Access-Control-Allow-Methods` - HTTP methods allowed
- `Access-Control-Allow-Headers` - Request headers allowed
- `Access-Control-Expose-Headers` - Response headers accessible to client
- `Access-Control-Allow-Credentials` - Whether credentials are allowed
- `Access-Control-Max-Age` - Preflight cache duration

## All Middleware Available

| Middleware | Purpose |
|------------|---------|
| `Logging` | Request/response logging |
| `Recovery` | Panic recovery |
| `RateLimit` | Request rate limiting |
| `Correlation` | Request ID tracking |
| `SecurityHeaders` | Add security headers |
| `Validation` | Request body validation |
| `CORS` | Cross-Origin Resource Sharing |