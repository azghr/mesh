// Package graphql provides enhanced GraphQL schema and resolver helpers.
//
// This package adds subscriptions, directives, and resolver helpers.
package graphql

import (
	"context"
	"sync"
	"time"
)

// Subscription types
type (
	// EventStream represents a stream of subscription events
	EventStream struct {
		Channel string
		Events  chan any
		Done    chan struct{}
	}

	// SubscriptionManager manages subscriptions
	SubscriptionManager struct {
		mu            sync.RWMutex
		subscriptions map[string]map[*EventStream]bool
		resolvers     map[string]func(ctx context.Context, channel string) (*EventStream, error)
	}

	// ResolverBuilder helps build resolvers
	ResolverBuilder struct {
		resolvers map[string]func(ctx context.Context, source any, args map[string]any) (any, error)
		mu        sync.RWMutex
	}
)

// NewEventStream creates a new event stream
func NewEventStream(channel string) *EventStream {
	return &EventStream{
		Channel: channel,
		Events:  make(chan any, 10),
		Done:    make(chan struct{}),
	}
}

// Send sends an event to subscribers
func (e *EventStream) Send(data any) {
	select {
	case e.Events <- data:
	case <-e.Done:
	case <-time.After(time.Millisecond):
	}
}

// Close closes the event stream
func (e *EventStream) Close() {
	close(e.Events)
	close(e.Done)
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]map[*EventStream]bool),
		resolvers:     make(map[string]func(ctx context.Context, channel string) (*EventStream, error)),
	}
}

// Subscribe registers a subscription
func (sm *SubscriptionManager) Subscribe(ctx context.Context, channel string, stream *EventStream) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.subscriptions[channel] == nil {
		sm.subscriptions[channel] = make(map[*EventStream]bool)
	}
	sm.subscriptions[channel][stream] = true
}

// Unsubscribe removes a subscription
func (sm *SubscriptionManager) Unsubscribe(channel string, stream *EventStream) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if streams := sm.subscriptions[channel]; streams != nil {
		delete(streams, stream)
		stream.Close()
	}
}

// Publish publishes an event to all subscribers
func (sm *SubscriptionManager) Publish(channel string, data any) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if streams := sm.subscriptions[channel]; streams != nil {
		for stream := range streams {
			stream.Send(data)
		}
	}
}

// RegisterResolver registers a subscription resolver
func (sm *SubscriptionManager) RegisterResolver(channel string, resolver func(ctx context.Context, channel string) (*EventStream, error)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resolvers[channel] = resolver
}

// ResolverBuilder methods
func NewResolverBuilder() *ResolverBuilder {
	return &ResolverBuilder{
		resolvers: make(map[string]func(ctx context.Context, source any, args map[string]any) (any, error)),
	}
}

func (rb *ResolverBuilder) Add(name string, resolve func(ctx context.Context, source any, args map[string]any) (any, error)) *ResolverBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.resolvers[name] = resolve
	return rb
}

func (rb *ResolverBuilder) Build() map[string]func(ctx context.Context, source any, args map[string]any) (any, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	result := make(map[string]func(ctx context.Context, source any, args map[string]any) (any, error))
	for k, v := range rb.resolvers {
		result[k] = v
	}
	return result
}

// Type coercion helpers
func AsString(val any) (string, bool) {
	if v, ok := val.(string); ok {
		return v, true
	}
	return "", false
}

func AsInt(val any) (int, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}

func AsFloat(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

func AsBool(val any) (bool, bool) {
	if v, ok := val.(bool); ok {
		return v, true
	}
	return false, false
}

func AsSlice(val any) ([]any, bool) {
	if v, ok := val.([]any); ok {
		return v, true
	}
	return nil, false
}

func AsMap(val any) (map[string]any, bool) {
	if v, ok := val.(map[string]any); ok {
		return v, true
	}
	return nil, false
}

// Built-in Directives
type Directive struct {
	Name        string
	Description string
	Args        []InputValue
}

var (
	DirectiveInclude = Directive{
		Name:        "include",
		Description: "Include field only if argument is true",
		Args: []InputValue{
			{Name: "if", Type: "Boolean!"},
		},
	}
	DirectiveSkip = Directive{
		Name:        "skip",
		Description: "Skip field only if argument is true",
		Args: []InputValue{
			{Name: "if", Type: "Boolean!"},
		},
	}
	DirectiveDeprecated = Directive{
		Name:        "deprecated",
		Description: "Mark field as deprecated",
		Args: []InputValue{
			{Name: "reason", Type: "String"},
		},
	}
)

// ResolveDirective resolves directive
func (d *Directive) ResolveDirective(ctx context.Context, args map[string]any) bool {
	switch d.Name {
	case "include":
		if args["if"] == true {
			return true
		}
		return false
	case "skip":
		if args["if"] == true {
			return false
		}
		return true
	}
	return true
}
