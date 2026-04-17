// Package eventbus provides dead letter handling for failed event processing.
//
// This package provides dead letter queue (DLQ) handling for events that fail
// processing after all retries are exhausted.
//
// # Overview
//
// The eventbus provides reliable message processing with:
//   - Automatic retry with exponential backoff
//   - Dead letter queue for failed messages
//   - Configurable retry behavior
//
// When an event handler fails:
//  1. Retries with exponential backoff (prevents thundering herd)
//  2. Delays: 100ms → 200ms → 400ms → 800ms → ... (capped at max)
//  3. On final failure, stores in DLQ
//  4. Optional handler processes dead letter
//
// # Basic Usage
//
// Create a reliable bus with default retry handling:
//
//	bus := eventbus.NewReliableBus(3, 100*time.Millisecond, 100)
//
// Or with full configuration:
//
//	bus := eventbus.NewReliableBusWithConfig(eventbus.RetryConfig{
//	    MaxRetries:    3,
//	    InitialDelay: 100*time.Millisecond,
//	    MaxDelay:     10*time.Second,
//	    BackoffFactor: 2.0,
//	    Jitter:       true,
//	    JitterRange: 0.3,
//	}, 100)
//
// Subscribe with automatic retries:
//
//	bus.SubscribeWithRetry("orders.created", func(payload any) {
//	    // Process order - retries with exponential backoff
//	})
//
// Subscribe with dead letter queue:
//
//	bus.SubscribeWithRetryAndDLQ("orders.created", func(payload any) {
//	    // Process order
//	})
//
// Handle dead letters:
//
//	bus.SetDLQHandler(func(dl eventbus.DeadLetter) {
//	    fmt.Printf("Failed after retries: %v\n", dl.Error)
//	    // Send to alerting, store in database, etc.
//	})
//
// Retry failed messages:
//
//	bus.RetryFailed(ctx)
//
// # Components
//
// DeadLetter represents a failed message:
//   - Topic: Original topic name
//   - Payload: Original message data
//   - Error: Last error message
//   - Attempts: Number of retry attempts
//   - Received: Time when added to DLQ
//   - LastTry: Time of last retry
//
// DeadLetterQueue manages failed messages with:
//   - Add: Store a dead letter
//   - GetAll: Retrieve all dead letters
//   - Clear: Remove all dead letters
//   - Size: Get queue size
//
// ReliableBus extends Bus with retry and DLQ support.
//
// # Best Practices
//
//   - Set retry counts based on expected failure types (3-5 for transient)
//   - Use exponential backoff for transient failures
//   - Monitor DLQ size in metrics
//   - Have a process to review and replay dead letters
//   - Set up alerting for high DLQ size (>100 is concerning)
//   - Implement dead letter reprocess periodically
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// DeadLetter represents a failed event for later retry or inspection.
type DeadLetter struct {
	Topic    string          `json:"topic"`
	Payload  json.RawMessage `json:"payload"`
	Error    string          `json:"error"`
	Attempts int             `json:"attempts"`
	Received time.Time       `json:"received"`
	LastTry  time.Time       `json:"last_try"`
}

// DeadLetterHandler is a function that processes dead letters.
type DeadLetterHandler func(DeadLetter)

// DeadLetterQueue manages failed events.
type DeadLetterQueue struct {
	mu      sync.RWMutex
	letters []DeadLetter
	maxSize int
	handler DeadLetterHandler
}

// NewDeadLetterQueue creates a new dead letter queue.
func NewDeadLetterQueue(maxSize int, handler DeadLetterHandler) *DeadLetterQueue {
	if maxSize == 0 {
		maxSize = 1000
	}
	return &DeadLetterQueue{
		letters: make([]DeadLetter, 0, maxSize),
		maxSize: maxSize,
		handler: handler,
	}
}

// Add stores a dead letter.
func (q *DeadLetterQueue) Add(dl DeadLetter) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Fire handler if configured
	if q.handler != nil {
		go q.handler(dl)
	}

	// Store if within capacity
	if len(q.letters) < q.maxSize {
		q.letters = append(q.letters, dl)
	}
}

// GetAll returns all dead letters.
func (q *DeadLetterQueue) GetAll() []DeadLetter {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]DeadLetter, len(q.letters))
	copy(result, q.letters)
	return result
}

// Clear removes all dead letters.
func (q *DeadLetterQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.letters = q.letters[:0]
}

// Size returns the number of dead letters.
func (q *DeadLetterQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.letters)
}

