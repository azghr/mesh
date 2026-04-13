package telemetry

import (
	"context"
	"fmt"

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
)

var (
	globalTracer        trace.Tracer
	globalTraceProvider *sdktrace.TracerProvider
	isInitialized       bool
)

// TraceConfig holds configuration for tracing
type TraceConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string // development, staging, production
	Exporter       ExporterType
	OTLPEndpoint   string  // OTLP endpoint (e.g., "localhost:4317")
	SampleRate     float64 // Sampling rate (0.0 to 1.0)
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
		exporter, err = otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		)
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
