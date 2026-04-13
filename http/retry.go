package http

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// BackoffFactor is the multiplier for exponential backoff
	BackoffFactor float64
	// Jitter enables jitter to prevent thundering herd
	Jitter bool
	// JitterRange is the range of jitter (0.0 to 1.0)
	JitterRange float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		JitterRange:   0.1,
	}
}

// RetryableError is an error that can be retried
type RetryableError struct {
	Err error
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// Error returns the error message
func (e *RetryableError) Error() string {
	if e.Err == nil {
		return "retryable error"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	return errors.As(err, &retryableErr)
}

// Retry executes a function with retry logic
func Retry(fn func() error) error {
	return RetryWithContext(context.Background(), fn, nil)
}

// RetryWithConfig executes a function with custom retry configuration
func RetryWithConfig(fn func() error, config *RetryConfig) error {
	return RetryWithContext(context.Background(), fn, config)
}

// RetryWithContext executes a function with retry logic and context support
func RetryWithContext(ctx context.Context, fn func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before executing
		if ctx.Err() != nil {
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't wait after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with backoff and jitter
		waitDelay := calculateDelay(delay, config)

		// Wait before retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during wait: %w", ctx.Err())
		case <-time.After(waitDelay):
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateDelay calculates the delay with optional jitter
func calculateDelay(baseDelay time.Duration, config *RetryConfig) time.Duration {
	if !config.Jitter {
		return baseDelay
	}

	// Add jitter to prevent thundering herd
	jitterRange := float64(baseDelay) * config.JitterRange
	minDelay := float64(baseDelay) - jitterRange
	maxDelay := float64(baseDelay) + jitterRange

	// Ensure we don't go negative
	if minDelay < 0 {
		minDelay = 0
	}

	// Add random jitter
	jitter := rand.Float64() * (maxDelay - minDelay)
	return time.Duration(minDelay + jitter)
}

// RetryWithData executes a function that returns data with retry logic
func RetryWithData[T any](fn func() (T, error)) (T, error) {
	return RetryWithDataWithContext(context.Background(), fn, nil)
}

// RetryWithDataWithConfig executes a function that returns data with custom retry configuration
func RetryWithDataWithConfig[T any](fn func() (T, error), config *RetryConfig) (T, error) {
	return RetryWithDataWithContext(context.Background(), fn, config)
}

// RetryWithDataWithContext executes a function that returns data with retry logic and context support
func RetryWithDataWithContext[T any](ctx context.Context, fn func() (T, error), config *RetryConfig) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var result T
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before executing
		if ctx.Err() != nil {
			return result, fmt.Errorf("retry cancelled: %w", ctx.Err())
		}

		// Execute the function
		data, err := fn()
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return result, fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't wait after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with backoff and jitter
		waitDelay := calculateDelay(delay, config)

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("retry cancelled during wait: %w", ctx.Err())
		case <-time.After(waitDelay):
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return result, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ExponentialBackoff calculates the delay for a given attempt using exponential backoff
func ExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt == 0 {
		return 0
	}

	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// LinearBackoff calculates the delay for a given attempt using linear backoff
func LinearBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt == 0 {
		return 0
	}

	delay := baseDelay * time.Duration(attempt)
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// IsNetworkError checks if an error is a network error that should be retried
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Add specific network error checks here
	// For now, we'll rely on explicit retryable errors
	return false
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Add specific timeout error checks here
	// For now, we'll rely on explicit retryable errors
	return false
}
