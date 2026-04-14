// Package eventbus provides a simple pub/sub event bus for internal service communication.
//
// This package implements a lightweight publish/subscribe pattern for handling
// internal events within a service.
//
// Example:
//
//	bus := eventbus.New()
//	bus.Subscribe("user.created", func(payload any) {
//	    fmt.Println("User created:", payload)
//	})
//	bus.Publish("user.created", map[string]string{"id": "123", "name": "John"})
package eventbus

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Handler is a function that handles an event
type Handler func(payload any)

// Bus is the event bus that handles pub/sub
type Bus struct {
	mu       sync.RWMutex
	subs     map[string][]Handler
	handlers map[string]map[uint64]Handler // keyed by subscription ID
	nextID   uint64
}

// New creates a new event bus
func New() *Bus {
	return &Bus{
		subs:     make(map[string][]Handler),
		handlers: make(map[string]map[uint64]Handler),
	}
}

// Subscribe registers a handler for an event topic.
// Returns an unsubscribe function.
func (b *Bus) Subscribe(topic string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	if b.handlers == nil {
		b.handlers = make(map[string]map[uint64]Handler)
	}
	if b.handlers[topic] == nil {
		b.handlers[topic] = make(map[uint64]Handler)
	}

	b.handlers[topic][id] = handler

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.handlers[topic], id)
	}
}

// Publish sends an event to all subscribers of the topic
func (b *Bus) Publish(topic string, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers, ok := b.handlers[topic]
	if !ok {
		return
	}

	for _, handler := range handlers {
		go handler(payload)
	}
}

// PublishSync sends an event synchronously to all subscribers
func (b *Bus) PublishSync(topic string, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers, ok := b.handlers[topic]
	if !ok {
		return
	}

	for _, handler := range handlers {
		handler(payload)
	}
}

// SubscribeOnce registers a handler that will be called only once
func (b *Bus) SubscribeOnce(topic string, handler Handler) {
	var onceCalled atomic.Bool
	onceHandler := func(payload any) {
		if !onceCalled.CompareAndSwap(false, true) {
			return
		}
		handler(payload)
	}
	b.Subscribe(topic, onceHandler)
}

// Unsubscribe removes a specific handler from a topic
func (b *Bus) Unsubscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers, ok := b.handlers[topic]
	if !ok {
		return
	}

	for id, h := range handlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			delete(handlers, id)
		}
	}
}

// Clear removes all handlers for a topic
func (b *Bus) Clear(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.handlers, topic)
}

// ClearAll removes all handlers
func (b *Bus) ClearAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers = make(map[string]map[uint64]Handler)
}

// Topics returns list of subscribed topics
func (b *Bus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.handlers))
	for topic := range b.handlers {
		topics = append(topics, topic)
	}
	return topics
}

// HandlerCount returns the number of handlers for a topic
func (b *Bus) HandlerCount(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers, ok := b.handlers[topic]
	if !ok {
		return 0
	}
	return len(handlers)
}

// JSONSubscribe subscribes and automatically unmarshals JSON payload
func (b *Bus) JSONSubscribe(topic string, handler func(data any) error) {
	b.Subscribe(topic, func(payload any) {
		var data any
		if bytes, ok := payload.([]byte); ok {
			json.Unmarshal(bytes, &data)
		} else {
			data = payload
		}
		handler(data)
	})
}

// GlobalBus is a package-level event bus for convenience
var globalBus = New()

// GlobalSubscribe subscribes to the global event bus
func GlobalSubscribe(topic string, handler Handler) func() {
	return globalBus.Subscribe(topic, handler)
}

// GlobalPublish publishes to the global event bus
func GlobalPublish(topic string, payload any) {
	globalBus.Publish(topic, payload)
}

// GlobalPublishSync publishes synchronously to the global event bus
func GlobalPublishSync(topic string, payload any) {
	globalBus.PublishSync(topic, payload)
}

// GlobalClear clears the global event bus
func GlobalClear() {
	globalBus.ClearAll()
}
