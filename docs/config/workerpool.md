# workerpool

Goroutine pool with backpressure for CPU/IO-bound tasks.

## What It Does

Limits the number of concurrent goroutines and provides backpressure when the queue is full. Useful for:
- Rate limiting background workers
- Bounded concurrency for resource-intensive tasks
- Clean shutdown of parallel work

## Usage

### Basic

```go
pool := workerpool.New(
    workerpool.WithSize(4),        // 4 workers (default: 4)
    workerpool.WithQueueSize(100), // 100 pending tasks (default: 100)
)
pool.Start()

// Submit work (tasks receive context for cancellation)
pool.Submit(ctx, func(ctx context.Context) {
    doWork(ctx)
})
```

### Submit Options

```go
// Blocking submit (waits for queue space)
err := pool.Submit(ctx, task)
if err == context.DeadlineExceeded {
    // Queue full, try again later
}

// Non-blocking submit (returns immediately)
err := pool.SubmitNonBlocking(task)
if err == workerpool.ErrQueueFull {
    // Queue is full
}
```

### Graceful Shutdown

```go
// Wait for pending tasks, stop accepting new ones
pool.Shutdown()

// Or with timeout
err := pool.ShutdownWithContext(ctx)
```

### Monitoring

```go
// Is pool running?
if pool.Running() {
    // Accepting work
}

// How many tasks pending?
queueLen := pool.QueueLength()
```

## Configuration Options

```go
// Number of concurrent workers
workerpool.WithSize(8)

// Size of task queue
workerpool.WithQueueSize(200)

// Default timeout for each task
workerpool.WithTaskTimeout(30*time.Second)
```

### Task Timeout

```go
// Pool-level timeout (applies to all tasks)
pool := workerpool.New(workerpool.WithTaskTimeout(30 * time.Second))

// Per-task timeout
err := pool.SubmitWithTimeout(ctx, task, 10*time.Second)

// Runtime timeout update
pool.SetTaskTimeout(30 * time.Second)
```

## When to Use

**Use when:**
- You want to limit concurrency (e.g., don't spawn 1000 goroutines)
- You need clean shutdown (wait for pending work)
- You want backpressure (caller slows down when queue fills)

**Don't use when:**
- Work is trivial (goroutines are cheap)
- You need dynamic sizing (use `golang.org/x/sync/errgroup`)
- You need result collection (use `errgroup` or channels)