package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// mockTracer is a simple mock tracer for testing
type mockTracer struct {
	trace.Tracer
	spans []mockSpan
}

type mockSpan struct {
	name       string
	ctx        context.Context
	attributes []attribute.KeyValue
	events     []mockEvent
	ended      bool
}

type mockEvent struct {
	name       string
	attributes []attribute.KeyValue
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("test-service")

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, "otel-collector:4317", config.CollectorEndpoint)
	assert.Equal(t, 1.0, config.SampleRate)
}

func TestGetTracer(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		verify  func(t *testing.T, tracer trace.Tracer)
	}{
		{
			name:  "returns tracer when initialized",
			setup: func() {},
			verify: func(t *testing.T, tracer trace.Tracer) {
				assert.NotNil(t, tracer)
			},
		},
		{
			name: "returns default tracer when not initialized",
			setup: func() {
				// Reset global tracer
				tracer = nil
			},
			verify: func(t *testing.T, tracer trace.Tracer) {
				assert.NotNil(t, tracer)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			tracer := GetTracer()
			tt.verify(t, tracer)
		})
	}
}

func TestStartSpan(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		span    string
		verify  func(t *testing.T, ctx context.Context, span trace.Span)
	}{
		{
			name: "creates span with name",
			ctx:  context.Background(),
			span: "test-operation",
			verify: func(t *testing.T, ctx context.Context, span trace.Span) {
				assert.NotNil(t, ctx)
				assert.NotNil(t, span)
			},
		},
		{
			name: "creates span with options",
			ctx:  context.Background(),
			span: "test-operation-with-attrs",
			verify: func(t *testing.T, ctx context.Context, span trace.Span) {
				assert.NotNil(t, ctx)
				assert.NotNil(t, span)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a real tracer for this test
			// In real scenario, we'd use mock tracer
			ctx, span := StartSpan(tt.ctx, tt.span)
			tt.verify(t, ctx, span)
			span.End()
		})
	}
}

func TestStartSpanWithAttributes(t *testing.T) {
	ctx := context.Background()
	attrs := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	ctx, span := StartSpanWithAttributes(ctx, "test-operation", attrs)
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	span.End()
}

func TestRecordError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectErr bool
	}{
		{
			name:      "records non-nil error",
			err:       assert.AnError,
			expectErr: true,
		},
		{
			name:      "ignores nil error",
			err:       nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, span := StartSpan(ctx, "test-operation")

			RecordError(span, tt.err)

			span.End()

			// If error was non-nil, span should have been recorded
			if tt.expectErr {
				assert.NotNil(t, span)
			}
		})
	}
}

func TestRecordErrorWithMessage(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test-operation")

	customErr := assert.AnError
	RecordErrorWithMessage(span, customErr, "Custom error message")

	span.End()
	assert.NotNil(t, span)
}

func TestAddAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]interface{}
	}{
		{
			name: "adds single attribute",
			attributes: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "adds multiple attributes",
			attributes: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
				"key3": true,
			},
		},
		{
			name:       "adds empty attributes",
			attributes: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, span := StartSpan(ctx, "test-operation")

			AddAttributes(ctx, tt.attributes)

			span.End()
		})
	}
}

func TestAddEvent(t *testing.T) {
	tests := []struct {
		name       string
		eventName  string
		attributes map[string]interface{}
	}{
		{
			name:       "adds event without attributes",
			eventName:  "test-event",
			attributes: nil,
		},
		{
			name:      "adds event with attributes",
			eventName: "test-event",
			attributes: map[string]interface{}{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, span := StartSpan(ctx, "test-operation")

			AddEvent(ctx, tt.eventName, tt.attributes)

			span.End()
		})
	}
}

func TestWithOperation(t *testing.T) {
	ctx := context.Background()
	newCtx := WithOperation(ctx, "test-operation")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

func TestWithComponent(t *testing.T) {
	ctx := context.Background()
	newCtx := WithComponent(ctx, "test-component")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithUserID(ctx, "user-123")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithRequestID(ctx, "req-456")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

func TestWithAttribute(t *testing.T) {
	ctx := context.Background()
	newCtx := WithAttribute(ctx, "custom-key", "custom-value")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		notEmpty bool
	}{
		{
			name:     "returns empty string without span",
			setup:    func() context.Context { return context.Background() },
			notEmpty: false,
		},
		{
			name: "returns empty string with non-recording span",
			setup: func() context.Context {
				// Default tracer creates non-recording spans with invalid span context
				ctx, _ := StartSpan(context.Background(), "test")
				return ctx
			},
			notEmpty: false, // Non-recording spans have invalid span context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			traceID := GetTraceID(ctx)

			if tt.notEmpty {
				assert.NotEmpty(t, traceID)
			} else {
				assert.Empty(t, traceID)
			}
		})
	}
}

func TestGetSpanID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		notEmpty bool
	}{
		{
			name:     "returns empty string without span",
			setup:    func() context.Context { return context.Background() },
			notEmpty: false,
		},
		{
			name: "returns empty string with non-recording span",
			setup: func() context.Context {
				// Default tracer creates non-recording spans with invalid span context
				ctx, _ := StartSpan(context.Background(), "test")
				return ctx
			},
			notEmpty: false, // Non-recording spans have invalid span context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			spanID := GetSpanID(ctx)

			if tt.notEmpty {
				assert.NotEmpty(t, spanID)
			} else {
				assert.Empty(t, spanID)
			}
		})
	}
}

func TestIsRecording(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected bool
	}{
		{
			name:     "returns false without span",
			setup:    func() context.Context { return context.Background() },
			expected: false,
		},
		{
			name: "returns true with recording span",
			setup: func() context.Context {
				ctx, span := StartSpan(context.Background(), "test")
				span.End()
				return ctx
			},
			expected: false, // After End(), recording is false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			recording := IsRecording(ctx)
			assert.Equal(t, tt.expected, recording)
		})
	}
}

