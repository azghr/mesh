// Package eventbus provides dead letter handling for failed event processing.
//
// This package provides dead letter queue (DLQ) handling for events that fail
// processing after all retries are exhausted.
//
// # Overview
//
// The eventbus provides reliable message processing with:
//   - Automatic retry on failure
//   - Dead letter queue for failed messages
//   - Configurable retry count and delay
//
// When an event handler fails:
//  1. Retries up to maxRetries times
//  2. Waits retryDelay between attempts
//  3. On final failure, stores in DLQ
//  4. Optional handler processes dead letter
//
// # Basic Usage
//
// Create a reliable bus with retry handling:
//
//	bus := eventbus.NewReliableBus(3, 100*time.Millisecond, 100)
//
// Subscribe with automatic retries:
//
//	bus.SubscribeWithRetry("orders.created", func(payload any) {
//	    // Process order - will retry 3 times on failure
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

// ReliableBus extends Bus with retry and dead letter handling.
type ReliableBus struct {
	*Bus
	dlq        *DeadLetterQueue
	maxRetries int
	retryDelay time.Duration
}

// NewReliableBus creates a new reliable event bus with dead letter handling.
func NewReliableBus(maxRetries int, retryDelay time.Duration, dlqSize int) *ReliableBus {
	bus := New()
	dlq := NewDeadLetterQueue(dlqSize, nil)

	return &ReliableBus{
		Bus:        bus,
		dlq:        dlq,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
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

// executeWithRetry executes a handler with retry logic.
func (b *ReliableBus) executeWithRetry(topic string, payload any, handler Handler) {
	var lastErr error

	for attempt := 0; attempt <= b.maxRetries; attempt++ {
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

		if attempt < b.maxRetries {
			time.Sleep(b.retryDelay)
		}
	}

	// All retries exhausted - add to dead letter queue
	payloadBytes, _ := json.Marshal(payload)
	b.dlq.Add(DeadLetter{
		Topic:    topic,
		Payload:  payloadBytes,
		Error:    lastErr.Error(),
		Attempts: b.maxRetries + 1,
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
