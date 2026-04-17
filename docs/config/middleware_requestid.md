# Request ID Middleware

HTTP middleware for request ID generation and propagation.

## What It Does

- Generates unique request ID for each incoming request
- Reuses existing ID from request header when present
- Makes request ID available in handlers via Fiber context
- Returns request ID in response headers for client correlation
- Enables distributed request tracing across services

## Installation

No additional dependencies required - built into the middleware package.

## Quick Start

```go
// Use default middleware (X-Request-ID)
app.Use(middleware.RequestID())

// Define routes
app.Get("/users", usersHandler)
app.Post("/orders", ordersHandler)
```

When a request comes in, the middleware:
1. Checks for X-Request-ID header
2. Generates new UUID if none exists
3. Stores in Fiber context as `request_id`
4. Returns in response header

## Usage

### Default Configuration

```go
app.Use(middleware.RequestID())
```

Uses default settings:
- Header: X-Request-ID
- Generator: UUID (v4)

### Custom Header Name

```go
app.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
    HeaderName: "X-Correlation-ID",
}))
```

### Custom ID Generator

```go
app.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
    Generator: func() string {
        return snowflake.NextID()
    },
}))
```

### Using Request ID in Handlers

```go
app.Get("/users", func(c *fiber.Ctx) error {
    // Get request ID
    requestID := middleware.GetRequestID(c)
    
    // Use in logging
    log.Info("handling request", "request_id", requestID)
    
    // Access via locals
    id := c.Locals("request_id")
    
    return c.JSON(fiber.Map{
        "request_id": requestID,
    })
})
```

## Integration with Logger

Include request ID in structured logs:

```go
app.Use(middleware.RequestID())

app.Use(func(c *fiber.Ctx) error {
    requestID := middleware.GetRequestID(c)
    
    // Custom logger with request ID
    log := logger.WithFields("request_id", requestID)
    log.Info("processing request")
    
    return c.Next()
})
```

## Distributed Tracing

For OpenTelemetry integration:

```go
app.Use(middleware.RequestID())

app.Use(func(c *fiber.Ctx) error {
    requestID := middleware.GetRequestID(c)
    
    // Add to OpenTelemetry span
    span := otelTrace.SpanFromContext(c.Context())
    span.SetAttributes(attribute.String("request.id", requestID))
    
    return c.Next()
})
```

## Response Headers

The middleware automatically adds the request ID to response headers:

```
X-Request-ID: 550e8400-e29b-41d4-a716-446655440000
```

Clients can use this ID to correlate requests across services.

## Best Practices

- Use consistent header name across all services
- Include request ID in all log statements
- Return request ID in API responses for client debugging
- Use for correlating distributed system issues
- Consider using for audit logging

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| HeaderName | string | "X-Request-ID" | Header to use |
| Generator | func() string | uuid.New() | ID generation function |
| ErrorHandler | fiber.Handler | nil | Custom error handler |

## Error Handling

Default behavior returns 500 on errors. Custom error handler:

```go
app.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
    ErrorHandler: func(c *fiber.Ctx) error {
        log.Error("request ID generation failed")
        return c.Status(500).SendString("Internal Server Error")
    },
}))
```