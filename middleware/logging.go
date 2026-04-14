// Package middleware provides HTTP middleware for request logging.
package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/azghr/mesh/logger"
)

const defaultBufferSize = 32 * 1024

type LoggingConfig struct {
	logger         logger.Logger
	bodyCapture    bool
	maxBodySize    int64
	includeHeaders bool
	excludePaths   []string
}

type LogOption func(*LoggingConfig)

func WithBodyCapture(enabled bool) LogOption {
	return func(c *LoggingConfig) {
		c.bodyCapture = enabled
	}
}

func WithMaxBodySize(size int64) LogOption {
	return func(c *LoggingConfig) {
		c.maxBodySize = size
	}
}

func WithLogger(l logger.Logger) LogOption {
	return func(c *LoggingConfig) {
		c.logger = l
	}
}

func WithExcludePaths(paths ...string) LogOption {
	return func(c *LoggingConfig) {
		c.excludePaths = paths
	}
}

func Logging(opts ...LogOption) func(http.Handler) http.Handler {
	cfg := &LoggingConfig{
		logger:      logger.GetGlobal(),
		bodyCapture: false,
		maxBodySize: defaultBufferSize,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isExcluded(r.URL.Path, cfg.excludePaths) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				body:           &strings.Builder{},
				statusCode:     http.StatusOK,
			}

			reqID := generateRequestID()

			cfg.logger.Info("HTTP request started",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			next.ServeHTTP(lrw, r)

			duration := time.Since(start)

			fields := map[string]any{
				"request_id":  reqID,
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      lrw.statusCode,
				"duration_ms": duration.Milliseconds(),
			}

			if lrw.body.Len() > 0 {
				fields["response_size"] = lrw.body.Len()
			}

			if lrw.statusCode >= 500 {
				cfg.logger.Error("HTTP request failed", fields)
			} else if lrw.statusCode >= 400 {
				cfg.logger.Warn("HTTP request warning", fields)
			} else {
				cfg.logger.Info("HTTP request completed", fields)
			}
		})
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func isExcluded(path string, excluded []string) bool {
	for _, p := range excluded {
		if path == p || strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

type loggingResponseWriter struct {
	http.ResponseWriter
	body       *strings.Builder
	statusCode int
	written    bool
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.written = true
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.written {
		lrw.WriteHeader(http.StatusOK)
	}

	if lrw.body.Len() < defaultBufferSize {
		lrw.body.Write(b)
	}

	return lrw.ResponseWriter.Write(b)
}
