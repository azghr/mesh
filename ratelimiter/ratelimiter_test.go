package ratelimiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleRateLimiter_Allow(t *testing.T) {
	limiter := NewSimpleRateLimiter(2, 60)

	// First request should be allowed
	allowed, err := limiter.Allow(nil, "test-key")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestSimpleRateLimiter_Denied(t *testing.T) {
	limiter := NewSimpleRateLimiter(2, 60)

	// Use two different keys - they have independent limits
	limiter.Allow(nil, "key-a")
	limiter.Allow(nil, "key-a")

	// Third on key-a should be denied
	allowed, err := limiter.Allow(nil, "key-a")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// But key-b should still be allowed
	allowed, err = limiter.Allow(nil, "key-b")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRedisRateLimiter_Interface(t *testing.T) {
	var _ RateLimiter = (*RedisRateLimiter)(nil)
}

func TestSimpleRateLimiter_Interface(t *testing.T) {
	var _ RateLimiter = (*SimpleRateLimiter)(nil)
}
