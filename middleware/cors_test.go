package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCORS_AllowedOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		config         []CORSOption
		origin         string
		expectedOrigin string
	}{
		{
			name:           "single origin allowed",
			config:         []CORSOption{WithAllowedOrigins("https://example.com")},
			origin:         "https://example.com",
			expectedOrigin: "https://example.com",
		},
		{
			name:           "multiple origins allowed",
			config:         []CORSOption{WithAllowedOrigins("https://example.com", "https://api.example.com")},
			origin:         "https://example.com",
			expectedOrigin: "https://example.com",
		},
		{
			name:           "origin not in list",
			config:         []CORSOption{WithAllowedOrigins("https://example.com")},
			origin:         "https://evil.com",
			expectedOrigin: "",
		},
		{
			name:           "wildcard allows all",
			config:         []CORSOption{WithAllowedOrigins("*")},
			origin:         "https://any.com",
			expectedOrigin: "https://any.com", // Echoes back origin when allowed (more secure)
		},
		{
			name:           "no origin header",
			config:         []CORSOption{WithAllowedOrigins("https://example.com")},
			origin:         "",
			expectedOrigin: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORS(tt.config...)(next)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.expectedOrigin)
			}
		})
	}
}

func TestCORS_Preflight(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name            string
		config          []CORSOption
		origin          string
		requestMethod   string
		requestHeaders  string
		expectedMethods string
		expectedHeaders string
		expectedStatus  int
	}{
		{
			name:           "simple preflight",
			config:         []CORSOption{WithAllowedOrigins("https://example.com"), WithAllowedMethods("GET", "POST"), WithAllowedHeaders("Content-Type")},
			origin:         "https://example.com",
			requestMethod:  "GET",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:            "preflight with method check",
			config:          []CORSOption{WithAllowedOrigins("https://example.com"), WithAllowedMethods("GET", "POST", "PUT")},
			origin:          "https://example.com",
			requestMethod:   "GET",
			requestHeaders:  "Content-Type",
			expectedMethods: "GET, POST, PUT",
			expectedHeaders: "Content-Type, Authorization", // Returns all allowed headers
			expectedStatus:  http.StatusNoContent,
		},
		{
			name:           "origin not allowed returns no content",
			config:         []CORSOption{WithAllowedOrigins("https://example.com")},
			origin:         "https://evil.com",
			requestMethod:  "GET",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORS(tt.config...)(next)

			req := httptest.NewRequest(http.MethodOptions, "/", nil)
			req.Header.Set("Origin", tt.origin)
			if tt.requestMethod != "" {
				req.Header.Set("Access-Control-Request-Method", tt.requestMethod)
			}
			if tt.requestHeaders != "" {
				req.Header.Set("Access-Control-Request-Headers", tt.requestHeaders)
			}

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.expectedMethods != "" {
				got := w.Header().Get("Access-Control-Allow-Methods")
				if got != tt.expectedMethods {
					t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, tt.expectedMethods)
				}
			}

			if tt.expectedHeaders != "" {
				got := w.Header().Get("Access-Control-Allow-Headers")
				if got != tt.expectedHeaders {
					t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, tt.expectedHeaders)
				}
			}
		})
	}
}

func TestCORS_AllowCredentials(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(WithAllowedOrigins("https://example.com"), WithAllowCredentials(true))(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	got := w.Header().Get("Access-Control-Allow-Credentials")
	if got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(WithAllowedOrigins("https://example.com"), WithExposedHeaders("X-Request-ID", "X-Rate-Limit"))(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	got := w.Header().Get("Access-Control-Expose-Headers")
	expected := "X-Request-ID, X-Rate-Limit"
	if got != expected {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, expected)
	}
}

func TestCORS_MaxAge(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(WithAllowedOrigins("https://example.com"), WithMaxAge(1*time.Hour))(next)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	got := w.Header().Get("Access-Control-Max-Age")
	if got != "3600" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "3600")
	}
}

func TestCORS_CustomOriginFunc(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Custom function that only allows origins containing "example"
	customFunc := func(origin string) bool {
		return strings.Contains(origin, "example")
	}

	middleware := CORS(WithAllowOriginFunc(customFunc))(next)

	tests := []struct {
		name           string
		origin         string
		expectedOrigin string
	}{
		{
			name:           "origin contains example",
			origin:         "https://example.com",
			expectedOrigin: "https://example.com",
		},
		{
			name:           "origin contains foo",
			origin:         "https://foo.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tt.origin)

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.expectedOrigin)
			}
		})
	}
}

func TestCORS_DefaultConfig(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use CORS with default config (allows all origins)
	middleware := CORS()(next)

	// Test with an origin header - should echo back the origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.com")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Default allows all, so origin is echoed back
	got := w.Header().Get("Access-Control-Allow-Origin")
	if got != "https://any.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://any.com")
	}
}

func TestCORS_NotOptionsRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(WithAllowedOrigins("https://example.com"))(next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Should still set CORS headers for actual request
	got := w.Header().Get("Access-Control-Allow-Origin")
	if got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
}
