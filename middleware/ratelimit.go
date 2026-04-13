package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter provides token-bucket rate limiting for HTTP requests.
//
// RateLimiter tracks requests per IP address and enforces configurable
// request limits within sliding time windows. It automatically cleans up
// old visitor entries to prevent memory leaks.
//
// The rate limiter adds the following headers to responses:
//   - X-RateLimit-Limit: Maximum requests allowed
//   - X-RateLimit-Remaining: Requests remaining in current window
//   - X-RateLimit-Reset: When the window will reset
//   - Retry-After: Suggested retry duration (when limited)
//
// Example:
//
//	// Allow 100 requests per minute
//	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
//	http.Handle("/api", rateLimiter.Middleware(handler))
type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
	rate     int           // requests allowed per window
	window   time.Duration // time window for rate limiting
}

// Visitor tracks requests from a single client (identified by IP)
type Visitor struct {
	requests  []time.Time
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter with the specified parameters.
//
// rate is the maximum number of requests allowed within the time window.
// window is the duration of the time window (e.g., time.Minute).
//
// The rate limiter automatically starts a cleanup goroutine that removes
// visitors inactive for more than 1 hour to prevent memory leaks.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		rate:     rate,
		window:   window,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// extractIP securely extracts the client IP address from the request.
// It validates the IP format and only trusts X-Forwarded-For from known proxies.
func extractIP(r *http.Request) string {
	// Get the direct connection IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// Validate the IP is in a proper format
	if net.ParseIP(ip) == nil {
		return "" // Invalid IP
	}

	// Check if request is from a trusted proxy
	// In production, you should maintain a list of trusted proxy IPs/CIDRs
	// For now, we'll only trust X-Forwarded-For if the direct connection
	// is from localhost or a known private network
	if isTrustedProxy(ip) {
		// Parse X-Forwarded-For header (can contain multiple IPs: "client, proxy1, proxy2")
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// Get the first IP (original client)
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				clientIP := strings.TrimSpace(parts[0])
				// Validate the client IP
				if net.ParseIP(clientIP) != nil {
					return clientIP
				}
			}
		}
	}

	// Return the direct connection IP
	return ip
}

// isTrustedProxy checks if the given IP is a trusted proxy.
// In production, this should check against a list of known proxy IPs.
func isTrustedProxy(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Trust localhost and private networks
	// You should customize this based on your infrastructure
	return parsedIP.IsLoopback() ||
		parsedIP.IsPrivate() ||
		parsedIP.IsLinkLocalUnicast()
}

// Middleware returns an HTTP middleware that implements rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Securely extract IP address
		ip := extractIP(r)

		// If we couldn't get a valid IP, reject the request
		if ip == "" {
			http.Error(w, "unable to determine client IP", http.StatusBadRequest)
			return
		}

		rl.mu.RLock()
		visitor, exists := rl.visitors[ip]
		rl.mu.RUnlock()

		now := time.Now()
		if !exists {
			visitor = &Visitor{lastReset: now}
			rl.mu.Lock()
			rl.visitors[ip] = visitor
			rl.mu.Unlock()
		}

		// Reset window if expired
		if now.Sub(visitor.lastReset) > rl.window {
			visitor.requests = nil
			visitor.lastReset = now
		}

		// Add current request
		visitor.requests = append(visitor.requests, now)

		// Check if limit exceeded
		if len(visitor.requests) > rl.rate {
			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", visitor.lastReset.Add(rl.window).Format(time.RFC1123))
			w.Header().Set("Retry-After", rl.window.String())

			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Set rate limit headers for successful request
		remaining := rl.rate - len(visitor.requests)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", visitor.lastReset.Add(rl.window).Format(time.RFC1123))

		next.ServeHTTP(w, r)
	})
}

// cleanup removes old visitor entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Hour) // Remove visitors inactive for 1 hour
		for ip, v := range rl.visitors {
			if v.lastReset.Before(cutoff) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// GetVisitorCount returns the current number of tracked visitors
func (rl *RateLimiter) GetVisitorCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.visitors)
}

// Reset clears all visitor data
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.visitors = make(map[string]*Visitor)
}

// SetRate changes the rate limit
func (rl *RateLimiter) SetRate(rate int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
}

// SetWindow changes the rate limit window
func (rl *RateLimiter) SetWindow(window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.window = window
}