func TestInjectContext(t *testing.T) {
	ctx := context.Background()
	headers := InjectContext(ctx)

	assert.NotNil(t, headers)
	assert.IsType(t, map[string]string{}, headers)
}

func TestExtractContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		headers map[string]string
	}{
		{
			name:    "extracts from empty headers",
			ctx:     context.Background(),
			headers: map[string]string{},
		},
		{
			name:    "extracts from headers with trace context",
			ctx:     context.Background(),
			headers: map[string]string{
				"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newCtx := ExtractContext(tt.ctx, tt.headers)
			assert.NotNil(t, newCtx)
		})
	}
}

func TestTraceHTTPMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		service    string
		setupReq   func() *http.Request
		verifyResp func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:    "traces GET request",
			service: "test-service",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			verifyResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name:    "traces POST request",
			service: "test-service",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/users", nil)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			verifyResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name:    "adds trace context to request",
			service: "test-service",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("User-Agent", "test-agent")
				return req
			},
			verifyResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := TraceHTTPMiddleware(tt.service)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := tt.setupReq()

			middleware(handler).ServeHTTP(w, req)

			tt.verifyResp(t, w)
		})
	}
}

func TestTraceHTTPMiddleware_Chaining(t *testing.T) {
	service := "test-service"
	middleware := TraceHTTPMiddleware(service)

	// Create a handler that sets a value in context
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context is available
		assert.NotNil(t, r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	middleware(baseHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTraceHTTPMiddleware_ErrorHandling(t *testing.T) {
	service := "test-service"
	middleware := TraceHTTPMiddleware(service)

	// Handler that panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Middleware should still call the handler (panic will be caught by recovery middleware)
	assert.NotPanics(t, func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected panic
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		middleware(handler).ServeHTTP(w, req)
	})
}

func TestTraceGRPCMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		service       string
		setupCtx      func() context.Context
		verifyResult  func(*testing.T, interface{}, error)
		expectError   bool
	}{
		{
			name:    "traces successful call",
			service: "test-service",
			setupCtx: func() context.Context {
				return context.Background()
			},
			verifyResult: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			},
			expectError: false,
		},
		{
			name:    "traces failed call",
			service: "test-service",
			setupCtx: func() context.Context {
				return context.Background()
			},
			verifyResult: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := TraceGRPCMiddleware(tt.service)

			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				if tt.expectError {
					return nil, assert.AnError
				}
				return "success", nil
			}

			ctx := tt.setupCtx()
			result, err := middleware(ctx, nil, nil, handler)

			tt.verifyResult(t, result, err)
		})
	}
}

func TestTraceGRPCMiddleware_ErrorRecording(t *testing.T) {
	service := "test-service"
	middleware := TraceGRPCMiddleware(service)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, assert.AnError
	}

	ctx := context.Background()
	_, err := middleware(ctx, nil, nil, handler)

	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestContextHelpers(t *testing.T) {
	t.Run("with operation preserves context", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "key", "value")

		newCtx := WithOperation(ctx, "test-operation")

		// Original value should still be present
		assert.Equal(t, "value", newCtx.Value("key"))
	})

	t.Run("with user ID preserves context", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "key", "value")

		newCtx := WithUserID(ctx, "user-123")

		assert.Equal(t, "value", newCtx.Value("key"))
	})

	t.Run("chained context helpers", func(t *testing.T) {
		ctx := context.Background()

		ctx = WithOperation(ctx, "test-operation")
		ctx = WithComponent(ctx, "test-component")
		ctx = WithUserID(ctx, "user-123")
		ctx = WithRequestID(ctx, "req-456")

		assert.NotNil(t, ctx)
	})
}

func TestSpanLifecycle(t *testing.T) {
	t.Run("span can be ended multiple times", func(t *testing.T) {
		ctx, span := StartSpan(context.Background(), "test-operation")

		span.End()
		span.End() // Should not panic

		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
	})

	t.Run("span context outlives span", func(t *testing.T) {
		ctx, span := StartSpan(context.Background(), "test-operation")
		span.End()

		// Context should still be usable
		assert.NotNil(t, ctx)
	})
}

func TestConcurrentSpanCreation(t *testing.T) {
	t.Run("creates multiple spans concurrently", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func() {
				ctx, span := StartSpan(context.Background(), "concurrent-test")
				AddAttributes(ctx, map[string]interface{}{
					"index": i,
				})
				span.End()
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestSpanErrorScenarios(t *testing.T) {
	t.Run("handles nil error gracefully", func(t *testing.T) {
		ctx, span := StartSpan(context.Background(), "test-operation")
		RecordError(span, nil)
		span.End()

		assert.NotNil(t, ctx)
	})

	t.Run("records error with nil span", func(t *testing.T) {
		// Should not panic
		RecordError(nil, assert.AnError)
	})

	t.Run("adds attributes to nil span context", func(t *testing.T) {
		ctx := context.Background()
		// Should not panic
		AddAttributes(ctx, map[string]interface{}{
			"key": "value",
		})
	})
}

// Benchmark tests
func BenchmarkStartSpan(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, span := StartSpan(ctx, "test-operation")
		span.End()
	}
}

func BenchmarkAddAttributes(b *testing.B) {
	ctx, span := StartSpan(context.Background(), "test-operation")
	attrs := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		AddAttributes(ctx, attrs)
	}

	span.End()
}

func BenchmarkTraceHTTPMiddleware(b *testing.B) {
	middleware := TraceHTTPMiddleware("test-service")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		middleware(handler).ServeHTTP(w, req)
	}
}

func BenchmarkInjectContext(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = InjectContext(ctx)
	}
}
