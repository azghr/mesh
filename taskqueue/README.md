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
queue.Enqueue(ctx, "email", map[string]interface{}{
    "to": "user@example.com",
    "subject": "Hello",
})

// With delay
queue.EnqueueWithDelay(ctx, "email", payload, 5*time.Minute)
```

### Process Jobs

```go
worker := queue.Worker("email")

err := worker.Process(ctx, func(ctx context.Context, payload json.RawMessage) error {
    var data map[string]string
    json.Unmarshal(payload, &data)
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

## API

### `New(client *redis.Client) *Queue`

Creates a new queue.

### `queue.Enqueue(ctx, name string, payload interface{}) error`

Enqueues a job.

### `queue.EnqueueWithDelay(ctx, name string, payload interface{}, delay time.Duration) error`

Enqueues a job with delay.

### `queue.Worker(name string, opts ...Option) *Worker`

Creates a worker.

### `worker.Process(ctx, handler JobHandler) error`

Starts processing jobs.

### `queue.QueueStats(ctx, name string) (Stats, error)`

Returns queue statistics.

## Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/azghr/mesh/taskqueue"
    "github.com/redis/go-redis/v9"
)

func main() {
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    queue := taskqueue.New(client)

    // Producer: enqueue jobs
    go func() {
        for {
            queue.Enqueue(context.Background(), "emails", map[string]string{
                "to": "user@example.com",
            })
            time.Sleep(time.Second)
        }
    }()

    // Worker: process jobs
    worker := queue.Worker("emails")
    worker.Process(context.Background(), func(ctx context.Context, payload json.RawMessage) error {
        log.Println("Processing job:", string(payload))
        return nil
    })
}
```