package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware adds security headers to all responses.
//
// This middleware implements defense-in-depth by adding multiple security headers:
// - HSTS (HTTP Strict Transport Security): Enforces HTTPS connections
// - CSP (Content Security Policy): Controls resource loading
// - X-Frame-Options: Prevents clickjacking attacks
// - X-Content-Type-Options: Prevents MIME type sniffing
// - Referrer-Policy: Controls referrer information leakage
// - Permissions-Policy: Restricts browser features
// - X-XSS-Protection: Enables XSS filtering (legacy)
// - Cross-Origin-Opener-Policy: Controls cross-origin window access
// - Cross-Origin-Resource-Policy: Controls cross-origin resource access
//
// Example:
//
//	http.Handler.With(middleware.SecurityHeadersMiddleware)
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HSTS (HTTP Strict Transport Security)
		// Enforces HTTPS for 1 year including subdomains, allows preload list
		w.Header().Set("Strict-Transport-Security",
			"max-age=31536000; includeSubDomains; preload")

		// Content Security Policy
		// Restricts resource loading to same origin by default
		// Allows inline scripts/styles for development (should be tightened in production)
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' wss:; "+
				"frame-ancestors 'none';")

		// X-Frame-Options: DENY
		// Prevents your page from being framed by any site
		w.Header().Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options: nosniff
		// Prevents browser from MIME-sniffing away from declared content-type
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Referrer-Policy: strict-origin-when-cross-origin
		// Only sends full referrer to same origin, stripped to origin for cross-origin
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy
		// Restricts access to browser features and APIs
		w.Header().Set("Permissions-Policy",
			"geolocation=(), "+
				"microphone=(), "+
				"camera=(), "+
				"fullscreen=(self), "+
				"payment=()")

		// X-XSS-Protection: 1; mode=block
		// Enables XSS filtering (legacy but still provides some protection)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Cross-Origin-Opener-Policy: same-origin
		// Ensures cross-origin opener is limited to same-origin
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		// Cross-Origin-Resource-Policy: same-origin
		// Prevents other origins from loading this resource
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

		// Remove server information to avoid revealing server version
		w.Header().Set("Server", "")

		next.ServeHTTP(w, r)
	})
}

// ValidateHTTPS checks if the request is using HTTPS (in production).
// Allows localhost and 127.0.0.1 for development.
// Redirects HTTP to HTTPS in production.
//
// Example:
//
//	http.Handler.With(middleware.ValidateHTTPS)
func ValidateHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow localhost and 127.0.0.1 for development
		host := r.Host
		if strings.HasPrefix(host, "localhost") ||
			strings.HasPrefix(host, "127.0.0.1") ||
			strings.HasPrefix(host, "[::1]") {
			next.ServeHTTP(w, r)
			return
		}

		// Check for HTTPS via X-Forwarded-Proto (when behind proxy)
		proto := r.Header.Get("X-Forwarded-Proto")
		if proto == "https" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if the request itself is HTTPS
		if r.URL.Scheme == "https" {
			next.ServeHTTP(w, r)
			return
		}

		// Redirect to HTTPS
		httpsURL := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, httpsURL, http.StatusPermanentRedirect)
	})
}

// ContentSecurityPolicyBuilder creates a custom CSP header.
// Use this to customize the CSP for specific routes.
//
// Example:
//
//	cspMiddleware := ContentSecurityPolicyBuilder(
//	    "default-src 'self'",
//	    "script-src 'self' https://cdn.example.com",
//	    "style-src 'self' 'unsafe-inline'",
//	)
func ContentSecurityPolicyBuilder(directives ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cspValue := strings.Join(directives, "; ")
			w.Header().Set("Content-Security-Policy", cspValue)
			next.ServeHTTP(w, r)
		})
	}
}
