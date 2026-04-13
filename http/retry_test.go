package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry_Success(t *testing.T) {
	fn := func() error { return nil }

	err := Retry(fn)

	assert.NoError(t, err)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return NewRetryableError(errors.New("temporary error"))
		}
		return nil
	}

	err := Retry(fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_MaxRetriesExceeded(t *testing.T) {
	fn := func() error {
		return NewRetryableError(errors.New("persistent error"))
	}

	err := Retry(fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestRetry_NonRetryableError(t *testing.T) {
	fn := func() error {
		return errors.New("non-retryable error")
	}

	err := Retry(fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable error")
}

func TestRetryWithContext_Cancel(t *testing.T) {
	fn := func() error {
		return NewRetryableError(errors.New("temporary error"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := RetryWithContext(ctx, fn, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
}

func TestRetryWithData_Success(t *testing.T) {
	fn := func() (string, error) {
		return "success", nil
	}

	result, err := RetryWithData(fn)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestRetryWithData_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	fn := func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, NewRetryableError(errors.New("temporary error"))
		}
		return 42, nil
	}

	result, err := RetryWithData(fn)

	assert.NoError(t, err)
	assert.Equal(t, 42, result)
	assert.Equal(t, 2, attempts)
}

func TestRetryWithConfig(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    5,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.5,
		Jitter:        false,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 4 {
			return NewRetryableError(errors.New("temporary error"))
		}
		return nil
	}

	start := time.Now()
	err := RetryWithConfig(fn, config)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 4, attempts)
	assert.Less(t, duration, 500*time.Millisecond) // Should complete quickly with small delays
}

func TestRetry_NoJitter(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    2,
		InitialDelay:  50 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return NewRetryableError(errors.New("temporary error"))
		}
		return nil
	}

	start := time.Now()
	err := RetryWithConfig(fn, config)
	duration := time.Since(start)

	assert.NoError(t, err)
	// First attempt succeeds at attempt 3, so we have delays: 50ms (between attempts 1-2)
	// Total should be around 50ms with some tolerance
	assert.Greater(t, duration, 40*time.Millisecond)
	assert.Less(t, duration, 100*time.Millisecond)
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		name       string
		attempt    int
		baseDelay  time.Duration
		maxDelay   time.Duration
		expected   time.Duration
	}{
		{
			name:      "first attempt",
			attempt:   0,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  0,
		},
		{
			name:      "second attempt",
			attempt:   1,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  100 * time.Millisecond,
		},
		{
			name:      "third attempt",
			attempt:   2,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  200 * time.Millisecond,
		},
		{
			name:      "fourth attempt",
			attempt:   3,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  400 * time.Millisecond,
		},
		{
			name:      "max delay capped",
			attempt:   10,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  1 * time.Second,
			expected:  1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := ExponentialBackoff(tt.attempt, tt.baseDelay, tt.maxDelay)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestLinearBackoff(t *testing.T) {
	tests := []struct {
		name      string
		attempt   int
		baseDelay time.Duration
		maxDelay  time.Duration
		expected  time.Duration
	}{
		{
			name:      "first attempt",
			attempt:   0,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  0,
		},
		{
			name:      "second attempt",
			attempt:   1,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  100 * time.Millisecond,
		},
		{
			name:      "third attempt",
			attempt:   2,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  200 * time.Millisecond,
		},
		{
			name:      "fourth attempt",
			attempt:   3,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  5 * time.Second,
			expected:  300 * time.Millisecond,
		},
		{
			name:      "max delay capped",
			attempt:   20,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  1 * time.Second,
			expected:  1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := LinearBackoff(tt.attempt, tt.baseDelay, tt.maxDelay)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestRetryableError(t *testing.T) {
	baseErr := errors.New("base error")
	retryableErr := NewRetryableError(baseErr)

	assert.True(t, IsRetryable(retryableErr))
	assert.False(t, IsRetryable(baseErr))
	assert.Equal(t, "base error", retryableErr.Error())
	assert.Equal(t, baseErr, errors.Unwrap(retryableErr))
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
	assert.Equal(t, 10*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.True(t, config.Jitter)
	assert.Equal(t, 0.1, config.JitterRange)
}
