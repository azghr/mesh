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

## Enhanced Probes (Kubernetes)

Kubernetes-style liveness, readiness, and startup probes with deep checks.

### Quick Start

```go
// Create deep health checker
checker := health.NewDeepHealthChecker()

// Register probes for different check types
checker.RegisterProbe(health.ProbeDefinition{
    Name:  "liveness",
    Type:  health.CheckTypeLiveness,
    Check: health.DatabaseLivenessCheck(db),
    Config: health.ProbeConfig{Timeout: 5 * time.Second},
})

checker.RegisterProbe(health.ProbeDefinition{
    Name:  "database",
    Type:  health.CheckTypeReadiness,
    Check: health.DatabaseReadinessCheck(db, health.ProbeConfig{
        MaxConnectionUtilization: 0.9,
    }),
})

checker.RegisterProbe(health.ProbeDefinition{
    Name:  "redis",
    Type:  health.CheckTypeReadiness,
    Check: health.RedisReadinessCheck(redisClient, health.ProbeConfig{}),
})
```

### Check Types

```go
const (
    CheckTypeLiveness  CheckType = "liveness"  // /healthz
    CheckTypeReadiness CheckType = "readiness" // /readyz
    CheckTypeStartup   CheckType = "startup"   // /startupz
)
```

### Probe Configuration

```go
type ProbeConfig struct {
    Timeout                  time.Duration // Check timeout (default: 5s liveness, 10s readiness)
    Critical                bool          // If true, failure affects overall status
    MaxConnectionUtilization float64       // 0.0-1.0, e.g., 0.9 = 90% max utilization
}
```

### Deep Database Check

```go
// Readiness: queries DB + checks connection pool utilization
checker.RegisterProbe(health.ProbeDefinition{
    Name:   "database",
    Type:   health.CheckTypeReadiness,
    Check:  health.DatabaseReadinessCheck(db, health.ProbeConfig{
        Timeout:                  3 * time.Second,
        MaxConnectionUtilization: 0.9, // Fail if >90% connections used
    }),
})

// Liveness: simple ping
checker.RegisterProbe(health.ProbeDefinition{
    Name:   "database-liveness",
    Type:   health.CheckTypeLiveness,
    Check:  health.DatabaseLivenessCheck(db),
})
```

### Deep Redis Check

```go
checker.RegisterProbe(health.ProbeDefinition{
    Name:  "redis",
    Type:  health.CheckTypeReadiness,
    Check: health.RedisReadinessCheck(redisClient, health.ProbeConfig{
        Timeout: 3 * time.Second,
    }),
})
```

### Checking Probes

```go
// Check readiness
readiness := checker.Readiness(ctx)
// Returns: {Status: "pass", Checks: {...}}

// Check liveness
livenessResults := checker.CheckProbes(ctx, health.CheckTypeLiveness)

// Check all
allResults := checker.AllChecks(ctx)

// Simple health check
if checker.IsHealthy(ctx, health.CheckTypeReadiness) {
    // Ready to serve traffic
}
```

### Setup Helpers

```go
// Setup default probes for typical stack
health.SetupDefaultProbes(checker, db, redisClient)
```

### Composite Checks

```go
// All checks must pass
composite := health.RequireAllCheck("critical", check1, check2)

// At least one must pass
fallback := health.RequireAnyCheck("backup", primaryCheck, backupCheck)
```

### Kubernetes Endpoints

```go
// GET /healthz - Liveness probe
app.Get("/healthz", func(c *fiber.Ctx) error {
    results := checker.CheckProbes(c.Context(), health.CheckTypeLiveness)
    return c.JSON(results)
})

// GET /readyz - Readiness probe
app.Get("/readyz", func(c *fiber.Ctx) error {
    readiness := checker.Readiness(c.Context())
    status := fiber.StatusOK
    if readiness.Status == health.StatusFail {
        status = fiber.StatusServiceUnavailable
    }
    return c.Status(status).JSON(readiness)
})

// GET /startupz - Startup probe
app.Get("/startupz", func(c *fiber.Ctx) error {
    results := checker.CheckProbes(c.Context(), health.CheckTypeStartup)
    // ...
})
```