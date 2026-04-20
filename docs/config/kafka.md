# Kafka

Kafka producer and consumer for message streaming.

## Overview

This package provides a simple interface for producing and consuming Kafka messages with support for consumer groups.

Note: Currently provides interface definition and stub implementation. Full Kafka support requires adding `confluent-kafka-go` dependency.

## Features

- Simple producer interface
- Consumer with group support
- JSON message encoding
- Context-based cancellation

## Installation

```go
import "github.com/azghr/mesh/kafka"
```

## Usage

### Producer

```go
producer, err := kafka.NewProducer(kafka.Config{
    Brokers: []string{"localhost:9092"},
    Topic:  "events",
})
if err != nil {
    log.Fatal(err)
}

// Send single message
err := producer.Send(ctx, "user:123", map[string]interface{}{
    "event":   "user.created",
    "user_id": "123",
})
if err != nil {
    log.Fatal(err)
}

// Send multiple messages
err := producer.SendMany(ctx, []kafka.Message{
    {Key: "key1", Value: map[string]string{"event": "test1"}},
    {Key: "key2", Value: map[string]string{"event": "test2"}},
})
```

### Consumer

```go
consumer, err := kafka.NewConsumer(kafka.Config{
    Brokers: []string{"localhost:9092"},
    Topic:  "events",
    Group:  "my-group",
})
if err != nil {
    log.Fatal(err)
}

err := consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
    fmt.Printf("received: %s %s\n", key, string(value))
    return nil
})
```

## Configuration

| Parameter | Description | Required |
|-----------|-------------|-----------|
| `Brokers` | Kafka broker addresses | Yes |
| `Topic` | Topic name | Yes |
| `Group` | Consumer group ID | Yes (consumer) |

## API Reference

### `NewProducer(cfg Config) (*Producer, error)`

Creates a new Kafka producer.

### `producer.Send(ctx context.Context, key string, value interface{}) error`

Sends a message to the topic. Value is JSON-encoded.

### `producer.SendMany(ctx context.Context, messages []Message) error`

Sends multiple messages.

### `producer.Close() error`

Closes the producer.

### `NewConsumer(cfg Config) (*Consumer, error)`

Creates a new Kafka consumer.

### `consumer.Consume(ctx context.Context, handler Handler) error`

Starts consuming messages. Blocks until context is cancelled.

### `Handler func(ctx context.Context, key string, value []byte) error`

Function type for processing messages.

### `consumer.Close() error`

Closes the consumer.

## Example: Event Streaming

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/kafka"
)

func main() {
    // Producer
    producer, _ := kafka.NewProducer(kafka.Config{
        Broker: []string{"localhost:9092"},
        Topic:  "user-events",
    })
    defer producer.Close()

    // Emit events
    for i := 0; i < 10; i++ {
        producer.Send(context.Background(), 
            "user:123", 
            map[string]interface{}{
                "type": "login",
                "ts":   i,
            })
    }

    log.Println("Events produced")
}
```

## Example: Event Processing

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/kafka"
)

func main() {
    consumer, _ := kafka.NewConsumer(kafka.Config{
        Brokers: []string{"localhost:9092"},
        Topic:  "user-events",
        Group:  "processors",
    })
    defer consumer.Close()

    log.Println("Starting consumer...")

    err := consumer.Consume(context.Background(), 
        func(ctx context.Context, key string, value []byte) error {
            log.Printf("Processing: %s = %s", key, string(value))
            return nil
        })
    if err != nil {
        log.Println("Consumer stopped:", err)
    }
}
```