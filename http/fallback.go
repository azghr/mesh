// Package http provides HTTP client utilities with resilience patterns.
//
// This package includes circuit breaker, retry logic, and fallback support for
// making resilient HTTP calls that handle failures gracefully.
//
// # Fallback Support
//
// The fallback system provides degraded responses when services fail.
// Combined with the circuit breaker, this enables graceful degradation.
//
// # Basic Usage
//
// Create a fallback client with circuit breaker:
//
//	client := http.NewFallbackClient(nil)
//
// Register a fallback for a service:
//
//	client.SetFallback("users", http.FallbackResponse{
//	    Content:   []map[string]string{{"id": "1", "name": "Default User"}},
//	    StatusCode: 200,
//	    TTL:       5 * time.Minute,
//	})
//
// Execute with fallback:
//
//	result, err := client.Execute(ctx, "users", func() (interface{}, error) {
//	    return fetchUsers(ctx)
//	})
//	if err != nil {
//	    // Both circuit is open and no fallback - real error
//	}
//
// # Fallback Response
//
// FallbackResponse contains:
//   - Content: the fallback data to return
//   - StatusCode: HTTP status code to use
//   - TTL: how long to cache the fallback
//
// Cached fallbacks avoid repeated service calls during outages.
// TTL should be set based on data freshness requirements.
//
// # Best Practices
//
//   - Set appropriate TTLs based on data freshness needs
//   - Log fallback usage for monitoring
//   - Return partial data when possible (e.g., cached list)
//   - Use for read-only operations primarily
//   - Monitor circuit breaker state changes
package http

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FallbackResponse represents a cached fallback response.
type FallbackResponse struct {
	Content    interface{}   `json:"content"`
	StatusCode int           `json:"status_code"`
	TTL        time.Duration `json:"ttl"`
}

// FallbackRegistry stores fallback responses for services.
type FallbackRegistry struct {
	fallbacks map[string]FallbackResponse
	mu        sync.RWMutex
}

// NewFallbackRegistry creates a new fallback registry.
func NewFallbackRegistry() *FallbackRegistry {
	return &FallbackRegistry{
		fallbacks: make(map[string]FallbackResponse),
	}
}

// Register stores a fallback response for a service.
func (r *FallbackRegistry) Register(service string, fb FallbackResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbacks[service] = fb
}

// Unregister removes a fallback for a service.
func (r *FallbackRegistry) Unregister(service string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.fallbacks, service)
}

// Get retrieves a fallback response for a service.
func (r *FallbackRegistry) Get(service string) (FallbackResponse, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fb, ok := r.fallbacks[service]
	return fb, ok
}

// List returns all registered fallbacks.
func (r *FallbackRegistry) List() map[string]FallbackResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]FallbackResponse, len(r.fallbacks))
	for k, v := range r.fallbacks {
		result[k] = v
	}
	return result
}

// FallbackClient combines a circuit breaker with fallback support.
type FallbackClient struct {
	circuitBreaker *CircuitBreaker
	fallbacks      *FallbackRegistry
	cache          *SimpleCache
}

// SimpleCache provides simple in-memory caching for fallback responses.
type SimpleCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	Value   interface{}
	Expires time.Time
}

// NewSimpleCache creates a new simple cache.
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		items: make(map[string]cacheItem),
	}
}

// Get retrieves a value from cache.
func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.Expires) {
		return nil, false
	}
	return item.Value, true
}

// Set stores a value in cache with TTL.
func (c *SimpleCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		Value:   value,
		Expires: time.Now().Add(ttl),
	}
}

// Delete removes a value from cache.
func (c *SimpleCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// NewFallbackClient creates a new fallback-enabled client.
func NewFallbackClient(cb *CircuitBreaker) *FallbackClient {
	if cb == nil {
		cb = NewCircuitBreaker(nil)
	}
	return &FallbackClient{
		circuitBreaker: cb,
		fallbacks:      NewFallbackRegistry(),
		cache:          NewSimpleCache(),
	}
}

// SetFallback registers a fallback response for a service.
func (c *FallbackClient) SetFallback(service string, fb FallbackResponse) {
	c.fallbacks.Register(service, fb)
}

// Execute runs fn with circuit breaker protection and fallback support.
// If the circuit is open, returns the fallback response instead of an error.
func (c *FallbackClient) Execute(ctx context.Context, service string, fn func() (interface{}, error)) (interface{}, error) {
	err := c.circuitBreaker.Execute(func() error {
		_, err := fn()
		return err
	})

	// If failed and circuit is open, try fallback
	if err != nil {
		if c.circuitBreaker.State() == StateOpen {
			if fb, ok := c.fallbacks.Get(service); ok {
				return c.serveFallback(ctx, service, fb)
			}
		}
		return nil, err
	}

	return fn()
}

// ExecuteWithFallback runs fn and returns fallback on circuit open.
// Unlike Execute, this never returns the circuit breaker error.
func (c *FallbackClient) ExecuteWithFallback(ctx context.Context, service string, fn func() (interface{}, error)) (interface{}, error) {
	err := c.circuitBreaker.Execute(func() error {
		_, err := fn()
		return err
	})

	// Always try fallback on any error if circuit was open
	if err != nil || c.circuitBreaker.State() == StateOpen {
		if fb, ok := c.fallbacks.Get(service); ok {
			return c.serveFallback(ctx, service, fb)
		}
	}

	if err != nil {
		return nil, err
	}
	return fn()
}

// serveFallback returns a cached fallback response.
func (c *FallbackClient) serveFallback(ctx context.Context, service string, fb FallbackResponse) (interface{}, error) {
	cacheKey := fmt.Sprintf("fallback:%s", service)

	// Try cache first
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	// Use configured fallback
	if fb.TTL > 0 {
		c.cache.Set(cacheKey, fb.Content, fb.TTL)
	}

	return fb.Content, nil
}

// CircuitBreaker returns the underlying circuit breaker.
func (c *FallbackClient) CircuitBreaker() *CircuitBreaker {
	return c.circuitBreaker
}

// Fallbacks returns the fallback registry.
func (c *FallbackClient) Fallbacks() *FallbackRegistry {
	return c.fallbacks
}
