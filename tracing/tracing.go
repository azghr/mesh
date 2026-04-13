// Package tracing provides distributed tracing utilities.
//
// Deprecated: Use github.com/azghr/mesh/telemetry instead.
// This package is kept for backwards compatibility and will be removed in a future release.
package tracing

import (
	"context"
	"net/http"

	"github.com/azghr/mesh/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Config holds the tracing configuration
type Config = telemetry.TraceConfig

// DefaultConfig returns a tracing configuration with sensible defaults
func DefaultConfig(serviceName string) *telemetry.TraceConfig {
	return telemetry.DefaultConfig(serviceName)
}

// Setup initializes the OpenTelemetry tracing system
// Returns a context with the tracer setup and a cancellation function
//
// Deprecated: Use telemetry.InitTracing instead.
func Setup(cfg *telemetry.TraceConfig) (context.Context, func()) {
	if err := telemetry.InitTracing(cfg); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return ctx, func() {
		telemetry.ShutdownTracing(context.Background())
		cancel()
	}
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
	return telemetry.Tracer()
}

// StartSpan creates a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return telemetry.StartSpan(ctx, name, opts...)
}

// StartSpanWithAttributes creates a new span with attributes
func StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]interface{}) (context.Context, trace.Span) {
	return telemetry.StartSpanWithAttributes(ctx, name, attrs)
}

// RecordError records an error on the span
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
}

// RecordErrorWithMessage records an error with a custom message on the span
func RecordErrorWithMessage(span trace.Span, err error, message string) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.message", message),
	))
}

// AddAttributes adds attributes to the current span in the context
func AddAttributes(ctx context.Context, attrs map[string]interface{}) {
	telemetry.AddAttributes(ctx, attrs)
}

// AddEvent adds an event to the current span in the context
func AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	telemetry.AddEvent(ctx, name, attrs)
}

// WithOperation returns a context with an operation name attribute
func WithOperation(ctx context.Context, operation string) context.Context {
	return telemetry.WithOperation(ctx, operation)
}

// WithComponent returns a context with a component name attribute
func WithComponent(ctx context.Context, component string) context.Context {
	return telemetry.WithComponent(ctx, component)
}

// WithUserID returns a context with a user ID attribute
func WithUserID(ctx context.Context, userID string) context.Context {
	return telemetry.WithUserID(ctx, userID)
}

// WithRequestID returns a context with a request ID attribute
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return telemetry.WithRequestID(ctx, requestID)
}

// WithAttribute returns a context with a custom attribute
func WithAttribute(ctx context.Context, key string, value interface{}) context.Context {
	return context.WithValue(ctx, struct{}{}, map[string]interface{}{key: value})
}

// TraceHTTPMiddleware returns a middleware that traces HTTP requests
func TraceHTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return telemetry.TraceHTTPMiddleware(serviceName)
}

// TraceGRPCMiddleware returns a gRPC unary interceptor that traces RPC calls
func TraceGRPCMiddleware(serviceName string) grpc.UnaryServerInterceptor {
	return telemetry.TraceGRPCMiddleware(serviceName)
}

// InjectContext injects trace context into a map for propagation
func InjectContext(ctx context.Context) map[string]string {
	return telemetry.InjectContext(ctx)
}

// ExtractContext extracts trace context from a map
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	return telemetry.ExtractContext(ctx, headers)
}

// GetTraceID returns the trace ID from the context
func GetTraceID(ctx context.Context) string {
	return telemetry.GetTraceID(ctx)
}

// GetSpanID returns the span ID from the context
func GetSpanID(ctx context.Context) string {
	return telemetry.GetSpanID(ctx)
}

// IsRecording returns whether the current span is recording
func IsRecording(ctx context.Context) bool {
	return telemetry.IsRecording(ctx)
}
