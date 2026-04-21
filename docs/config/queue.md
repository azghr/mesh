# queue

In-memory job queue for asynchronous task processing.

## What It Does

Provides a simple in-memory queue:
- Multiple named queues
- Worker pool for processing
- Retry support
- Graceful shutdown

## Usage

### Create Queue

```go
queue := queue.New(
    queue.WithBufferSize(100),
    queue.WithWorkers(4),
)
```

### Enqueue Jobs

```go
err := queue.Enqueue(ctx, queue.Job{
    Type:    "send_email",
    Payload: map[string]any{
        "to":      "user@example.com",
        "subject": "Hello",
    },
})
```

### Process Jobs

```go
worker := queue.Worker("send_email")
worker.Start(ctx, func(ctx context.Context, job queue.Job) error {
    payload := job.Payload.(map[string]any)
    to := payload["to"].(string)
    
    return sendEmail(ctx, to)
})
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| WithBufferSize | 100 | Queue buffer size |
| WithWorkers | 1 | Number of workers |

## Job Structure

```go
type Job struct {
    ID         string    // Unique ID (auto-generated if empty)
    Type       string    // Job type/queue name
    Payload    any       // Job data
    Retries    int       // Current retry count
    MaxRetries int       // Max retries (default: 3)
    CreatedAt  time.Time // Creation timestamp
}
```

## Error Handling

```go
// Queue full
err := queue.Enqueue(ctx, job)
// Returns ErrQueueFull if buffer is full

// Queue not found
job, err := queue.Dequeue(ctx, "nonexistent")
// Returns ErrQueueNotFound
```

## Multiple Queues

```go
// Different job types use different queues
queue.Enqueue(ctx, queue.Job{Type: "email", Payload: ...})
queue.Enqueue(ctx, queue.Job{Type: "sms", Payload: ...})
queue.Enqueue(ctx, queue.Job{Type: "notification", Payload: ...})

// Workers for each type
emailWorker := queue.Worker("email")
smsWorker := queue.Worker("sms")
```

## Example: Email Worker

```go
func main() {
    q := queue.New(queue.WithBufferSize(1000))

    // Email worker
    emailWorker := q.Worker("email")
    emailWorker.Start(context.Background(), func(ctx context.Context, job queue.Job) error {
        payload := job.Payload.(map[string]any)
        
        to := payload["to"].(string)
        subject := payload["subject"].(string)
        
        return sendEmail(to, subject)
    })

    // Enqueue jobs
    q.Enqueue(context.Background(), queue.Job{
        Type: "email",
        Payload: map[string]any{
            "to":      "user1@example.com",
            "subject": "Welcome!",
        },
    })

    // Cleanup
    defer q.Close()
}
```

## Graceful Shutdown

```go
q := queue.New()

// ... add workers ...

// On shutdown
if err := q.Close(); err != nil {
    log.Printf("queue close error: %v", err)
}
```

## Retry

Jobs automatically retry on failure (up to MaxRetries):

```go
job := queue.Job{
    Type:       "email",
    Payload:    data,
    MaxRetries: 5, // retry up to 5 times
}
```

## With Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := queue.Enqueue(ctx, job)
// Returns ctx.Err() on timeout/cancellation
```

## Difference from taskqueue

| Feature | queue | taskqueue |
|---------|-------|-----------|
| Storage | In-memory | Redis |
| Persistence | No | Yes |
| Distributed | No | Yes |
| Use case | Single instance | Multi-instance |
