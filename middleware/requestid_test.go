package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRequestID_GeneratesNewID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Locals("request_id").(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRequestID_UsesExistingID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Locals("request_id").(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "existing-id-123")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRequestID_SetsResponseHeader(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}

func TestRequestID_WithCustomHeader(t *testing.T) {
	cfg := DefaultRequestIDConfig()
	cfg.HeaderName = "X-Correlation-ID"

	app := fiber.New()
	app.Use(RequestIDWithConfig(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Correlation-ID", "custom-id")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, "custom-id", resp.Header.Get("X-Correlation-ID"))
}

func TestRequestID_WithCustomGenerator(t *testing.T) {
	cfg := DefaultRequestIDConfig()
	cfg.Generator = func() string { return "custom-generator-id" }

	app := fiber.New()
	app.Use(RequestIDWithConfig(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Locals("request_id").(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, "custom-generator-id", resp.Header.Get("X-Request-ID"))
}

func TestGetRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/", func(c *fiber.Ctx) error {
		id := GetRequestID(c)
		return c.SendString(id)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}
