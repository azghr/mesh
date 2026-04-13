package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMiddlewareVerification verifies the middleware works correctly
// This is meant to be run against deployed services to verify middleware is active
func TestMiddlewareVerification(t *testing.T) {
	t.Run("Validation middleware rejects invalid JSON", func(t *testing.T) {
		validation := NewValidationMiddleware()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := validation.ValidateRequest()(handler)

		// Test with invalid JSON
		req := httptest.NewRequest("POST", "/api", strings.NewReader(`{"invalid": json}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Validation middleware accepts valid JSON", func(t *testing.T) {
		validation := NewValidationMiddleware()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := validation.ValidateRequest()(handler)

		// Test with valid JSON
		req := httptest.NewRequest("POST", "/api", strings.NewReader(`{"valid": "json"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("Rate limiter blocks excess requests", func(t *testing.T) {
		rateLimiter := NewRateLimiter(2, time.Minute)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := rateLimiter.Middleware(handler)

		// Make 2 successful requests
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api", nil)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
			}
		}

		// Third request should be rate limited
		req := httptest.NewRequest("GET", "/api", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", w.Code)
		}
	})

	t.Run("Rate limiter resets after window expires", func(t *testing.T) {
		rateLimiter := NewRateLimiter(2, 200*time.Millisecond)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := rateLimiter.Middleware(handler)

		// Use up the limit
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api", nil)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
		}

		// Should be rate limited
		req := httptest.NewRequest("GET", "/api", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", w.Code)
		}

		// Wait for window to expire
		time.Sleep(250 * time.Millisecond)

		// Should work again
		req = httptest.NewRequest("GET", "/api", nil)
		w = httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 after reset, got %d", w.Code)
		}
	})
}

// TestMiddlewareCombined verifies both middleware work together
func TestMiddlewareCombined(t *testing.T) {
	validation := NewValidationMiddleware()
	rateLimiter := NewRateLimiter(5, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	middleware := rateLimiter.Middleware(
		validation.ValidateRequest()(handler),
	)

	// Test valid request
	validJSON := `{"test": "data"}`
	req := httptest.NewRequest("POST", "/api", strings.NewReader(validJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid request: Expected status 200, got %d", w.Code)
	}

	// Test invalid JSON - should be rejected by validation, not counted against rate limit
	invalidJSON := `{"bad": json}`
	for i := 0; i < 3; i++ {
		req = httptest.NewRequest("POST", "/api", strings.NewReader(invalidJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Invalid JSON: Expected status 400, got %d", w.Code)
		}
	}

	// Should still have rate limit available
	req = httptest.NewRequest("GET", "/api", nil)
	w = httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Should still have rate limit available, got status %d", w.Code)
	}
}
