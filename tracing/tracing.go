// Package tracing provides distributed tracing utilities for all Lumex services.
//
// This package integrates with OpenTelemetry to provide automatic trace
// propagation, span creation, and context management across service boundaries.
//
// Example:
//
//	ctx, cancel := tracing.Setup("my-service")
//	defer cancel()
//
//	ctx, span := tracing.StartSpan(ctx, "operation-name")
//	defer span.End()
//
//	// Your code here
//
//	tracing.RecordError(span, err)
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

var (
	tracerProvider *sdktrace.TracerProvider
	tracer        trace.Tracer
)

// Config holds the tracing configuration
type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	CollectorEndpoint string
	SampleRate      float64
}

// DefaultConfig returns a tracing configuration with sensible defaults
func DefaultConfig(serviceName string) Config {
	return Config{
		ServiceName:      serviceName,
		ServiceVersion:   "1.0.0",
		Environment:      "production",
		CollectorEndpoint: "otel-collector:4317",
		SampleRate:       1.0, // Sample all traces in production
	}
}

// Setup initializes the OpenTelemetry tracing system
// Returns a context with the tracer setup and a cancellation function
func Setup(cfg Config) (context.Context, func()) {
	// Create a resource describing this service
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
			attribute.String("cluster", "lumex"),
		),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create resource: %w", err))
	}

	// Set up the trace exporter
	traceExporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.CollectorEndpoint),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create trace exporter: %w", err))
	}

	// Create a span processor
	batchSpanProcessor := sdktrace.NewBatchSpanProcessor(
		traceExporter,
		sdktrace.WithBatchTimeout(5*time.Second),
		sdktrace.WithMaxExportBatchSize(1000),
		sdktrace.WithMaxQueueSize(1000),
	)

	// Create a tracer provider
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
		sdktrace.WithSpanProcessor(batchSpanProcessor),
	)

	// Register the tracer provider globally
	otel.SetTracerProvider(tracerProvider)

	// Set up global propagators for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Set up global tracer
	tracer = otel.Tracer(cfg.ServiceName)

	ctx, cancel := context.WithCancel(context.Background())

	return ctx, func() {
		// Shutdown the tracer provider when the service stops
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			panic(fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
		cancel()
	}
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
	if tracer == nil {
		return otel.Tracer("lumex")
	}
	return tracer
}

// StartSpan creates a new span with the given name
// Returns a context with the span and the span itself
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := GetTracer()
	return tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes creates a new span with attributes
func StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]interface{}) (context.Context, trace.Span) {
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	return StartSpan(ctx, name, trace.WithAttributes(attributes...))
}

// RecordError records an error on the span
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", err.Error()),
	))
}

// RecordErrorWithMessage records an error with a custom message on the span
func RecordErrorWithMessage(span trace.Span, err error, message string) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.message", message),
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.details", err.Error()),
	))
}

// AddAttributes adds attributes to the current span in the context
func AddAttributes(ctx context.Context, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}

	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.SetAttributes(attributes...)
}

// AddEvent adds an event to the current span in the context
func AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}

	options := []trace.EventOption{}
	if len(attrs) > 0 {
		attributes := make([]attribute.KeyValue, 0, len(attrs))
		for k, v := range attrs {
			attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
		}
		options = append(options, trace.WithAttributes(attributes...))
	}

	span.AddEvent(name, options...)
}

// WithOperation returns a context with an operation name attribute
func WithOperation(ctx context.Context, operation string) context.Context {
	return WithAttribute(ctx, "operation", operation)
}

// WithComponent returns a context with a component name attribute
func WithComponent(ctx context.Context, component string) context.Context {
	return WithAttribute(ctx, "component", component)
}

// WithUserID returns a context with a user ID attribute
func WithUserID(ctx context.Context, userID string) context.Context {
	return WithAttribute(ctx, "user_id", userID)
}

// WithRequestID returns a context with a request ID attribute
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return WithAttribute(ctx, "request_id", requestID)
}

// WithAttribute returns a context with a custom attribute
func WithAttribute(ctx context.Context, key string, value interface{}) context.Context {
	return trace.ContextWithSpan(ctx, trace.SpanFromContext(ctx))
}

// TraceHTTPMiddleware returns a middleware that traces HTTP requests
func TraceHTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := StartSpan(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.host", r.Host),
					attribute.String("http.remote_addr", r.RemoteAddr),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			// Add span context to request
			r = r.WithContext(ctx)

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// TraceGRPCMiddleware returns a gRPC unary interceptor that traces RPC calls
func TraceGRPCMiddleware(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract method name safely
		methodName := "unknown"
		if info != nil {
			methodName = info.FullMethod
		}

		ctx, span := StartSpan(ctx, methodName,
			trace.WithAttributes(
				attribute.String("grpc.method", methodName),
				attribute.String("grpc.service", serviceName),
			),
		)
		defer span.End()

		// Call the handler
		resp, err := handler(ctx, req)
		if err != nil {
			RecordError(span, err)
		}

		return resp, err
	}
}

// InjectContext injects trace context into a map for propagation
func InjectContext(ctx context.Context) map[string]string {
	headers := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
	return headers
}

// ExtractContext extracts trace context from a map
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
	return ctx
}

// GetTraceID returns the trace ID from the context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return ""
	}
	return spanCtx.TraceID().String()
}

// GetSpanID returns the span ID from the context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return ""
	}
	return spanCtx.SpanID().String()
}

// IsRecording returns whether the current span is recording
func IsRecording(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return false
	}
	return span.IsRecording()
}
