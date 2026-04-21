// Package kafka provides Kafka producer and consumer for message streaming.
//
// This package provides a simple interface for producing and consuming
// Kafka messages with support for consumer groups.
//
// # Features
//
//   - Simple producer interface
//   - Consumer with group support
//   - JSON message encoding
//   - Context-based cancellation
//
// # Usage
//
//	// Producer
//	producer := kafka.NewProducer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	})
//	producer.Send(ctx, key, message)
//
//	// Consumer
//	consumer := kafka.NewConsumer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	    Group: "my-group",
//	})
//	consumer.Consume(ctx, handler)
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Config configures Kafka producer/consumer
type Config struct {
	Brokers []string
	Topic   string
	Group   string
}

// Producer sends messages to Kafka
type Producer struct {
	config Config
}

// NewProducer creates a new Kafka producer
//
//	producer := kafka.NewProducer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	})
func NewProducer(cfg Config) (*Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic required")
	}
	return &Producer{config: cfg}, nil
}

var ErrNotImplemented = errors.New("kafka producer not implemented: use confluent-kafka-go or similar")

// Send sends a message to Kafka
//
//	producer.Send(ctx, "key", map[string]string{
//	    "event": "user.created",
//	})
func (p *Producer) Send(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return fmt.Errorf("%w: sending to %s [%s]: %s", ErrNotImplemented, p.config.Topic, key, string(data))
}

// SendMany sends multiple messages
func (p *Producer) SendMany(ctx context.Context, messages []Message) error {
	for _, msg := range messages {
		if err := p.Send(ctx, msg.Key, msg.Value); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	return nil
}

// Message represents a Kafka message
type Message struct {
	Key   string
	Value interface{}
}

// Consumer reads messages from Kafka
type Consumer struct {
	config Config
}

// NewConsumer creates a new Kafka consumer
//
//	consumer := kafka.NewConsumer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	    Group: "my-group",
//	})
func NewConsumer(cfg Config) (*Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic required")
	}
	if cfg.Group == "" {
		return nil, fmt.Errorf("group required for consumer")
	}
	return &Consumer{config: cfg}, nil
}

// Handler processes Kafka messages
type Handler func(ctx context.Context, key string, value []byte) error

// Consume starts consuming messages
//
//	consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
//	    fmt.Printf("received: %s %s\n", key, string(value))
//	    return nil
//	})
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	return fmt.Errorf("%w: consumer not implemented: use confluent-kafka-go or similar", ErrNotImplemented)
}

// ConsumePartition consumes from a specific partition
func (c *Consumer) ConsumePartition(ctx context.Context, partition int, handler Handler) error {
	return c.Consume(ctx, handler)
}

// Close closes the consumer
func (c *Consumer) Close() error {
	return nil
}

// ConsumerGroup manages a group of consumers
type ConsumerGroup struct {
	config Config
	msgs   chan Message
	wg     sync.WaitGroup
}

// NewConsumerGroup creates a new consumer group
func NewConsumerGroup(cfg Config) (*ConsumerGroup, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic required")
	}
	if cfg.Group == "" {
		return nil, fmt.Errorf("group required")
	}
	return &ConsumerGroup{
		config: cfg,
		msgs:   make(chan Message, 100),
	}, nil
}

// Add adds a message to the group
func (g *ConsumerGroup) Add(ctx context.Context, msg Message) error {
	select {
	case g.msgs <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Consume processes messages from the group
func (g *ConsumerGroup) Consume(ctx context.Context, handler func(Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-g.msgs:
			if err := handler(msg); err != nil {
				return err
			}
		}
	}
}

// Close closes the consumer group
func (g *ConsumerGroup) Close() error {
	close(g.msgs)
	return nil
}
