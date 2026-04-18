# logger

Structured logging for services, with pretty terminal output for development and JSON for production.

## What It Does

Wraps Go's standard `slog` with a nice terminal-friendly handler and service-aware formatting. Supports component context, operation tagging, and global logger convenience.

## Usage

### Creating a Logger

```go
// New(serviceName, level, jsonFormat)
log := logger.New("auth", "debug", false)  // Pretty terminal output
log := logger.New("auth", "info", true)    // JSON for production

// Or use global logger
logger.SetGlobal(log)
log := logger.GetGlobal()
```

### Logging

```go
log.Debug("debug message", "key", "value")
log.Info("user logged in", "user_id", userID)
log.Warn("rate limit approaching", "remaining", 10)
log.Error("database query failed", "error", err)

// Fatal exits the program
log.Fatal("unrecoverable error", "error", err)
```

### Structured Fields

```go
// Multiple fields
log.Info("request processed", 
    "request_id", reqID,
    "user_id", userID,
    "duration_ms", duration.Milliseconds())

// With fields (returns new logger)
logged := log.With("user_id", userID, "role", "admin")
logged.Info("admin action")
```

### Component/Operation Context

```go
// Tag operations for better tracing
log.WithComponent("database").Info("query executed")
log.WithOperation("user.create").Info("operation started")
log.WithContext(ctx).Info("request received") // Includes trace_id if present
```

### Global Convenience Functions

```go
// Use global logger directly
logger.Info("service started", "port", 8080)
logger.Error("connection failed", "error", err)

// With helpers
logger.WithComponent("auth").Warn("token expired")
```

## Banner Service

Startup banners with service name and color:

```go
// Print a startup banner
logger.PrintBanner("auth", "1.0.0")
// Output:
// [AUTH  ] [cyan]MESH[/cyan]
// [AUTH  ] Version: 1.0.0 | Starting up...

// Or with custom message
logger.PrintBannerWithMessage("api", "1.0.0", "Initializing...")

// Register custom banner
logger.RegisterBanner("payments", "═══════")
logger.RegisterBannerColor("payments", logger.Green)
```

## Levels

```go
// String to Level
level := logger.ParseLevel("debug") // LevelDebug

// Level values
logger.LevelDebug  // 0
logger.LevelInfo   // 1
logger.LevelWarn   // 2
logger.LevelError  // 3

// Set level at runtime
log.SetLevel(logger.LevelDebug)
```

## Output Formats

### Terminal (JSON = false)
```
[15:04:05] AUTH  |INFO  user logged in user_id=123
[15:04:05] AUTH  |ERROR database query failed error="connection refused"
```

### JSON (JSON = true)
```json
{"time":"2024-01-15T15:04:05Z","level":"INFO","msg":"user logged in","service":"auth","user_id":"123"}
```

## Context Helpers

```go
// Add trace ID to context
ctx := logger.WithTraceID(ctx, "trace-123")

// Get trace ID from context
traceID := logger.GetTraceID(ctx)
```

## Log Aggregation

Ship logs to a central aggregation service:

```go
logger.SetAggregator(logger.AggregatorConfig{
    Endpoint:     "https://logs.example.com/api/v1/logs",
    BatchSize:    100,            // Max logs per batch
    FlushInterval: 5*time.Second, // Flush interval
    RetryConfig: logger.RetryConfig{
        MaxRetries: 3,
        Backoff:    time.Second,
    },
})
```

### How it works

1. Logs are buffered locally
2. On batch size (100) or interval (5s), flush to endpoint
3. On failure, retry with exponential backoff
4. On success, continue; on final failure, log error

### Example

```go
// Setup
log := logger.New("api", "info", false)
logger.SetAggregator(logger.AggregatorConfig{
    Endpoint:    "https://logs.example.com/api/v1/logs",
    BatchSize:   100,
    FlushInterval: 5 * time.Second,
})

// Logs are automatically shipped
log.Info("request processed", "trace_id", traceID, "user_id", userID)

// Shutdown - flush remaining logs
logger.StopAggregation()
```