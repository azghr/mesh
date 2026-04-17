# eventbus

Simple publish/subscribe for in-process event handling.

## What It Does

Lightweight pub/sub for decoupled communication within a service. Not for cross-service communication (use a message queue for that).

## Usage

### Basic Pub/Sub

```go
bus := eventbus.New()

// Subscribe to a topic
bus.Subscribe("user.created", func(payload any) {
    user := payload.(map[string]string)
    fmt.Println("User created:", user["id"])
})

// Publish an event
bus.Publish("user.created", map[string]string{
    "id":   "123",
    "name": "John",
})
```

### Unsubscribe

```go
// Subscribe returns an unsubscribe function
unsubscribe := bus.Subscribe("order.completed", handler)

// Later, stop receiving events
unsubscribe()
```

### Sync Publishing

```go
// Async (fire and forget) - handlers run in goroutines
bus.Publish("event", data)

// Sync - handlers run in same goroutine, waits for completion
bus.PublishSync("event", data)
```

### One-time Subscription

```go
// Handler runs once, then auto-unsubscribes
bus.SubscribeOnce("init", func(payload any) {
    fmt.Println("Initialized once")
})
```

### Managing Subscriptions

```go
// Remove specific handler
bus.Unsubscribe("topic", handler)

// Clear all handlers for a topic
bus.Clear("topic")

// Clear everything
bus.ClearAll()

// Get topic info
topics := bus.Topics()       // []string
count := bus.HandlerCount("topic") // int
```

### JSON Payloads

```go
// Auto-unmarshal JSON
bus.JSONSubscribe("user.created", func(data any) error {
    // data is already parsed JSON
    user := data.(map[string]interface{})
    return nil
})
```

## Global Bus

Package-level convenience for simple use cases:

```go
// Subscribe globally
eventbus.GlobalSubscribe("app.started", func(p any) { ... })

// Publish globally
eventbus.GlobalPublish("app.started", nil)

// Clear global bus
eventbus.GlobalClear()
```

## When to Use

- Decoupling parts of your service (e.g., when user signs up, notify multiple systems)
- Simple event-driven patterns
- **Not** for: cross-service events, durable events, distributed systems

## When NOT to Use

- Need persistence (events shouldn't vanish on crash) → use a message queue
- Need exactly-once delivery → use a message queue
- Cross-service communication → use a message queue

## Performance

- Handlers run concurrently (Publish)
- No ordering guarantees
- No delivery confirmations
- Lightweight - good for thousands of events/second in single service

## Dead Letter Queue

Reliable message processing with retry and dead letter handling.

### Basic Usage

```go
// Create reliable bus with retry handling
bus := eventbus.NewReliableBus(3, 100*time.Millisecond, 100)
// 3 retries, 100ms delay, queue capacity 100
```

### Subscribe with Retry

```go
// Subscribe with automatic retry
bus.SubscribeWithRetry("orders.created", func(payload any) {
    // Process order - will retry 3 times on failure
    processOrder(payload)
})

// Subscribe with retry AND dead letter queue
bus.SubscribeWithRetryAndDLQ("orders.created", func(payload any) {
    processOrder(payload)
})
```

### Handle Dead Letters

```go
// Custom handler for failed messages
bus.SetDLQHandler(func(dl eventbus.DeadLetter) {
    fmt.Printf("Failed: topic=%s error=%v attempts=%d\n",
        dl.Topic, dl.Error, dl.Attempts)
    // Send to alerting, store in database, etc.
})
```

### Retry Failed Messages

```go
// Reprocess all dead letters
bus.RetryFailed(ctx)

// Access dead letter queue
dlq := bus.DLQ()
letters := dlq.GetAll()
size := dlq.Size()
dlq.Clear()
```

### Dead Letter Structure

```go
type DeadLetter struct {
    Topic     string          // Original topic
    Payload   json.RawMessage // Original message
    Error     string        // Last error
    Attempts int           // Retry count
    Received time.Time   // When added to DLQ
    LastTry  time.Time   // Last retry time
}
```

### Best Practices

- Set retry count (3-5) based on expected failures
- Use exponential backoff for transient failures
- Monitor DLQ size - alert if > 100
- Have process to review/replay dead letters
- Log all dead letters for debugging

## When to Use

- Decoupling parts of your service (e.g., when user signs up, notify multiple systems)
- Simple event-driven patterns
- **Not** for: cross-service events, durable events, distributed systems