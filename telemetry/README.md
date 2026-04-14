# telemetry

Observability - metrics, tracing, and logging integration.

## What It Does

Provides Prometheus metrics and OpenTelemetry tracing with HTTP/gRPC middleware.

## Metrics

### Initialization

```go
telemetry.InitMetrics(&telemetry.MetricsConfig{
    ServiceName: "my-service",
    Enabled:     true,
    Buckets:     prometheus.DefBuckets,  // or custom
})
```

### Recording Metrics

```go
// HTTP requests
telemetry.RecordHTTPRequest("my-service", "GET", "/users", 200, duration)
telemetry.IncrementHTTPRequestsInFlight("my-service")
telemetry.DecrementHTTPRequestsInFlight("my-service")

// Database queries
telemetry.RecordDBQuery("my-service", "SELECT", true, duration)

// External API calls
telemetry.RecordExternalAPICall("my-service", "stripe", "/charges", true, duration)

// Cache
telemetry.RecordCacheHit("my-service", "users")
telemetry.RecordCacheMiss("my-service", "users")
```

### Exposing Metrics

```go
// Add to your HTTP server
router.Get("/metrics", func(c *fiber.Ctx) error {
    return c.SendString(telemetry.Handler().ServeHTTP(c.Response()))
})
// Or use promhttp.Handler() directly
```

## Tracing

### Initialization

```go
// Development (prints to stdout)
telemetry.InitTracing(telemetry.DefaultConfig("my-service"))

// Production (sends to OTLP collector)
telemetry.InitTracing(&telemetry.TraceConfig{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0",
    Environment:    "production",
    Exporter:       telemetry.ExporterOTLP,
    OTLPEndpoint:   "localhost:4317",
    TLSEnabled:     true,
    SampleRate:     0.1,  // 10% of traces
})
```

### Creating Spans

```go
// Basic span
ctx, span := telemetry.StartSpan(ctx, "fetch-user")
defer span.End()

// With attributes
ctx, span := telemetry.StartSpanWithAttributes(ctx, "fetch-user", map[string]interface{}{
    "user_id": userID,
})

// Add attributes to current span
telemetry.AddAttributes(ctx, map[string]interface{}{
    "key": "value",
})

// Add event
telemetry.AddEvent(ctx, "cache-miss", map[string]interface{}{
    "key": "user:123",
})

// Record error
telemetry.RecordError(ctx, err)
```

### Context Helpers

```go
// Add to context for propagation
ctx = telemetry.WithOperation(ctx, "user.create")
ctx = telemetry.WithComponent(ctx, "database")
ctx = telemetry.WithUserID(ctx, userID)
ctx = telemetry.WithRequestID(ctx, requestID)

// Get trace info
traceID := telemetry.GetTraceID(ctx)
spanID := telemetry.GetSpanID(ctx)
```

### HTTP Middleware

```go
// Auto-trace HTTP requests
router.Use(telemetry.TraceHTTPMiddleware("my-service"))
```

### gRPC Middleware

```go
// Auto-trace gRPC calls
server := grpc.NewServer(
    grpc.UnaryInterceptor(telemetry.TraceGRPCMiddleware("my-service")),
)
```

### Context Propagation

```go
// Inject trace context into outgoing requests (e.g., to another service)
headers := telemetry.InjectContext(ctx)
// Add headers to HTTP request or gRPC metadata

// Extract trace context from incoming requests
ctx := telemetry.ExtractContext(ctx, headers)
```

### Shutdown

```go
// Call on service shutdown
telemetry.ShutdownTracing(ctx)
```

## Fiber Integration

```go
// Use built-in middleware
app.Use(telemetry.FiberMiddleware("my-service"))
// Adds: request duration metrics, tracing
```

## HTTP Middleware (for net/http)

```go
// Use with standard library
handler := telemetry.HTTPMiddleware("my-service", nextHandler)
```