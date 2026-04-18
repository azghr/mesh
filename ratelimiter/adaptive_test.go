package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAdaptiveLimiter(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil)
	assert.NotNil(t, limiter)
}

func TestNewAdaptiveLimiterWithOptions(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(500),
		WithMinRate(50),
		WithMaxRate(5000),
		WithTargetLatency(200*time.Millisecond),
		WithAdjustmentStep(50),
		WithAdaptiveWindow(2*time.Minute),
	)
	assert.NotNil(t, limiter)
}

func TestAdaptiveConfigDefaults(t *testing.T) {
	cfg := &AdaptiveConfig{}
	WithBaseRate(1000)(cfg)
	WithMinRate(100)(cfg)
	WithMaxRate(10000)(cfg)

	assert.Equal(t, float64(1000), cfg.BaseRate)
	assert.Equal(t, float64(100), cfg.MinRate)
	assert.Equal(t, float64(10000), cfg.MaxRate)
}

func TestAdaptiveConfigMinRate(t *testing.T) {
	cfg := &AdaptiveConfig{}
	WithMinRate(50)(cfg)
	WithMaxRate(200)(cfg)

	limiter := &AdaptiveLimiter{config: cfg}
	limiter.config = cfg

	assert.Equal(t, float64(50), cfg.MinRate)
}

func TestAdaptiveString(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil)
	str := limiter.String()
	assert.Contains(t, str, "AdaptiveLimiter")
}

func TestForceRate(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(1000),
		WithMinRate(100),
		WithMaxRate(5000),
	)

	limiter.ForceRate(500)
	assert.Equal(t, float64(500), limiter.getCurrentRate())
}

func TestForceRateBelowMin(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(1000),
		WithMinRate(100),
		WithMaxRate(5000),
	)

	limiter.ForceRate(50)
	assert.Equal(t, float64(100), limiter.getCurrentRate())
}

func TestForceRateAboveMax(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(1000),
		WithMinRate(100),
		WithMaxRate(5000),
	)

	limiter.ForceRate(6000)
	assert.Equal(t, float64(5000), limiter.getCurrentRate())
}

func TestSetConfig(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(1000),
		WithMinRate(100),
		WithMaxRate(5000),
	)

	limiter.SetConfig(&AdaptiveConfig{
		BaseRate: 2000,
		MinRate:  200,
		MaxRate:  10000,
	})

	assert.Equal(t, float64(1000), limiter.config.BaseRate)
	assert.Equal(t, float64(200), limiter.config.MinRate)
	assert.Equal(t, float64(10000), limiter.config.MaxRate)
}

func TestWithAdjustmentStep(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithBaseRate(1000),
		WithAdjustmentStep(200),
	)

	assert.Equal(t, float64(200), limiter.config.AdjustmentStep)
}

func TestWithTargetLatency(t *testing.T) {
	limiter := NewAdaptiveLimiter(nil,
		WithTargetLatency(150*time.Millisecond),
	)

	assert.Equal(t, 150*time.Millisecond, limiter.config.TargetLatency)
}