// RetryConfig configures retry behavior with exponential backoff.
type RetryConfig struct {
	MaxRetries    int           // Maximum retry attempts (default: 3)
	InitialDelay  time.Duration // Initial delay (default: 100ms)
	MaxDelay      time.Duration // Maximum delay cap (default: 10s)
	BackoffFactor float64       // Multiplier for each retry (default: 2.0)
	Jitter        bool          // Add randomness to delay (default: true)
	JitterRange   float64       // Jitter range 0.0-1.0 (default: 0.3)
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		JitterRange:   0.3,
	}
}

// calculateDelay computes delay for a given attempt with exponential backoff.
func (c *RetryConfig) calculateDelay(attempt int) time.Duration {
	delay := float64(c.InitialDelay) * math.Pow(c.BackoffFactor, float64(attempt))
	if delay > float64(c.MaxDelay) {
		delay = float64(c.MaxDelay)
	}

	if c.Jitter {
		jitter := delay * c.JitterRange * rand.Float64()
		delay -= jitter
	}

	return time.Duration(delay)
}

// ReliableBus extends Bus with retry and dead letter handling.
type ReliableBus struct {
	*Bus
	dlq *DeadLetterQueue
	cfg RetryConfig
}

// NewReliableBus creates a new reliable event bus with dead letter handling.
func NewReliableBus(maxRetries int, retryDelay time.Duration, dlqSize int) *ReliableBus {
	cfg := DefaultRetryConfig()
	cfg.MaxRetries = maxRetries
	if retryDelay > 0 {
		cfg.InitialDelay = retryDelay
	}

	bus := New()
	dlq := NewDeadLetterQueue(dlqSize, nil)

	return &ReliableBus{
		Bus: bus,
		dlq: dlq,
		cfg: cfg,
	}
}

// NewReliableBusWithConfig creates a reliable bus with full configuration.
func NewReliableBusWithConfig(cfg RetryConfig, dlqSize int) *ReliableBus {
	bus := New()
	dlq := NewDeadLetterQueue(dlqSize, nil)

	return &ReliableBus{
		Bus: bus,
		dlq: dlq,
		cfg: cfg,
	}
}

// SubscribeWithRetry registers a handler with automatic retry.
func (b *ReliableBus) SubscribeWithRetry(topic string, handler Handler) func() {
	wrappedHandler := func(payload any) {
		b.executeWithRetry(topic, payload, handler)
	}
	return b.Bus.Subscribe(topic, wrappedHandler)
}

// SubscribeWithRetryAndDLQ registers a handler with retry and dead letter queue.
func (b *ReliableBus) SubscribeWithRetryAndDLQ(topic string, handler Handler) func() {
	wrappedHandler := func(payload any) {
		b.executeWithRetry(topic, payload, handler)
	}
	return b.Bus.Subscribe(topic, wrappedHandler)
}

// executeWithRetry executes a handler with exponential backoff retry.
func (b *ReliableBus) executeWithRetry(topic string, payload any, handler Handler) {
	var lastErr error

	for attempt := 0; attempt <= b.cfg.MaxRetries; attempt++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					lastErr = fmt.Errorf("panic: %v", r)
				}
			}()
			handler(payload)
		}()

		if lastErr == nil {
			return // Success
		}

		if attempt < b.cfg.MaxRetries {
			delay := b.cfg.calculateDelay(attempt)
			time.Sleep(delay)
		}
	}

	// All retries exhausted - add to dead letter queue
	payloadBytes, _ := json.Marshal(payload)
	b.dlq.Add(DeadLetter{
		Topic:    topic,
		Payload:  payloadBytes,
		Error:    lastErr.Error(),
		Attempts: b.cfg.MaxRetries + 1,
		Received: time.Now(),
		LastTry:  time.Now(),
	})
}

// DLQ returns the dead letter queue.
func (b *ReliableBus) DLQ() *DeadLetterQueue {
	return b.dlq
}

// RetryFailed retries all dead letters.
func (b *ReliableBus) RetryFailed(ctx context.Context) {
	letters := b.dlq.GetAll()

	for _, dl := range letters {
		select {
		case <-ctx.Done():
			return
		default:
			b.Bus.Publish(dl.Topic, dl.Payload)
		}
	}

	b.dlq.Clear()
}

// SetDLQHandler sets a custom handler for dead letters.
func (b *ReliableBus) SetDLQHandler(handler DeadLetterHandler) {
	b.dlq.handler = handler
}
