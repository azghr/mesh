package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRateLimiter_AllowsRequestsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// Make 5 requests (should all succeed)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestHTTPRateLimiter_BlocksRequestsOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// Make 3 requests (should succeed)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 for rate limited request, got %d", w.Code)
	}

	if w.Body.String() != "rate limit exceeded\n" {
		t.Errorf("Expected 'rate limit exceeded' message, got '%s'", w.Body.String())
	}
}

func TestHTTPRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// Make 2 requests (use up limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Now requests should work again
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	middleware.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 after window reset, got %d", w2.Code)
	}
}

func TestHTTPRateLimiter_TracksMultipleIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// IP 1 makes 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP1 Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// IP 2 makes 1 request (leaving room for another)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:1234"
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("IP2 Request 1: Expected status 200, got %d", w.Code)
	}

	// IP 1 should be rate limited on 3rd request
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	middleware.ServeHTTP(w1, req1)

	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("IP1: Expected status 429, got %d", w1.Code)
	}

	// IP 2 should still work (has only made 1 request so far)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:1234"
	w2 := httptest.NewRecorder()
	middleware.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("IP2: Expected status 200, got %d", w2.Code)
	}
}

func TestHTTPRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Use up the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Increase rate limit
	rl.SetRate(5)

	// Now 3rd request should work
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	middleware.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 after increasing rate, got %d", w2.Code)
	}
}

func TestHTTPRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Use up the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}

	// Reset should clear all visitor data
	rl.Reset()

	// Now requests should work again
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 after reset, got %d", w.Code)
	}
}

func TestHTTPRateLimiter_GetVisitorCount(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Initially 0 visitors
	if count := rl.GetVisitorCount(); count != 0 {
		t.Errorf("Expected 0 visitors, got %d", count)
	}

	// Make requests from 3 different IPs
	ips := []string{"192.168.1.1:1234", "192.168.1.2:1234", "192.168.1.3:1234"}
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}

	// Should have 3 visitors
	if count := rl.GetVisitorCount(); count != 3 {
		t.Errorf("Expected 3 visitors, got %d", count)
	}
}

func TestHTTPRateLimiter_RateLimitHeaders(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Make a request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Check headers
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}

	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Expected X-RateLimit-Remaining header")
	}

	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("Expected X-RateLimit-Reset header")
	}
}

func TestHTTPRateLimiter_ConcurrentRequests(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	done := make(chan bool)
	// Make 20 concurrent requests (limit is 10)
	for range 20 {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all to complete
	for range 20 {
		<-done
	}

	// Should have some rate limited requests
	// (We can't easily test exact count due to concurrency, but we verify no panic)
}
