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
//   - Uses github.com/segmentio/kafka-go (pure Go, no CGO required)
//
// # Usage
//
//	// Producer
//	producer, err := kafka.NewProducer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	})
//	if err != nil {
//	    return err
//	}
//	defer producer.Close()
//
//	err := producer.Send(ctx, "key", map[string]string{
//	    "event": "user.created",
//	})
//
//	// Consumer
//	consumer, err := kafka.NewConsumer(kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    Topic:  "events",
//	    Group: "my-group",
//	})
//	if err != nil {
//	    return err
//	}
//	defer consumer.Close()
//
//	err := consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
//	    fmt.Printf("received: %s %s\n", key, string(value))
//	    return nil
//	})
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// Config configures Kafka producer/consumer
type Config struct {
	Brokers      []string
	Topic        string
	Group        string
	Balancer     kafka.Balancer
	PartitionKey func(key string) int
	Async        bool
	BatchSize    int
	BatchTimeout time.Duration
}

// Producer sends messages to Kafka
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Kafka producer
//
//	producer, err := kafka.NewProducer(kafka.Config{
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

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     cfg.Balancer,
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		Async:        cfg.Async,
	}

	return &Producer{writer: writer}, nil
}

// Send sends a message to Kafka
//
//	producer.Send(ctx, "key", map[string]string{
//	    "event": "user.created",
//	})
func (p *Producer) Send(ctx context.Context, key string, value interface{}) error {
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
	}

	return p.writer.WriteMessages(ctx, msg)
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
	reader *kafka.Reader
}

// NewConsumer creates a new Kafka consumer
//
//	consumer, err := kafka.NewConsumer(kafka.Config{
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

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.Group,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &Consumer{reader: reader}, nil
}

// Handler processes Kafka messages
type Handler func(ctx context.Context, key string, value []byte) error

// Consume starts consuming messages
//
//	err := consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
//	    fmt.Printf("received: %s %s\n", key, string(value))
//	    return nil
//	})
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}

			if err := handler(ctx, string(msg.Key), msg.Value); err != nil {
				// Log error but continue processing
				fmt.Printf("handler error: %v\n", err)
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				return err
			}
		}
	}
}

// ConsumePartition consumes from a specific partition
func (c *Consumer) ConsumePartition(ctx context.Context, partition int, handler Handler) error {
	// Use specific partition reader
	oldReader := c.reader
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:   oldReader.Config().Brokers,
		Topic:     oldReader.Config().Topic,
		GroupID:   oldReader.Config().GroupID,
		Partition: partition,
	})
	defer func() { c.reader = oldReader }()

	return c.Consume(ctx, handler)
}

// Close closes the consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// ConsumerGroup manages a group of consumers
type ConsumerGroup struct {
	readers []*kafka.Reader
	wg      sync.WaitGroup
}

// NewConsumerGroup creates a new consumer group with multiple consumers
func NewConsumerGroup(cfg Config, numConsumers int) (*ConsumerGroup, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic required")
	}
	if cfg.Group == "" {
		return nil, fmt.Errorf("group required")
	}

	readers := make([]*kafka.Reader, numConsumers)
	for i := 0; i < numConsumers; i++ {
		readers[i] = kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Brokers,
			Topic:   cfg.Topic,
			GroupID: fmt.Sprintf("%s-%d", cfg.Group, i),
		})
	}

	return &ConsumerGroup{readers: readers}, nil
}

// Consume processes messages from the group
func (g *ConsumerGroup) Consume(ctx context.Context, handler Handler) error {
	errCh := make(chan error, len(g.readers))

	for _, reader := range g.readers {
		g.wg.Add(1)
		go func(r *kafka.Reader) {
			defer g.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					msg, err := r.FetchMessage(ctx)
					if err != nil {
						if errors.Is(err, io.EOF) {
							return
						}
						errCh <- err
						return
					}

					if err := handler(ctx, string(msg.Key), msg.Value); err != nil {
						errCh <- err
						continue
					}

					if err := r.CommitMessages(ctx, msg); err != nil {
						errCh <- err
					}
				}
			}
		}(reader)
	}

	g.wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("consumer errors: %v", errs)
	}
	return nil
}

// Close closes the consumer group
func (g *ConsumerGroup) Close() error {
	for _, r := range g.readers {
		r.Close()
	}
	return nil
}
