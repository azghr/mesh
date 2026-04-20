# Task Queue

Redis-based asynchronous task queue for job processing.

## Overview

This package provides a Redis-based task queue for processing jobs asynchronously. It supports multiple queues, delayed execution, retry with backoff, and dead letter queue.

## Features

- Multiple queues
- Delayed execution
- Retry with exponential backoff
- Dead letter queue for failed jobs
- Job statistics

## Installation

```go
import "github.com/azghr/mesh/taskqueue"
```

## Usage

### Enqueue Jobs

```go
queue := taskqueue.New(redisClient)

// Simple enqueue
err := queue.Enqueue(ctx, "email", map[string]interface{}{
    "to": "user@example.com",
    "subject": "Hello",
})
if err != nil {
    log.Fatal(err)
}

// With delay (e.g., send 5 minutes later)
err := queue.EnqueueWithDelay(ctx, "email", payload, 5*time.Minute)
```

### Process Jobs

```go
worker := queue.Worker("email")

err := worker.Process(ctx, func(ctx context.Context, payload json.RawMessage) error {
    var data map[string]string
    if err := json.Unmarshal(payload, &data); err != nil {
        return err
    }
    
    // Process the job
    return sendEmail(data["to"], data["subject"])
})
```

### Options

```go
worker := queue.Worker("email",
    taskqueue.WithMaxRetries(5),
    taskqueue.WithRetryDelay(2*time.Second),
    taskqueue.WithPollInterval(500*time.Millisecond),
)
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxRetries(n)` | 3 | Maximum retry attempts |
| `WithRetryDelay(d)` | 1s | Delay between retries |
| `WithPollInterval(d)` | 1s | Polling interval |

## API Reference

### `New(client *redis.Client) *Queue`

Creates a new task queue.

### `queue.Enqueue(ctx context.Context, name string, payload interface{}) error`

Enqueues a job to the specified queue.

### `queue.EnqueueWithDelay(ctx context.Context, name string, payload interface{}, delay time.Duration) error`

Enqueues a job with delay before processing.

### `queue.Worker(name string, opts ...Option) *Worker`

Creates a worker for the specified queue.

### `worker.Process(ctx context.Context, handler JobHandler) error`

Starts processing jobs from the queue. Blocks until context is cancelled.

### `queue.QueueStats(ctx context.Context, name string) (Stats, error)`

Returns statistics for a queue:

```go
stats, _ := queue.QueueStats(ctx, "emails")
fmt.Printf("Pending: %d, Failed: %d, Delayed: %d\n", 
    stats.Length, stats.DeadLetter, stats.Delayed)
```

### `JobHandler func(ctx context.Context, payload json.RawMessage) error`

Function type for processing jobs.

## Example

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "time"

    "github.com/azghr/mesh/taskqueue"
    "github.com/redis/go-redis/v9"
)

func main() {
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()
    
    queue := taskqueue.New(client)
    ctx := context.Background()
    
    // Start worker in background
    go func() {
        worker := queue.Worker("notifications",
            taskqueue.WithMaxRetries(3),
            taskqueue.WithRetryDelay(time.Second),
        )
        
        worker.Process(ctx, func(ctx context.Context, payload json.RawMessage) error {
            var msg map[string]string
            if err := json.Unmarshal(payload, &msg); err != nil {
                return err
            }
            
            log.Printf("Sending notification to %s: %s", msg["to"], msg["body"])
            return sendNotification(msg["to"], msg["body"])
        })
    }()
    
    // Producer: enqueue jobs
    for i := 0; i < 10; i++ {
        queue.Enqueue(ctx, "notifications", map[string]string{
            "to":   fmt.Sprintf("user%d@example.com", i),
            "body": "Hello!",
        })
        time.Sleep(100 * time.Millisecond)
    }
    
    // Keep running
    select {}
}
```

## Retry Behavior

When a job handler returns an error:

1. Job is requeued with incremented retry count
2. Retry delay increases exponentially: `baseDelay * retries`
3. After max retries, job goes to dead letter queue

Use `queue.QueueStats()` to monitor failed jobs.