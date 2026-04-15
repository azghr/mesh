# health

Health checks for Kubernetes readiness/liveness probes.

## What It Does

Provides a flexible health check system that tracks the status of dependencies (database, Redis, etc.) and exposes them via HTTP for Kubernetes probes.

## Usage

### Creating a Checker

```go
checker := health.NewChecker()
```

### Registering Checks

```go
// Simple check
checker.Register("database", func(ctx context.Context) error {
    return db.Ping(ctx)
})

checker.Register("redis", func(ctx context.Context) error {
    return redisClient.Ping(ctx).Err()
})

// Or with custom timeout
checker.RegisterWithOptions("external-api", checkFn, 10*time.Second)
```

### Running Checks

```go
// Check all
results := checker.Check(ctx)
// Returns map of check name -> Result

// Check one specific
result, err := checker.CheckOne(ctx, "database")

// Get overall status
status := checker.Status(ctx)
// Returns StatusPass, StatusFail, or StatusWarn
```

### Result Structure

```go
type Result struct {
    Name      string    // Check name
    Status    Status    // "pass", "fail", "warn"
    Error     string    // Error message if failed
    Timestamp time.Time // When check ran
    Duration  string    // How long it took
}
```

### Listing Checks

```go
// List all registered checks
names := checker.List()

// Get count
count := checker.Count()

// Get last known status for a check
status, err := checker.GetLastStatus("database")
lastChecked, err := checker.GetLastChecker("database")
```

## HTTP Handlers

Use with your HTTP server:

```go
import "github.com/azghr/mesh/health"

// Readiness (includes all dependencies)
router.GET("/ready", func(c *fiber.Ctx) error {
    results := checker.Check(c.Context())
    // Return 200 if all pass, 503 if any fail
    ...
})

// Liveness (simple check)
router.GET("/live", func(c *fiber.Ctx) error {
    return c.SendStatus(fiber.StatusOK)
})
```

Or use the built-in HTTP handler in `health/http.go`.

## Pre-built Checkers

```go
// Database health check
checker.Register("database", health.DatabaseCheck(dbPool))

// Redis health check  
checker.Register("redis", health.RedisCheck(redisClient))
```

## Status Values

```go
const (
    StatusPass    Status = "pass"     // All good
    StatusFail    Status = "fail"     // Failed
    StatusWarn    Status = "warn"     // Degraded
    StatusUnknown Status = "unknown"  // Not checked
)
```