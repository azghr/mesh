package telemetry

import (
	"net/http"
	"time"
)

// HTTPMiddleware returns middleware that records HTTP request metrics
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Increment in-flight counter
			IncrementHTTPRequestsInFlight(serviceName)
			defer DecrementHTTPRequestsInFlight(serviceName)

			// Call next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			RecordHTTPRequest(serviceName, r.Method, r.URL.Path, rw.status, duration)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
