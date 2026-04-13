package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	cfg "github.com/azghr/mesh/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	globalTracer        trace.Tracer
	globalTraceProvider *sdktrace.TracerProvider
	isInitialized       bool
	shutdownFunc        func()
)

// TraceConfig holds configuration for tracing
type TraceConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Exporter       ExporterType
	OTLPEndpoint   string
	SampleRate     float64
	TLSEnabled     bool
	TLSConfig      *tls.Config
}

// ExporterType defines the type of trace exporter
type ExporterType string

const (
	ExporterNone   ExporterType = "none"
	ExporterStdout ExporterType = "stdout"
	ExporterOTLP   ExporterType = "otlp"
)

// DefaultConfig returns a default trace configuration
func DefaultConfig(serviceName string) *TraceConfig {
	return &TraceConfig{
		ServiceName:    serviceName,
		ServiceVersion: "1.0.0",
		Environment:    cfg.GetEnv("ENV", "development"),
		Exporter:       ExporterStdout,
		OTLPEndpoint:   cfg.GetEnv("OTLP_ENDPOINT", "localhost:4317"),
		SampleRate:     1.0, // Sample all traces in development
	}
}

// InitTracing initializes the OpenTelemetry tracing
func InitTracing(cfg *TraceConfig) error {
	if isInitialized {
		return fmt.Errorf("tracing already initialized")
	}

	if cfg == nil {
		return fmt.Errorf("trace config cannot be nil")
	}

	// Validate exporter type
	if cfg.Exporter == "" {
		cfg.Exporter = ExporterStdout
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case ExporterStdout:
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return fmt.Errorf("failed to create stdout exporter: %w", err)
		}

	case ExporterOTLP:
		if cfg.OTLPEndpoint == "" {
			cfg.OTLPEndpoint = "localhost:4317"
		}
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.TLSEnabled {
			var grpcOpts []grpc.DialOption
			if cfg.TLSConfig != nil {
				grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(cfg.TLSConfig)))
			} else {
				grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
			}
			opts = append(opts, otlptracegrpc.WithDialOption(grpcOpts...))
		} else {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(context.Background(), opts...)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}

	case ExporterNone:
		// No exporter, disabled tracing
		exporter = nil

	default:
		return fmt.Errorf("unsupported exporter type: %s", cfg.Exporter)
	}

	// Create trace provider
	var providerOpts []sdktrace.TracerProviderOption
	if exporter != nil {
		// Add batch span processor with exporter
		providerOpts = append(providerOpts,
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)

		// Configure sampling
		if cfg.SampleRate >= 0.0 && cfg.SampleRate <= 1.0 {
			providerOpts = append(providerOpts,
				sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
			)
		}
	}

	globalTraceProvider = sdktrace.NewTracerProvider(providerOpts...)

	// Set global trace provider
	otel.SetTracerProvider(globalTraceProvider)

	// Set global propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer
	globalTracer = globalTraceProvider.Tracer(
		cfg.ServiceName,
		trace.WithInstrumentationVersion(cfg.ServiceVersion),
	)

	isInitialized = true
	return nil
}

// ShutdownTracing shuts down the trace provider and flushes remaining spans
func ShutdownTracing(ctx context.Context) error {
	if !isInitialized || globalTraceProvider == nil {
		return nil
	}

	return globalTraceProvider.Shutdown(ctx)
}

// Tracer returns the global tracer instance
func Tracer() trace.Tracer {
	if !isInitialized {
		// Return a no-op tracer if not initialized
		return trace.NewNoopTracerProvider().Tracer("")
	}
	return globalTracer
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err, opts...)
	}
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(code, description)
	}
}

// WithSpan creates a span and runs the given function within it
func WithSpan(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, name)
	defer span.End()

	if err := fn(ctx); err != nil {
		RecordError(ctx, err)
		return err
	}

	return nil
}

// IsInitialized returns whether tracing has been initialized
func IsInitialized() bool {
	return isInitialized
}

// GetTraceID returns the trace ID from the context if available
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the context if available
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// StartSpanWithAttributes creates a new span with attributes
func StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]interface{}) (context.Context, trace.Span) {
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attributes = append(attributes, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	return StartSpan(ctx, name, trace.WithAttributes(attributes...))
}

// WithOperation returns a context with an operation name attribute
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey("operation"), operation)
}

// WithComponent returns a context with a component name attribute
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, operationKey("component"), component)
}

// WithUserID returns a context with a user ID attribute
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, operationKey("user_id"), userID)
}

// WithRequestID returns a context with a request ID attribute
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, operationKey("request_id"), requestID)
}

type operationKey string

func (operationKey) String() string { return string(operationKey("")) }

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
					attribute.String("service.name", serviceName),
				),
			)
			defer span.End()

			r = r.WithContext(ctx)
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
		methodName := "unknown"
		if info != nil {
			methodName = info.FullMethod
		}

		ctx, span := StartSpan(ctx, methodName,
			trace.WithAttributes(
				attribute.String("rpc.method", methodName),
				attribute.String("rpc.service", serviceName),
				attribute.String("service.name", serviceName),
			),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			RecordError(ctx, err)
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
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}

// IsRecording returns whether the current span is recording
func IsRecording(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	return span.IsRecording()
}

// AddAttributes adds attributes to the current span in the context
func AddAttributes(ctx context.Context, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
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
	if !span.IsRecording() {
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
