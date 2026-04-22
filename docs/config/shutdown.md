# shutdown

Graceful shutdown management for services and HTTP servers.

## What It Does

Coordinates shutdown of multiple services in the right order. Ensures:
- Dependencies are stopped after dependents (database after API, not before)
- Timeouts prevent hanging shutdowns
- Errors are collected and reported

## Usage

### Creating a Manager

```go
mgr := shutdown.NewManager()

// With options
mgr := shutdown.NewManager(
    shutdown.WithLogger(log.Default()),
    shutdown.WithTimeout(30*time.Second),
)
```

### Registering Services

```go
// Register shutdown functions
mgr.Register("database", func(ctx context.Context) error {
    return dbPool.Close()
})

mgr.Register("redis", func(ctx context.Context) error {
    return redisClient.Close()
})

mgr.Register("http-server", func(ctx context.Context) error {
    return server.Shutdown(ctx)
})
```

### Dependencies

```go
// Database depends on Redis - stop Redis first
mgr.Register("database", dbClose, shutdown.WithDependsOn("redis"))
mgr.Register("redis", redisClose)  // Stops first

// More complex
mgr.Register("cache", cacheClose, shutdown.WithDependsOn("redis"))
mgr.Register("api", apiClose, shutdown.WithDependsOn("cache", "database"))
```

### Triggering Shutdown

```go
// Simple
err := mgr.Shutdown(context.Background())

// With timeout on all tasks
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
err := mgr.Shutdown(ctx)

// Or wait for OS signals
err := mgr.WaitForSignal(context.Background())
// Listens for SIGINT, SIGTERM, calls Shutdown automatically
```

### Callbacks

```go
// Run code when shutdown starts
mgr.OnShutdown(func() {
    log.Println("Shutting down...")
})
```

### Errors

```go
// After shutdown
if err := mgr.Shutdown(ctx); err != nil {
    log.Printf("Shutdown had errors: %v", err)
}

// Or check later
if mgr.Error() != nil {
    // Report error
}
```

### Listing Tasks

```go
// See what's registered
tasks := mgr.Tasks()
// []string{"database", "redis", "http-server"}
```

## How It Works

1. **Sorts by dependency** using topological sort
2. **Executes in parallel** for independent tasks
3. **Respects timeouts** - each task has its own timeout
4. **Collects errors** - continues shutting down other services even if one fails

## Example Full Flow

```go
func main() {
    mgr := shutdown.NewManager()
    
    // Setup - register in dependency order (reverse of shutdown order)
    mgr.Register("db", db.Close, shutdown.WithDependsOn("cache"))
    mgr.Register("cache", cache.Close, shutdown.WithDependsOn("redis"))
    mgr.Register("redis", redis.Close)
    mgr.Register("server", server.Shutdown)
    
    // Handle OS signals
    if err := mgr.WaitForSignal(context.Background()); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

## Graceful HTTP Server Shutdown

```go
import "github.com/azghr/mesh/shutdown"

// HTTP server shutdown with draining active connections
server := &http.Server{
    Addr:    ":8080",
    Handler: router,
}

go func() {
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}()

// On shutdown signal
trap := make(chan os.Signal, 1)
signal.Notify(trap, os.Interrupt, syscall.SIGTERM)
<-trap

err := shutdown.GracefulHTTP(server, shutdown.Config{
    Timeout: 30 * time.Second,
    ShutdownHooks: []shutdown.Hook{
        func(ctx context.Context) error { return db.Close() },
        func(ctx context.Context) error { return redis.Close() },
    },
})
```

### Configuration

```go
// Config options
shutdown.Config{
    Timeout: 30 * time.Second,           // drain timeout
    PreShutdownHooks: []Hook{...},       // before server.Stop
    ShutdownHooks: []Hook{...},          // after server.Stop
}
```

## Errors

```go
var (
    ErrShutdownTimeout   = errors.New("shutdown timed out")
    ErrShutdownCancelled = errors.New("shutdown was cancelled")
)

