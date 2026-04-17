// Package http provides distributed circuit breaker with Redis-backed state.
//
// This package provides circuit breaker state that is shared across
// multiple instances using Redis for consistent failure tracking.
//
// # Overview
//
// The distributed circuit breaker shares failure state across multiple
// service instances using Redis. This ensures consistent behavior
// in horizontally scaled deployments.
//
// When circuit should open:
//   - Local failures count toward threshold
//   - Distributed failures from Redis also count
//   - Cross-instance failure rate is calculated
//
// # Basic Usage
//
// Create a distributed circuit breaker:
//
//	config := http.DefaultDistributedConfig(redisClient)
//	config.Threshold = 0.5 // 50% failure rate
//
//	dcb := http.NewDistributedCircuitState(nil, config)
//
// Use with fallback client:
//
//	client := http.NewDistributedClient(nil, config)
//	client.SetFallback("users", http.FallbackResponse{
//	    Content:   cachedUsers,
//	    StatusCode: 200,
//	    TTL:       5*time.Minute,
//	})
//
//	result, err := client.Execute(ctx, "users", fetchUsers)
//
// # Configuration
//
//	DistributedConfig{
//	    Redis:       redisClient,    // Required
//	    KeyPrefix:   "circuit",     // Redis key prefix
//	    Threshold:   0.5,          // 50% failure rate opens
//	    StateTTL:    5*time.Minute, // How long to keep state
//	}
//
// # Best Practices
//
//   - Use same threshold across all instances
//   - Set appropriate TTL based on recovery time
//   - Monitor distributed state via GetDistributedState
//   - Use with fallback for graceful degradation
package http

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultStateTTL is how long to keep state in Redis
	DefaultStateTTL = 5 * time.Minute
	// DefaultThreshold is failures/total requests to open circuit
	DefaultThreshold = float64(0.5)
)

// DistributedConfig configures distributed circuit breaker.
type DistributedConfig struct {
	Redis     *redis.Client
	KeyPrefix string
	Threshold float64       // Failure rate to open (0.0-1.0)
	StateTTL  time.Duration // How long to keep state
}

// DefaultDistributedConfig returns sensible defaults.
func DefaultDistributedConfig(redisClient *redis.Client) DistributedConfig {
	return DistributedConfig{
		Redis:     redisClient,
		KeyPrefix: "circuit_breaker",
		Threshold: DefaultThreshold,
		StateTTL:  DefaultStateTTL,
	}
}

// DistributedCircuitState tracks state in Redis for cross-instance sharing.
type DistributedCircuitState struct {
	local  *CircuitBreaker
	redis  *redis.Client
	key    string
	config DistributedConfig
	mu     sync.RWMutex
}

// NewDistributedCircuitState creates a distributed state tracker.
func NewDistributedCircuitState(cb *CircuitBreaker, config DistributedConfig) *DistributedCircuitState {
	if cb == nil {
		cb = NewCircuitBreaker(nil)
	}
	if config.Redis == nil {
		panic("redis client is required for distributed circuit breaker")
	}

	return &DistributedCircuitState{
		local:  cb,
		redis:  config.Redis,
		key:    config.KeyPrefix,
		config: config,
	}
}

// stateKey returns the Redis key for a host.
func (d *DistributedCircuitState) stateKey(host string) string {
	return fmt.Sprintf("%s:%s", d.key, host)
}

// RecordSuccess records a successful call.
func (d *DistributedCircuitState) RecordSuccess(ctx context.Context, host string) {
	// Record locally
	d.local.Execute(func() error { return nil })

	// Record in Redis
	pipe := d.redis.Pipeline()
	pipe.HIncrBy(ctx, d.stateKey(host), "successes", 1)
	pipe.HSet(ctx, d.stateKey(host), "last_success", strconv.FormatInt(time.Now().Unix(), 10))
	pipe.Expire(ctx, d.stateKey(host), d.config.StateTTL)
	pipe.Exec(ctx)
}

// RecordFailure records a failed call.
func (d *DistributedCircuitState) RecordFailure(ctx context.Context, host string) {
	// Record locally
	d.local.Execute(func() error { return fmt.Errorf("failure") })

	// Record in Redis
	pipe := d.redis.Pipeline()
	pipe.HIncrBy(ctx, d.stateKey(host), "failures", 1)
	pipe.HSet(ctx, d.stateKey(host), "last_failure", strconv.FormatInt(time.Now().Unix(), 10))
	pipe.Expire(ctx, d.stateKey(host), d.config.StateTTL)
	pipe.Exec(ctx)
}

// IsOpen checks if circuit should be open based on distributed state.
func (d *DistributedCircuitState) IsOpen(ctx context.Context, host string) bool {
	// Check local state first
	if d.local.State() == StateOpen {
		return true
	}

	// Check distributed state
	state, err := d.redis.HGetAll(ctx, d.stateKey(host)).Result()
	if err != nil || len(state) == 0 {
		return false
	}

	successes, _ := strconv.ParseInt(state["successes"], 10, 64)
	failures, _ := strconv.ParseInt(state["failures"], 10, 64)
	total := successes + failures

	// Not enough data - use local decision
	if total < 10 {
		return d.local.State() == StateOpen
	}

	failureRate := float64(failures) / float64(total)

	// If distributed failure rate is high, open the circuit
	// But only override if local is closed (don't close locally open)
	if failureRate >= d.config.Threshold && d.local.State() == StateClosed {
		return true
	}

	return false
}

// GetDistributedState returns current distributed state for monitoring.
func (d *DistributedCircuitState) GetDistributedState(ctx context.Context, host string) (map[string]string, error) {
	return d.redis.HGetAll(ctx, d.stateKey(host)).Result()
}

// DistributedClient wraps a client with distributed circuit breaker state.
type DistributedClient struct {
	inner     *FallbackClient
	distState *DistributedCircuitState
}

// NewDistributedClient creates a client with distributed state.
func NewDistributedClient(cb *CircuitBreaker, config DistributedConfig) *DistributedClient {
	distState := NewDistributedCircuitState(cb, config)
	fallbackClient := NewFallbackClient(cb)

	return &DistributedClient{
		inner:     fallbackClient,
		distState: distState,
	}
}

// Execute runs the function with distributed circuit breaker tracking.
func (c *DistributedClient) Execute(ctx context.Context, service string, fn func() (interface{}, error)) (interface{}, error) {
	// First check if circuit is open
	if c.distState.IsOpen(ctx, service) {
		// Try fallback
		if fb, ok := c.inner.fallbacks.Get(service); ok {
			return c.inner.serveFallback(ctx, service, fb)
		}
	}

	// Execute function
	result, err := fn()

	// Record result
	if err != nil {
		c.distState.RecordFailure(ctx, service)
	} else {
		c.distState.RecordSuccess(ctx, service)
	}

	// Check if should use fallback after state update
	if err != nil && c.distState.IsOpen(ctx, service) {
		if fb, ok := c.inner.fallbacks.Get(service); ok {
			return c.inner.serveFallback(ctx, service, fb)
		}
	}

	return result, err
}

// SetFallback registers a fallback for a service.
func (c *DistributedClient) SetFallback(service string, fb FallbackResponse) {
	c.inner.SetFallback(service, fb)
}

// CircuitBreaker returns the underlying circuit breaker.
func (c *DistributedClient) CircuitBreaker() *CircuitBreaker {
	return c.distState.local
}
