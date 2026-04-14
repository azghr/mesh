// Package http provides HTTP client utilities with resilience patterns.
//
// This package includes circuit breaker and retry logic for making resilient
// HTTP calls that handle failures gracefully.
//
// # Circuit Breaker
//
// Prevents cascading failures by stopping requests to a failing service.
// States: Closed (normal) -> Open (failing) -> HalfOpen (testing recovery)
//
// Quick example:
//
//	cb := http.NewCircuitBreaker(nil)
//	err := cb.Execute(func() error {
//	    return callExternalService()
//	})
//	if err != nil {
//	    // service is unavailable
//	}
//
// # Retry
//
// Exponential backoff with jitter for transient failures.
// Use RetryableError to mark errors that should be retried.
package http

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int
	// ResetTimeout is how long to wait before attempting to close the circuit
	ResetTimeout time.Duration
	// HalfOpenMaxCalls is the number of calls allowed in HalfOpen state
	HalfOpenMaxCalls int
	// SuccessThreshold is the number of successful calls needed to close the circuit
	SuccessThreshold int
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu              sync.RWMutex
	config          *CircuitBreakerConfig
	state           CircuitState
	failureCount    int
	successCount    int
	halfOpenCalls   int
	lastFailureTime time.Time
	lastStateChange time.Time
	nextAttempt     time.Time
	onStateChange   func(from, to CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker with default configuration
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// SetStateChangeCallback sets a callback function to be called when the state changes
func (cb *CircuitBreaker) SetStateChangeCallback(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Execute runs the given function, applying circuit breaker logic
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check if we can execute
	if err := cb.canExecute(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.recordResult(err)

	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateOpen:
		// Check if we should transition to HalfOpen
		if now.After(cb.nextAttempt) {
			cb.setState(StateHalfOpen)
			cb.halfOpenCalls = 0
			return nil
		}
		return errors.New("circuit breaker is OPEN")

	case StateHalfOpen:
		// Limit the number of calls in HalfOpen state
		if cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
			return errors.New("circuit breaker is HALF_OPEN and at max calls")
		}
		cb.halfOpenCalls++

	case StateClosed:
		// Allow execution
	}

	return nil
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failure
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	cb.successCount = 0

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.MaxFailures {
			cb.setState(StateOpen)
			cb.nextAttempt = time.Now().Add(cb.config.ResetTimeout)
		}

	case StateHalfOpen:
		// Immediately open on failure in HalfOpen
		cb.setState(StateOpen)
		cb.nextAttempt = time.Now().Add(cb.config.ResetTimeout)
	}
}

// onSuccess handles a success
func (cb *CircuitBreaker) onSuccess() {
	cb.successCount++
	cb.failureCount = 0

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
		}

	case StateClosed:
		// Reset failure count on success in Closed state
		cb.failureCount = 0
	}
}

// setState changes the state and calls the callback if set.
// Callback is invoked as a goroutine to avoid deadlocks since setState
// may be called while holding the lock.
func (cb *CircuitBreaker) setState(newState CircuitState) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState
		cb.lastStateChange = time.Now()

		if cb.onStateChange != nil {
			go cb.onStateChange(oldState, newState)
		}
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the current failure count
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// SuccessCount returns the current success count
func (cb *CircuitBreaker) SuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successCount
}

// LastFailureTime returns the time of the last failure
func (cb *CircuitBreaker) LastFailureTime() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastFailureTime
}

// LastStateChange returns the time of the last state change
func (cb *CircuitBreaker) LastStateChange() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastStateChange
}

// NextAttempt returns the time when the next attempt will be allowed
func (cb *CircuitBreaker) NextAttempt() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.nextAttempt
}

// Reset resets the circuit breaker to Closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCalls = 0
	cb.lastFailureTime = time.Time{}
	cb.lastStateChange = time.Now()
	cb.nextAttempt = time.Time{}
}

// IsOpen returns true if the circuit breaker is in Open state
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}

// IsClosed returns true if the circuit breaker is in Closed state
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.State() == StateClosed
}

// IsHalfOpen returns true if the circuit breaker is in HalfOpen state
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.State() == StateHalfOpen
}

// Stats returns statistics about the circuit breaker
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state.String(),
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"half_open_calls":   cb.halfOpenCalls,
		"last_failure_time": cb.lastFailureTime,
		"last_state_change": cb.lastStateChange,
		"next_attempt":      cb.nextAttempt,
		"max_failures":      cb.config.MaxFailures,
		"reset_timeout":     cb.config.ResetTimeout.String(),
	}
}
