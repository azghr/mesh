// Package middleware provides HTTP middleware for Fiber applications.
package middleware

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/azghr/mesh/ratelimiter"
	"github.com/gofiber/fiber/v2"
)

// LimitConfig configures the rate limit middleware.
type LimitConfig struct {
	// Limiter is the rate limiter to use (Redis or in-memory).
	Limiter ratelimiter.RateLimiter
	// KeyFunc determines the key for rate limiting (IP, user, endpoint, etc.).
	KeyFunc func(*fiber.Ctx) string
	// SkipFailed allows requests that return errors to bypass rate limiting.
	SkipFailed bool
}

// LimitMetrics tracks rate limiting statistics.
type LimitMetrics struct {
	Allowed  int64
	Rejected int64
}

// NewLimitMetrics creates a new metrics counter.
func NewLimitMetrics() *LimitMetrics {
	return &LimitMetrics{}
}

// AllowedTotal returns the total number of allowed requests.
func (m *LimitMetrics) AllowedTotal() int64 {
	return atomic.LoadInt64(&m.Allowed)
}

// RejectedTotal returns the total number of rejected requests.
func (m *LimitMetrics) RejectedTotal() int64 {
	return atomic.LoadInt64(&m.Rejected)
}

// Reset clears all metrics.
func (m *LimitMetrics) Reset() {
	atomic.StoreInt64(&m.Allowed, 0)
	atomic.StoreInt64(&m.Rejected, 0)
}

// LimitMiddleware provides rate limiting for Fiber applications.
// It wraps a RateLimiter and applies it to incoming requests.
//
// Example:
//
//	limiter := ratelimiter.NewRedisRateLimiter(client, 100, time.Minute)
//	mw := middleware.NewLimit(limiter)
//	app.Use(mw.Handler())
type LimitMiddleware struct {
	config  LimitConfig
	metrics *LimitMetrics
}

// NewLimit creates a new rate limit middleware.
//
//	limiter := ratelimiter.NewRedisRateLimiter(redisClient, 100, time.Minute)
//	mw := middleware.NewLimit(limiter)
func NewLimit(limiter ratelimiter.RateLimiter) *LimitMiddleware {
	return &LimitMiddleware{
		config: LimitConfig{
			Limiter: limiter,
			KeyFunc: DefaultKeyFunc,
		},
		metrics: NewLimitMetrics(),
	}
}

// DefaultKeyFunc extracts the client IP as the rate limit key.
// It respects X-Forwarded-For headers from trusted proxies.
func DefaultKeyFunc(c *fiber.Ctx) string {
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		for _, ip := range splitAndTrim(xff) {
			if isValidIP(ip) {
				return "ip:" + ip
			}
		}
	}
	return "ip:" + c.IP()
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	var result []string
	for _, part := range split(s, ",") {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// split splits a string by separator without using strings package.
func split(s string, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// isValidIP checks if a string is a valid IPv4 address.
func isValidIP(s string) bool {
	if len(s) == 0 {
		return false
	}
	dots := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			dots++
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return dots == 3
}

// UserKeyFunc uses the user ID from context as the rate limit key.
// Falls back to IP if no user ID is set.
func UserKeyFunc(c *fiber.Ctx) string {
	userID := c.Locals("user_id")
	if userID == nil {
		return DefaultKeyFunc(c)
	}
	return "user:" + toString(userID)
}

// toString converts an interface{} to string.
func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return ""
	}
}

// EndpointKeyFunc uses the HTTP method and path as the rate limit key.
// Format: "endpoint:GET:/users"
func EndpointKeyFunc(c *fiber.Ctx) string {
	return "endpoint:" + c.Method() + ":" + c.Path()
}

// KeyFunc sets a custom key function for rate limiting.
//
// Example:
//
//	mw := middleware.NewLimit(limiter)
//	mw.KeyFunc(func(c *fiber.Ctx) string {
//	    return "tenant:" + c.Locals("tenant_id").(string)
//	})
func (m *LimitMiddleware) KeyFunc(f func(*fiber.Ctx) string) *LimitMiddleware {
	m.config.KeyFunc = f
	return m
}

// Metrics returns the rate limiting metrics.
func (m *LimitMiddleware) Metrics() *LimitMetrics {
	return m.metrics
}

// Handler returns the Fiber middleware handler.
func (m *LimitMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := m.config.KeyFunc(c)

		allowed, err := m.config.Limiter.Allow(context.Background(), key)
		if err != nil {
			return c.Next()
		}

		current, remaining, _ := m.config.Limiter.GetLimit(context.Background(), key)

		c.Set("X-RateLimit-Limit", strconv.Itoa(current))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		if !allowed {
			atomic.AddInt64(&m.metrics.Rejected, 1)
			c.Set("Retry-After", "60")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
				"code":  "RATE_LIMITED",
			})
		}

		atomic.AddInt64(&m.metrics.Allowed, 1)
		return c.Next()
	}
}
