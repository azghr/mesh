// Package retry provides retry logic for operations with exponential backoff.
//
// This package implements retry patterns for database operations and external services:
// - Configurable max attempts
// - Multiple backoff strategies (constant, linear, exponential)
// - Jitter support for distributed systems
// - Context cancellation support
// - Error filtering
//
// Example:
//
//	err := retry.Do(ctx, func() error {
//	    return db.Tx(ctx, func(tx *sql.Tx) error {
//	        // operation
//	    })
//	}, retry.WithMaxAttempts(3), retry.WithBackoff(retry.Exponential))
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Backoff     BackoffType
	Jitter      bool
	RetryIf     func(error) bool
}

// BackoffType defines the backoff strategy.
type BackoffType int

const (
	Constant    BackoffType = iota // Constant delay
	Linear                         // Linear increase
	Exponential                    // Exponential backoff
)

// Option configures retry behavior.
type Option func(*Config)

// WithMaxAttempts sets the maximum number of attempts.
// Default: 3
func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		c.MaxAttempts = n
	}
}

// WithBaseDelay sets the base delay between retries.
// Default: 100ms
func WithBaseDelay(d time.Duration) Option {
	return func(c *Config) {
		c.BaseDelay = d
	}
}

// WithMaxDelay sets the maximum delay cap.
// Default: 30s
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithBackoff sets the backoff strategy.
// Default: Exponential
func WithBackoff(b BackoffType) Option {
	return func(c *Config) {
		c.Backoff = b
	}
}

// WithJitter enables jitter to prevent thundering herd.
// Uses FullJitter strategy.
func WithJitter() Option {
	return func(c *Config) {
		c.Jitter = true
	}
}

// WithRetryIf sets custom retry condition.
// Retries if the function returns true.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *Config) {
		c.RetryIf = fn
	}
}

// Default config
var defaultConfig = Config{
	MaxAttempts: 3,
	BaseDelay:   100 * time.Millisecond,
	MaxDelay:    30 * time.Second,
	Backoff:     Exponential,
	Jitter:      true,
	RetryIf:     DefaultRetryIf,
}

// DefaultRetryIf retries on transient errors.
func DefaultRetryIf(err error) bool {
	if err == nil {
		return false
	}
	// Don't retry on context cancellation
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// Do executes the function with retry logic.
// Returns the last error or nil if successful.
func Do(ctx context.Context, fn func() error, opts ...Option) error {
	cfg := defaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if we should retry
			if attempt >= cfg.MaxAttempts {
				break
			}
			if cfg.RetryIf != nil && !cfg.RetryIf(err) {
				break
			}

			// Check context
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Calculate delay
			delay := cfg.calculateDelay(attempt)

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

// DoWithResult executes the function with retry and returns a result.
func DoWithResult[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	var zero T

	cfg := defaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := fn()
		if err != nil {
			lastErr = err

			if attempt >= cfg.MaxAttempts {
				break
			}
			if cfg.RetryIf != nil && !cfg.RetryIf(err) {
				break
			}

			if ctx.Err() != nil {
				return zero, ctx.Err()
			}

			delay := cfg.calculateDelay(attempt)

			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return result, nil
	}
	return zero, lastErr
}

func (c *Config) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch c.Backoff {
	case Constant:
		delay = c.BaseDelay
	case Linear:
		delay = c.BaseDelay * time.Duration(attempt)
	case Exponential:
		delay = c.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	}

	if delay > c.MaxDelay {
		delay = c.MaxDelay
	}

	if c.Jitter {
		jitter := rand.Int63n(int64(delay))
		delay = time.Duration(jitter)
	}

	return delay
}

// Retry executes fn repeatedly until successful or max attempts.
// Simpler version without config options.
func Retry(ctx context.Context, maxAttempts int, fn func() error) error {
	return Do(ctx, fn, WithMaxAttempts(maxAttempts))
}

// Forever keeps retrying until successful (until context cancels).
func Forever(ctx context.Context, fn func() error) error {
	return Do(ctx, fn, WithMaxAttempts(math.MaxInt))
}
