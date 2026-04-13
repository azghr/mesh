package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecordHTTPRequest(t *testing.T) {
	t.Run("RecordHTTPRequest_Success", func(t *testing.T) {
		// This should not panic
		RecordHTTPRequest("test-service", "GET", "/api/test", 200, 100*time.Millisecond)
		RecordHTTPRequest("test-service", "POST", "/api/users", 201, 150*time.Millisecond)
		RecordHTTPRequest("test-service", "GET", "/api/test", 404, 50*time.Millisecond)
		RecordHTTPRequest("test-service", "GET", "/api/error", 500, 200*time.Millisecond)
	})

	t.Run("RecordHTTPRequest_StatusLabels", func(t *testing.T) {
		// Test different status code ranges
		tests := []struct {
			statusCode int
			wantLabel  string
		}{
			{200, "2xx"},
			{201, "2xx"},
			{299, "2xx"},
			{301, "3xx"},
			{302, "3xx"},
			{400, "4xx"},
			{404, "4xx"},
			{500, "5xx"},
			{503, "5xx"},
		}

		for _, tt := range tests {
			// This should not panic
			RecordHTTPRequest("test-service", "GET", "/api/test", tt.statusCode, 100*time.Millisecond)
		}
	})
}

func TestHTTPRequestsInFlight(t *testing.T) {
	t.Run("IncrementDecrementRequestsInFlight", func(t *testing.T) {
		// This should not panic
		IncrementHTTPRequestsInFlight("test-service")
		IncrementHTTPRequestsInFlight("test-service")
		DecrementHTTPRequestsInFlight("test-service")
		DecrementHTTPRequestsInFlight("test-service")
	})
}

func TestRecordGRPCRequest(t *testing.T) {
	t.Run("RecordGRPCRequest_Success", func(t *testing.T) {
		// This should not panic
		RecordGRPCRequest("test-service", "CreateUser", "OK", 50*time.Millisecond)
		RecordGRPCRequest("test-service", "GetUser", "OK", 25*time.Millisecond)
		RecordGRPCRequest("test-service", "DeleteUser", "NotFound", 30*time.Millisecond)
		RecordGRPCRequest("test-service", "UpdateUser", "Internal", 100*time.Millisecond)
	})
}

func TestRecordDBQuery(t *testing.T) {
	t.Run("RecordDBQuery_Success", func(t *testing.T) {
		// This should not panic
		RecordDBQuery("test-service", "SELECT", true, 50*time.Millisecond)
		RecordDBQuery("test-service", "INSERT", true, 75*time.Millisecond)
		RecordDBQuery("test-service", "UPDATE", true, 80*time.Millisecond)
		RecordDBQuery("test-service", "DELETE", true, 40*time.Millisecond)
	})

	t.Run("RecordDBQuery_Error", func(t *testing.T) {
		// This should not panic
		RecordDBQuery("test-service", "SELECT", false, 100*time.Millisecond)
		RecordDBQuery("test-service", "INSERT", false, 50*time.Millisecond)
	})
}

func TestSetDBConnections(t *testing.T) {
	t.Run("SetDBConnections", func(t *testing.T) {
		// This should not panic
		SetDBConnections("test-service", 10.0)
		SetDBConnections("test-service", 20.0)
		SetDBConnections("test-service", 5.0)
		SetDBConnections("test-service", 0.0)
	})
}

func TestRecordExternalAPICall(t *testing.T) {
	t.Run("RecordExternalAPICall_Success", func(t *testing.T) {
		// This should not panic
		RecordExternalAPICall("test-service", "coingecko", "/api/v3/coins/bitcoin", true, 200*time.Millisecond)
		RecordExternalAPICall("test-service", "dexscreener", "/dex/search", true, 150*time.Millisecond)
	})

	t.Run("RecordExternalAPICall_Error", func(t *testing.T) {
		// This should not panic
		RecordExternalAPICall("test-service", "coingecko", "/api/v3/coins/bitcoin", false, 500*time.Millisecond)
		RecordExternalAPICall("test-service", "geckoterminal", "/api/v2/networks", false, 1000*time.Millisecond)
	})
}

