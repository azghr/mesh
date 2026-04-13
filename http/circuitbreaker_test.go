package http

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_New(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.FailureCount())
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	successFn := func() error { return nil }

	err := cb.Execute(successFn)

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.FailureCount())
	assert.Equal(t, 1, cb.SuccessCount())
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: time.Second,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }

	// Execute failures until circuit opens
	for i := 0; i < 3; i++ {
		err := cb.Execute(failFn)
		assert.Error(t, err)
	}

	// Circuit should now be open
	assert.True(t, cb.IsOpen())
	assert.Equal(t, 3, cb.FailureCount())
}

func TestCircuitBreaker_Execute_Open(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  2,
		ResetTimeout: 100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(failFn)
	}

	assert.True(t, cb.IsOpen())

	// Next call should fail immediately
	err := cb.Execute(failFn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPEN")
}

func TestCircuitBreaker_HalfOpen_ToClosed(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:      2,
		ResetTimeout:     50 * time.Millisecond,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }
	successFn := func() error { return nil }

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(failFn)
	}
	assert.True(t, cb.IsOpen())

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// First success should transition to HalfOpen
	err := cb.Execute(successFn)
	assert.NoError(t, err)
	assert.True(t, cb.IsHalfOpen())

	// Second success should close the circuit
	err = cb.Execute(successFn)
	assert.NoError(t, err)
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_HalfOpen_Failure(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:      2,
		ResetTimeout:     50 * time.Millisecond,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }
	successFn := func() error { return nil }

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(failFn)
	}
	assert.True(t, cb.IsOpen())

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// First success should transition to HalfOpen
	err := cb.Execute(successFn)
	assert.NoError(t, err)
	assert.True(t, cb.IsHalfOpen())

	// Failure in HalfOpen should open circuit again
	err = cb.Execute(failFn)
	assert.Error(t, err)
	assert.True(t, cb.IsOpen())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures: 2,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(failFn)
	}
	assert.True(t, cb.IsOpen())

	// Reset
	cb.Reset()

	// Should be closed again
	assert.True(t, cb.IsClosed())
	assert.Equal(t, 0, cb.FailureCount())
	assert.Equal(t, 0, cb.SuccessCount())
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	failFn := func() error { return errors.New("failure") }

	stateChanges := make([]struct {
		from CircuitState
		to   CircuitState
	}, 0)

	cb.SetStateChangeCallback(func(from, to CircuitState) {
		stateChanges = append(stateChanges, struct {
			from CircuitState
			to   CircuitState
		}{from, to})
	})

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(failFn)
	}

	// Wait a bit for callback to execute
	time.Sleep(10 * time.Millisecond)

	require.Len(t, stateChanges, 1)
	assert.Equal(t, StateClosed, stateChanges[0].from)
	assert.Equal(t, StateOpen, stateChanges[0].to)
}

func TestCircuitBreaker_Stats(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  5,
		ResetTimeout: 60 * time.Second,
	}
	cb := NewCircuitBreaker(config)

	stats := cb.Stats()

	assert.Equal(t, "CLOSED", stats["state"])
	assert.Equal(t, 0, stats["failure_count"])
	assert.Equal(t, 0, stats["success_count"])
	assert.Equal(t, 5, stats["max_failures"])
	assert.Equal(t, "1m0s", stats["reset_timeout"])
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.Equal(t, 5, config.MaxFailures)
	assert.Equal(t, 60*time.Second, config.ResetTimeout)
	assert.Equal(t, 3, config.HalfOpenMaxCalls)
	assert.Equal(t, 2, config.SuccessThreshold)
}
