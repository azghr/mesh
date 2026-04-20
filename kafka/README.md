# Kafka

Kafka producer and consumer for message streaming.

## Overview

This package provides a simple interface for producing and consuming Kafka messages. Note: Currently provides interface and stubs. Full Kafka support requires `confluent-kafka-go` or similar library.

## Installation

```go
import "github.com/azghr/mesh/kafka"
```

## Usage

### Producer

```go
producer, _ := kafka.NewProducer(kafka.Config{
    Brokers: []string{"localhost:9092"},
    Topic:  "events",
})

producer.Send(ctx, "key", map[string]string{
    "event": "user.created",
})
```

### Consumer

```go
consumer, _ := kafka.NewConsumer(kafka.Config{
    Brokers: []string{"localhost:9092"},
    Topic:  "events",
    Group:  "my-group",
})

consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
    fmt.Printf("received: %s %s\n", key, string(value))
    return nil
})
```

## API

### Producer

`NewProducer(cfg Config) (*Producer, error)` - Creates producer

`producer.Send(ctx, key, value) error` - Sends message

`producer.SendMany(ctx, messages) error` - Sends multiple messages

### Consumer

`NewConsumer(cfg Config) (*Consumer, error)` - Creates consumer

`consumer.Consume(ctx, handler) error` - Consumes messages

Note: Requires Kafka library dependency for full implementation.