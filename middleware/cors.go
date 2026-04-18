// Package middleware provides HTTP middleware for CORS (Cross-Origin Resource Sharing).
//
// The CORS middleware handles:
// - CORS preflight requests (OPTIONS)
// - Setting appropriate CORS response headers
// - Origin validation
// - credentials handling
//
// Example:
//
//	http.Handle("/", middleware.CORS(allowedOrigins, handler))
package middleware

import (
	"net/http"
	"strings"
	"time"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string                 // Origins allowed to access the resource (e.g., ["https://example.com"])
	AllowedMethods   []string                 // HTTP methods allowed (e.g., ["GET", "POST", "PUT", "DELETE"])
	AllowedHeaders   []string                 // Request headers allowed (e.g., ["Content-Type", "Authorization"])
	ExposedHeaders   []string                 // Response headers accessible to the client (e.g., ["X-Request-ID"])
	AllowCredentials bool                     // Whether to allow credentials (cookies, auth headers)
	MaxAge           time.Duration            // How long the preflight response can be cached
	AllowOriginFunc  func(origin string) bool // Custom origin validation function
}

// CORSOption is a function that modifies CORSConfig
type CORSOption func(*CORSConfig)

var defaultCORSConfig = CORSConfig{
	AllowedOrigins:   []string{"*"},
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders:   []string{"Content-Type", "Authorization"},
	ExposedHeaders:   []string{},
	AllowCredentials: false,
	MaxAge:           1 * time.Hour,
	AllowOriginFunc:  nil,
}

// WithAllowedOrigins sets the allowed origins for CORS.
func WithAllowedOrigins(origins ...string) CORSOption {
	return func(c *CORSConfig) {
		c.AllowedOrigins = origins
	}
}

// WithAllowedMethods sets the allowed HTTP methods for CORS.
func WithAllowedMethods(methods ...string) CORSOption {
	return func(c *CORSConfig) {
		c.AllowedMethods = methods
	}
}

// WithAllowedHeaders sets the allowed request headers for CORS.
func WithAllowedHeaders(headers ...string) CORSOption {
	return func(c *CORSConfig) {
		c.AllowedHeaders = headers
	}
}

// WithExposedHeaders sets the exposed response headers for CORS.
func WithExposedHeaders(headers ...string) CORSOption {
	return func(c *CORSConfig) {
		c.ExposedHeaders = headers
	}
}

// WithAllowCredentials sets whether to allow credentials in CORS requests.
func WithAllowCredentials(allow bool) CORSOption {
	return func(c *CORSConfig) {
		c.AllowCredentials = allow
	}
}

// WithMaxAge sets the max age for CORS preflight cache.
func WithMaxAge(maxAge time.Duration) CORSOption {
	return func(c *CORSConfig) {
		c.MaxAge = maxAge
	}
}

// WithAllowOriginFunc sets a custom origin validation function.
func WithAllowOriginFunc(fn func(origin string) bool) CORSOption {
	return func(c *CORSConfig) {
		c.AllowOriginFunc = fn
	}
}

// CORS creates a CORS middleware with the given options.
//
// The middleware handles:
// - Preflight (OPTIONS) requests by returning appropriate headers
// - Adding CORS headers to all responses
// - Validating the Origin header against allowed origins
//
// Example:
//
//	CORS middleware with custom configuration:
//
//	middleware.CORS(
//	    middleware.WithAllowedOrigins("https://example.com"),
//	    middleware.WithAllowedMethods("GET", "POST", "PUT", "DELETE", "OPTIONS"),
//	    middleware.WithAllowedHeaders("Content-Type", "Authorization", "X-Client-Version"),
//	    middleware.WithAllowCredentials(true),
//	    middleware.WithMaxAge(1*time.Hour),
//	)
func CORS(opts ...CORSOption) func(http.Handler) http.Handler {
	cfg := &CORSConfig{}
	*cfg = defaultCORSConfig

	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				handlePreflight(w, r, cfg, origin)
				return
			}

			// Validate origin for simple requests
			if origin != "" && !isOriginAllowed(origin, cfg) {
				// Origin not allowed - still process but don't set CORS headers
				// This is a security measure - origin validation is optional
				next.ServeHTTP(w, r)
				return
			}

			// Set CORS headers for actual request
			setCORSHeaders(w, r, cfg, origin)

			next.ServeHTTP(w, r)
		})
	}
}

// handlePreflight handles CORS preflight (OPTIONS) requests
func handlePreflight(w http.ResponseWriter, r *http.Request, cfg *CORSConfig, origin string) {
	// Validate origin
	if !isOriginAllowed(origin, cfg) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Set preflight response headers
	setCORSHeaders(w, r, cfg, origin)

	// Access-Control-Request-Method is sent by browser to check which method is allowed
	if method := r.Header.Get("Access-Control-Request-Method"); method != "" {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
	}

	// Access-Control-Request-Headers is sent by browser to check which headers are allowed
	// We respond with all allowed headers (not just requested), which is standard practice
	if len(cfg.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
	}

	w.WriteHeader(http.StatusNoContent)
}

// setCORSHeaders sets CORS headers on the response
func setCORSHeaders(w http.ResponseWriter, r *http.Request, cfg *CORSConfig, origin string) {
	// If origin is allowed, set it in the response
	if origin != "" && isOriginAllowed(origin, cfg) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if len(cfg.AllowedOrigins) > 0 {
		// Use first allowed origin if origin validation is not being used strictly
		w.Header().Set("Access-Control-Allow-Origin", cfg.AllowedOrigins[0])
	} else {
		// Allow all origins (wildcard) - be careful with credentials
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	// Set exposed headers
	if len(cfg.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
	}

	// Set allowed methods for preflight responses
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
	}

	// Set credentials flag
	if cfg.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set max age for preflight cache
	if cfg.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", formatCacheControlMaxAge(cfg.MaxAge))
	}
}

// isOriginAllowed checks if the given origin is allowed
func isOriginAllowed(origin string, cfg *CORSConfig) bool {
	if origin == "" {
		return true
	}

	// Check custom function first
	if cfg.AllowOriginFunc != nil {
		return cfg.AllowOriginFunc(origin)
	}

	// Check against allowed origins
	for _, allowed := range cfg.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains (e.g., "https://*.example.com")
		if strings.HasPrefix(allowed, "*.") {
			prefix := allowed[1:] // ".example.com"
			if strings.HasSuffix(origin, prefix) && strings.HasPrefix(origin, "*") {
				// Only allow exact match for wildcard origins
				continue
			}
			// Check if origin ends with the domain part
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// formatCache-Control-Max-Age formats duration for Access-Control-Max-Age header
// The value should be in seconds
func formatCacheControlMaxAge(d time.Duration) string {
	seconds := int(d.Seconds())
	return intToString(seconds)
}

func intToString(i int) string {
	if i <= 0 {
		return "0"
	}
	var buf []byte
	for i > 0 {
		buf = append(buf, byte('0'+i%10))
		i /= 10
	}
	// Reverse the buffer
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}

// AllowAllOrigins creates a CORS configuration that allows all origins.
// This is useful for APIs that need to accept requests from any origin.
// Note: When using this, credentials must be disabled.
func AllowAllOrigins() CORSOption {
	return func(c *CORSConfig) {
		c.AllowedOrigins = []string{"*"}
	}
}

// AllowSpecificOrigins creates a CORS configuration that allows only specific origins.
func AllowSpecificOrigins(origins ...string) CORSOption {
	return func(c *CORSConfig) {
		c.AllowedOrigins = origins
	}
}
