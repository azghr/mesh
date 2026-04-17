// Package middleware provides HTTP middleware for Fiber applications.
//
// This package includes request ID middleware for distributed tracing,
// JWT authentication, RBAC authorization, and other common HTTP middleware.
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RequestIDConfig configures request ID middleware.
type RequestIDConfig struct {
	HeaderName   string
	Generator    func() string
	ErrorHandler fiber.Handler
}

// DefaultRequestIDConfig returns default configuration.
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		HeaderName: "X-Request-ID",
		Generator: func() string {
			return uuid.New().String()
		},
	}
}

// RequestID returns a middleware that generates and propagates request IDs.
func RequestID() fiber.Handler {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns a middleware with custom configuration.
func RequestIDWithConfig(cfg RequestIDConfig) fiber.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Request-ID"
	}
	if cfg.Generator == nil {
		cfg.Generator = func() string {
			return uuid.New().String()
		}
	}

	return func(c *fiber.Ctx) error {
		// Check for existing request ID in header
		requestID := c.Get(cfg.HeaderName)

		// Generate new if not present
		if requestID == "" {
			requestID = cfg.Generator()
		}

		// Store in context locals for handler access
		c.Locals("request_id", requestID)

		// Set response header
		c.Set(cfg.HeaderName, requestID)

		// Continue to next handler
		if cfg.ErrorHandler != nil {
			return cfg.ErrorHandler(c)
		}

		return c.Next()
	}
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return c.Get("X-Request-ID")
}
