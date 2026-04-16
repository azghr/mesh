package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/azghr/mesh/ratelimiter"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLimiter struct {
	allowed    bool
	count      int
	remaining  int
	err        error
	allowCalls int
}

func (m *mockLimiter) Allow(ctx context.Context, key string) (bool, error) {
	m.allowCalls++
	return m.allowed, m.err
}

func (m *mockLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	m.allowCalls++
	return m.allowed, m.err
}

func (m *mockLimiter) Reset(ctx context.Context, key string) error {
	return nil
}

func (m *mockLimiter) GetLimit(ctx context.Context, key string) (int, int, error) {
	return m.count, m.remaining, nil
}

func TestLimitMiddleware_Allows(t *testing.T) {
	app := fiber.New()

	limiter := &mockLimiter{allowed: true, count: 100, remaining: 99}
	mw := NewLimit(limiter)
	mw.KeyFunc(func(c *fiber.Ctx) string {
		return "test:key"
	})

	app.Use(mw.Handler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	metrics := mw.Metrics()
	assert.Equal(t, int64(1), metrics.AllowedTotal())
	assert.Equal(t, int64(0), metrics.RejectedTotal())
}

func TestLimitMiddleware_Rejects(t *testing.T) {
	app := fiber.New()

	limiter := &mockLimiter{allowed: false, count: 100, remaining: 0}
	mw := NewLimit(limiter)

	app.Use(mw.Handler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode)

	metrics := mw.Metrics()
	assert.Equal(t, int64(0), metrics.AllowedTotal())
	assert.Equal(t, int64(1), metrics.RejectedTotal())
}

func TestLimitMiddleware_Headers(t *testing.T) {
	app := fiber.New()

	limiter := &mockLimiter{allowed: true, count: 100, remaining: 50}
	mw := NewLimit(limiter)

	app.Use(mw.Handler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
}

func TestLimitMiddleware_KeyFunctions(t *testing.T) {
	app := fiber.New()

	var capturedKey string
	limiter := &mockLimiter{allowed: true}
	mw := NewLimit(limiter)
	mw.KeyFunc(func(c *fiber.Ctx) string {
		key := "custom:" + c.Path()
		capturedKey = key
		return key
	})

	app.Use(mw.Handler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req)

	assert.Equal(t, "custom:/test", capturedKey)
}

func TestLimitMiddleware_Integration(t *testing.T) {
	app := fiber.New()

	limiter := ratelimiter.NewSimpleRateLimiter(5, time.Minute)
	mw := NewLimit(limiter)

	app.Use(mw.Handler())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "Request %d should succeed", i+1)
	}

	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode, "6th request should be rejected")

	metrics := mw.Metrics()
	assert.Equal(t, int64(5), metrics.AllowedTotal())
	assert.Equal(t, int64(1), metrics.RejectedTotal())
}

func TestLimitMetrics_Reset(t *testing.T) {
	metrics := NewLimitMetrics()

	metrics.Allowed = 10
	metrics.Rejected = 5

	metrics.Reset()

	assert.Equal(t, int64(0), metrics.AllowedTotal())
	assert.Equal(t, int64(0), metrics.RejectedTotal())
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a, b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{"a,b,c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"", false},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"256.1.1.1", true},
		{"abc", false},
		{"192.168.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidIP(tt.ip))
		})
	}
}
