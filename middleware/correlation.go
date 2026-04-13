// Package middleware provides HTTP middleware for correlation IDs and request tracking.
//
// The correlation middleware provides:
// - Correlation ID generation and tracking
// - Request logging with timing
// - Response status code capture
// - User ID context tracking
//
// Example:
//
//	http.Handle("/", middleware.CorrelationMiddleware("my-service", handler))
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/azghr/mesh/logger"
)

// Context keys for correlation data
type contextKey int

const (
	CorrelationIDKey contextKey = iota
	UserIDKey
)

// CorrelationMiddleware adds correlation IDs to requests and provides request logging.
//
// The middleware:
// 1. Extracts or generates a correlation ID from X-Correlation-ID or X-Request-ID headers
// 2. Adds the correlation ID to the response headers
// 3. Stores the correlation ID and optional user ID in the request context
// 4. Captures the response status code
// 5. Logs the request with timing information
func CorrelationMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get or generate correlation ID
		cid := r.Header.Get("X-Correlation-ID")
		if cid == "" {
			cid = r.Header.Get("X-Request-ID")
		}
		if cid == "" {
			cid = generateCorrelationID()
		}

		// Create context with correlation data
		ctx := WithCorrelationID(r.Context(), cid)

		// Add user ID if available from auth header
		if userID := r.Header.Get("X-User-Id"); userID != "" {
			ctx = WithUserID(ctx, userID)
		}

		// Create response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Log request with correlation data
		duration := time.Since(start)
		logRequest(serviceName, cid, r, duration, rw.statusCode)

		// Add correlation ID to response headers
		w.Header().Set("X-Correlation-ID", cid)
	})
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if cid, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return cid
	}
	return ""
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserID retrieves the user ID from the context
func GetUserID(ctx context.Context) string {
	if uid, ok := ctx.Value(UserIDKey).(string); ok {
		return uid
	}
	return ""
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// generateCorrelationID generates a new correlation ID
func generateCorrelationID() string {
	return generateUUID()
}

// logRequest logs an HTTP request with correlation data
func logRequest(serviceName, correlationID string, r *http.Request, duration time.Duration, statusCode int) {
	logger.GetGlobal().Info("HTTP request",
		"service", serviceName,
		"correlation_id", correlationID,
		"method", r.Method,
		"path", r.URL.Path,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
	)
}

// generateUUID generates a random UUID string (simplified implementation)
func generateUUID() string {
	// Simple UUID v4-like generation
	// In production, use github.com/google/uuid or similar
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