func TestRecordCache(t *testing.T) {
	t.Run("RecordCacheHit", func(t *testing.T) {
		// This should not panic
		RecordCacheHit("test-service", "token-cache")
		RecordCacheHit("test-service", "user-cache")
		RecordCacheHit("test-service", "price-cache")
	})

	t.Run("RecordCacheMiss", func(t *testing.T) {
		// This should not panic
		RecordCacheMiss("test-service", "token-cache")
		RecordCacheMiss("test-service", "user-cache")
		RecordCacheMiss("test-service", "price-cache")
	})
}

func TestRecordAIRequest(t *testing.T) {
	t.Run("RecordAIRequest_Success", func(t *testing.T) {
		// This should not panic
		RecordAIRequest("test-service", "gpt-4", true, 1500*time.Millisecond)
		RecordAIRequest("test-service", "gpt-3.5-turbo", true, 800*time.Millisecond)
	})

	t.Run("RecordAIRequest_Error", func(t *testing.T) {
		// This should not panic
		RecordAIRequest("test-service", "gpt-4", false, 5000*time.Millisecond)
		RecordAIRequest("test-service", "gpt-3.5-turbo", false, 3000*time.Millisecond)
	})
}

func TestRecordTokenSync(t *testing.T) {
	t.Run("RecordTokenSync_Success", func(t *testing.T) {
		// This should not panic
		RecordTokenSync("test-service", true, 2000*time.Millisecond)
		RecordTokenSync("test-service", true, 1500*time.Millisecond)
	})

	t.Run("RecordTokenSync_Error", func(t *testing.T) {
		// This should not panic
		RecordTokenSync("test-service", false, 5000*time.Millisecond)
		RecordTokenSync("test-service", false, 3000*time.Millisecond)
	})
}

func TestHandler(t *testing.T) {
	t.Run("Handler_ReturnsHandler", func(t *testing.T) {
		handler := Handler()

		if handler == nil {
			t.Errorf("Expected handler to be non-nil")
		}

		// Test that the handler is functional
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Check that we got a response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Check content type
		contentType := w.Header().Get("Content-Type")
		if len(contentType) < 10 || contentType[:10] != "text/plain" {
			t.Errorf("Expected content type to start with 'text/plain', got %s", contentType)
		}
	})
}

func TestGetStatusLabel(t *testing.T) {
	t.Run("GetStatusLabel_2xx", func(t *testing.T) {
		tests := []int{200, 201, 202, 204, 299}
		for _, code := range tests {
			label := getStatusLabel(code)
			if label != "2xx" {
				t.Errorf("Expected '2xx' for status %d, got %s", code, label)
			}
		}
	})

	t.Run("GetStatusLabel_3xx", func(t *testing.T) {
		tests := []int{300, 301, 302, 304, 399}
		for _, code := range tests {
			label := getStatusLabel(code)
			if label != "3xx" {
				t.Errorf("Expected '3xx' for status %d, got %s", code, label)
			}
		}
	})

	t.Run("GetStatusLabel_4xx", func(t *testing.T) {
		tests := []int{400, 401, 403, 404, 499}
		for _, code := range tests {
			label := getStatusLabel(code)
			if label != "4xx" {
				t.Errorf("Expected '4xx' for status %d, got %s", code, label)
			}
		}
	})

	t.Run("GetStatusLabel_5xx", func(t *testing.T) {
		tests := []int{500, 501, 502, 503, 599}
		for _, code := range tests {
			label := getStatusLabel(code)
			if label != "5xx" {
				t.Errorf("Expected '5xx' for status %d, got %s", code, label)
			}
		}
	})

	t.Run("GetStatusLabel_Unknown", func(t *testing.T) {
		tests := []int{100, 150, 1, 99, 600, 999}
		for _, code := range tests {
			label := getStatusLabel(code)
			if label != "unknown" {
				t.Errorf("Expected 'unknown' for status %d, got %s", code, label)
			}
		}
	})
}

// Benchmark tests

func BenchmarkRecordHTTPRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordHTTPRequest("test-service", "GET", "/api/test", 200, 100*time.Millisecond)
	}
}

func BenchmarkRecordDBQuery(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordDBQuery("test-service", "SELECT", true, 50*time.Millisecond)
	}
}

func BenchmarkRecordCacheHit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordCacheHit("test-service", "test-cache")
	}
}

func BenchmarkRecordExternalAPICall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RecordExternalAPICall("test-service", "test-provider", "/api/test", true, 100*time.Millisecond)
	}
}
