// Package middleware provides HTTP middleware for request validation and rate limiting.
//
// # Validation Middleware
//
// The ValidationMiddleware provides comprehensive request validation:
//   - Request body size limits (prevents memory exhaustion)
//   - Content-Type validation (ensures JSON for POST/PUT/PATCH)
//   - JSON format validation (prevents parsing errors)
//
// Example usage:
//
//	validation := middleware.NewValidationMiddleware()
//	validation.SetMaxBodySize(2 << 20) // 2MB
//	http.Handle("/api", validation.ValidateRequest()(handler))
//
// # Rate Limiting Middleware
//
// The RateLimiter provides token-bucket rate limiting per IP address:
//   - Configurable request limits per time window
//   - Automatic cleanup of old entries
//   - Rate limit headers in responses
//
// Example usage:
//
//	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
//	http.Handle("/api", rateLimiter.Middleware(handler))
//
// # Combining Middleware
//
// Middleware can be chained for comprehensive protection:
//
//	validation := middleware.NewValidationMiddleware()
//	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
//
//	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		// Your handler code
//	})
//
//	http.Handle("/api",
//		rateLimiter.Middleware(
//			validation.ValidateRequest()(handler,
//		),
//	)
package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ValidationMiddleware provides HTTP request validation
type ValidationMiddleware struct {
	maxBodySize int64
}

// NewValidationMiddleware creates a new validation middleware
// maxBodySize is the maximum allowed request body size in bytes (default 1MB)
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		maxBodySize: 1 << 20, // 1MB default
	}
}

// SetMaxBodySize sets the maximum body size
func (v *ValidationMiddleware) SetMaxBodySize(size int64) *ValidationMiddleware {
	v.maxBodySize = size
	return v
}

// ValidateRequest returns an http.Handler that validates incoming requests
func (v *ValidationMiddleware) ValidateRequest() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate content length
			if r.ContentLength > v.maxBodySize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Validate content type for POST/PUT/PATCH
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				ct := r.Header.Get("Content-Type")
				if ct != "" && !strings.HasPrefix(ct, "application/json") {
					http.Error(w, "invalid content type, application/json required", http.StatusUnsupportedMediaType)
					return
				}

				// Validate JSON body if present
				if r.Body != nil && ct != "" && strings.HasPrefix(ct, "application/json") {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "invalid request body", http.StatusBadRequest)
						return
					}
					defer r.Body.Close()

					// Check if body is too large (Content-Length might be missing)
					if int64(len(body)) > v.maxBodySize {
						http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
						return
					}

					// Validate JSON
					if len(body) > 0 && !json.Valid(body) {
						http.Error(w, "invalid JSON format", http.StatusBadRequest)
						return
					}

					// Replace body for next handler
					r.Body = io.NopCloser(strings.NewReader(string(body)))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateJSON is a helper function to validate JSON bytes
func ValidateJSON(data []byte) bool {
	return json.Valid(data)
}

// ValidateContentType checks if the content type is valid
func ValidateContentType(ct string, allowedTypes []string) bool {
	if ct == "" {
		return true
	}
	for _, allowed := range allowedTypes {
		if strings.HasPrefix(ct, allowed) {
			return true
		}
	}
	return false
}
