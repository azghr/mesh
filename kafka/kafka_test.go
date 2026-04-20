package kafka

import (
	"context"
	"testing"
)

func TestNewProducer(t *testing.T) {
	p, err := NewProducer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
	})
	if err != nil {
		t.Fatalf("NewProducer error = %v", err)
	}
	if p == nil {
		t.Fatal("NewProducer returned nil")
	}
}

func TestNewProducer_MissingBrokers(t *testing.T) {
	_, err := NewProducer(Config{
		Brokers: []string{},
		Topic:   "test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewProducer_MissingTopic(t *testing.T) {
	_, err := NewProducer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProducer_Send(t *testing.T) {
	p, _ := NewProducer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
	})

	err := p.Send(context.Background(), "key", map[string]string{"event": "test"})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
}

func TestProducer_SendMany(t *testing.T) {
	p, _ := NewProducer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
	})

	messages := []Message{
		{Key: "key1", Value: map[string]string{"event": "test1"}},
		{Key: "key2", Value: map[string]string{"event": "test2"}},
	}

	err := p.SendMany(context.Background(), messages)
	if err != nil {
		t.Fatalf("SendMany error = %v", err)
	}
}

func TestNewConsumer(t *testing.T) {
	c, err := NewConsumer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		Group:   "test-group",
	})
	if err != nil {
		t.Fatalf("NewConsumer error = %v", err)
	}
	if c == nil {
		t.Fatal("NewConsumer returned nil")
	}
}

func TestNewConsumer_MissingGroup(t *testing.T) {
	_, err := NewConsumer(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		Group:   "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConsumerGroup(t *testing.T) {
	g, err := NewConsumerGroup(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		Group:   "test-group",
	})
	if err != nil {
		t.Fatalf("NewConsumerGroup error = %v", err)
	}
	if g == nil {
		t.Fatal("NewConsumerGroup returned nil")
	}
}

func TestConsumerGroup_Add(t *testing.T) {
	g, _ := NewConsumerGroup(Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		Group:   "test-group",
	})

	err := g.Add(context.Background(), Message{
		Key:   "key",
		Value: map[string]string{"event": "test"},
	})
	if err != nil {
		t.Fatalf("Add error = %v", err)
	}
}
