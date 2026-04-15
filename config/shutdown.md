# shutdown

Graceful shutdown management with dependency ordering.

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

## Errors

```go
var (
    ErrShutdownTimeout   = errors.New("shutdown timed out")
    ErrShutdownCancelled = errors.New("shutdown was cancelled")
)
```