package telemetry

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// FiberMiddleware returns Fiber middleware that records HTTP request metrics
func FiberMiddleware(serviceName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Increment in-flight counter
		IncrementHTTPRequestsInFlight(serviceName)
		defer DecrementHTTPRequestsInFlight(serviceName)

		// Continue with request
		err := c.Next()

		// Record metrics
		duration := time.Since(start)
		statusCode := c.Response().StatusCode()
		RecordHTTPRequest(serviceName, c.Method(), c.Path(), statusCode, duration)

		return err
	}
}