## Enhanced Graceful Shutdown

Enhanced shutdown management with health checks, connection draining, and ordered phases.

### Enhanced Manager

```go
mgr := shutdown.NewEnhancedManager()

// Track connections
connTracker := mgr.GetConnTracker()

// In HTTP handler
func handler(w http.ResponseWriter, r *http.Request) {
    connTracker.RegisterConnection()
    defer connTracker.UnregisterConnection()
    // Handle request
}
```

### Health Checks

```go
// Add health check functions
mgr.AddHealthCheck(func(ctx context.Context) error {
    if !dbPool.IsHealthy() {
        return errors.New("database unhealthy")
    }
    return nil
})

// Functions to run before/after shutdown
mgr.AddBeforeShutdown(func() error {
    log.Println("Preparing for shutdown...")
    return nil
})

mgr.AddAfterShutdown(func() error {
    log.Println("Cleanup complete")
    return nil
})
```

### Connection Draining

```go
// Start draining (stop accepting new connections)
connTracker.StartDraining()

// Wait for all connections to close
err := connTracker.WaitForDrain(30 * time.Second)
if err != nil {
    log.Printf("Warning: %v", err)
}

// Check status
active := connTracker.ActiveConnections()
isDraining := connTracker.IsDraining()
```

### Ordered Phases

```go
// Add shutdown phases
mgr.AddPhase("stop-accepting", []string{"http"}, 1)
mgr.AddPhase("drain-connections", []string{"drain"}, 2)
mgr.AddPhase("close-resources", []string{"cache", "db"}, 3)

// Register tasks
mgr.RegisterTask("http", func(ctx context.Context) error {
    return server.Shutdown(ctx)
})

mgr.RegisterTask("cache", func(ctx context.Context) error {
    return cache.Close()
})

mgr.RegisterTask("db", func(ctx context.Context) error {
    return dbPool.Close()
})
```

### Health Check Endpoints

```go
// Get HTTP handler for health endpoints
handler := mgr.HTTPHandler()

// Endpoints:
// GET /health - Returns {"status":"healthy","connections":N}
// GET /ready - Returns {"status":"ready","connections":N} if ready and no connections
// GET /drain - Starts draining
```

### HTTP Server with Graceful Drain

```go
err := shutdown.GracefulDrain(server, shutdown.DrainConfig{
    Timeout:      30 * time.Second,
    GracePeriod:  5 * time.Second,
    OnDrainStarted: func() {
        log.Println("Started draining connections")
    },
    OnAllDrained: func() {
        log.Println("All connections drained")
    },
})
```

### API Summary

| Method | Description |
|--------|-------------|
| `NewEnhancedManager()` | Create enhanced manager |
| `RegisterTask(name, fn, opts...)` | Register shutdown task |
| `AddPhase(name, tasks, order)` | Add shutdown phase |
| `AddHealthCheck(fn)` | Add health check |
| `AddBeforeShutdown(fn)` | Add pre-shutdown function |
| `AddAfterShutdown(fn)` | Add post-shutdown function |
| `GetConnTracker()` | Get connection tracker |
| `GetHealthChecker()` | Get health checker |
| `HTTPHandler()` | Get HTTP handler for /health, /ready, /drain |
| `GracefulDrain(srv, cfg)` | Gracefully drain HTTP server |

| ConnTracker Method | Description |
|------------------|-------------|
| `RegisterConnection()` | Register active connection |
| `UnregisterConnection()` | Unregister connection |
| `ActiveConnections()` | Get connection count |
| `StartDraining()` | Start draining mode |
| `IsDraining()` | Check if draining |
| `WaitForDrain(timeout)` | Wait for all to drain |

| HealthChecker Method | Description |
|---------------------|-------------|
| `SetHealthy(bool)` | Set healthy status |
| `SetReady(bool)` | Set ready status |
| `IsHealthy()` | Check healthy |
| `IsReady()` | Check ready |
| `AddHealthCheck(fn)` | Add health check |
```