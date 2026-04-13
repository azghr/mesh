package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("DefaultConfig_ValidConfig", func(t *testing.T) {
		cfg := DefaultConfig("test-service")

		if cfg.ServiceName != "test-service" {
			t.Errorf("Expected service name 'test-service', got %s", cfg.ServiceName)
		}

		if cfg.ServiceVersion != "1.0.0" {
			t.Errorf("Expected version '1.0.0', got %s", cfg.ServiceVersion)
		}

		if cfg.Exporter != ExporterStdout {
			t.Errorf("Expected exporter 'stdout', got %s", cfg.Exporter)
		}

		if cfg.SampleRate != 1.0 {
			t.Errorf("Expected sample rate 1.0, got %f", cfg.SampleRate)
		}
	})

	t.Run("DefaultConfig_CustomEnvironment", func(t *testing.T) {
		t.Setenv("ENV", "production")
		cfg := DefaultConfig("test-service")

		if cfg.Environment != "production" {
			t.Errorf("Expected environment 'production', got %s", cfg.Environment)
		}
	})
}

func TestInitTracing(t *testing.T) {
	t.Run("InitTracing_NilConfig", func(t *testing.T) {
		err := InitTracing(nil)
		if err == nil {
			t.Errorf("Expected error for nil config, got nil")
		}
	})

	t.Run("InitTracing_StdoutExporter", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false
		globalTracer = nil
		globalTraceProvider = nil

		cfg := &TraceConfig{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			Exporter:       ExporterStdout,
			SampleRate:     1.0,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		if !isInitialized {
			t.Errorf("Expected isInitialized to be true")
		}

		if globalTracer == nil {
			t.Errorf("Expected global tracer to be set")
		}

		if globalTraceProvider == nil {
			t.Errorf("Expected global trace provider to be set")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})

	t.Run("InitTracing_NoExporter", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false
		globalTracer = nil
		globalTraceProvider = nil

		cfg := &TraceConfig{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Environment:    "test",
			Exporter:       ExporterNone,
			SampleRate:     1.0,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		if !isInitialized {
			t.Errorf("Expected isInitialized to be true")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})

	t.Run("InitTracing_DoubleInit", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		// Try to initialize again
		err = InitTracing(cfg)
		if err == nil {
			t.Errorf("Expected error for double initialization, got nil")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})

	t.Run("InitTracing_InvalidExporter", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterType("invalid"),
		}

		err := InitTracing(cfg)
		if err == nil {
			t.Errorf("Expected error for invalid exporter, got nil")
		}
	})
}

func TestTracer(t *testing.T) {
	t.Run("Tracer_NotInitialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false
		globalTracer = nil

		tracer := Tracer()
		if tracer == nil {
			t.Errorf("Expected tracer to be non-nil (should return no-op tracer)")
		}
	})

	t.Run("Tracer_Initialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false
		globalTracer = nil

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		tracer := Tracer()
		if tracer == nil {
			t.Errorf("Expected tracer to be non-nil")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestStartSpan(t *testing.T) {
	t.Run("StartSpan_NotInitialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		if ctx == nil {
			t.Errorf("Expected context to be non-nil")
		}

		if span == nil {
			t.Errorf("Expected span to be non-nil")
		}

		span.End()
	})

	t.Run("StartSpan_Initialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		if ctx == nil {
			t.Errorf("Expected context to be non-nil")
		}

		if span == nil {
			t.Errorf("Expected span to be non-nil")
		}

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestAddSpanAttributes(t *testing.T) {
	t.Run("AddSpanAttributes", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		// This should not panic
		AddSpanAttributes(ctx,
			attribute.String("key1", "value1"),
			attribute.Int("key2", 42),
		)

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestAddSpanEvent(t *testing.T) {
	t.Run("AddSpanEvent", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		// This should not panic
		AddSpanEvent(ctx, "test-event",
			attribute.String("event_key", "event_value"),
		)

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestRecordError(t *testing.T) {
	t.Run("RecordError_NilError", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		// This should not panic
		RecordError(ctx, nil)

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})

	t.Run("RecordError_WithError", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		testErr := context.DeadlineExceeded
		// This should not panic
		RecordError(ctx, testErr)

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestSetSpanStatus(t *testing.T) {
	t.Run("SetSpanStatus", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")

		// This should not panic
		SetSpanStatus(ctx, codes.Ok, "success")

		span.End()

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestWithSpan(t *testing.T) {
	t.Run("WithSpan_Success", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		err = WithSpan(ctx, "test-operation", func(ctx context.Context) error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})

	t.Run("WithSpan_WithError", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		expectedErr := context.Canceled
		err = WithSpan(ctx, "test-operation", func(ctx context.Context) error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("IsInitialized_False", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		if IsInitialized() {
			t.Errorf("Expected IsInitialized to be false")
		}
	})

	t.Run("IsInitialized_True", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		if !IsInitialized() {
			t.Errorf("Expected IsInitialized to be true")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestGetTraceID(t *testing.T) {
	t.Run("GetTraceID_NoSpan", func(t *testing.T) {
		ctx := context.Background()
		traceID := GetTraceID(ctx)

		if traceID != "" {
			t.Errorf("Expected empty trace ID for context without span, got %s", traceID)
		}
	})

	t.Run("GetTraceID_WithSpan", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")
		traceID := GetTraceID(ctx)
		span.End()

		if traceID == "" {
			t.Errorf("Expected non-empty trace ID for context with span")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestGetSpanID(t *testing.T) {
	t.Run("GetSpanID_NoSpan", func(t *testing.T) {
		ctx := context.Background()
		spanID := GetSpanID(ctx)

		if spanID != "" {
			t.Errorf("Expected empty span ID for context without span, got %s", spanID)
		}
	})

	t.Run("GetSpanID_WithSpan", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx := context.Background()
		ctx, span := StartSpan(ctx, "test-span")
		spanID := GetSpanID(ctx)
		span.End()

		if spanID == "" {
			t.Errorf("Expected non-empty span ID for context with span")
		}

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ShutdownTracing(ctx)
		isInitialized = false
	})
}

func TestShutdownTracing(t *testing.T) {
	t.Run("ShutdownTracing_NotInitialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false
		globalTraceProvider = nil

		ctx := context.Background()
		err := ShutdownTracing(ctx)

		if err != nil {
			t.Errorf("Expected no error when shutting down uninitialized tracing, got %v", err)
		}
	})

	t.Run("ShutdownTracing_Initialized", func(t *testing.T) {
		// Reset for clean test
		isInitialized = false

		cfg := &TraceConfig{
			ServiceName: "test-service",
			Exporter:    ExporterNone,
		}

		err := InitTracing(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize tracing: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = ShutdownTracing(ctx)
		if err != nil {
			t.Errorf("Expected no error when shutting down tracing, got %v", err)
		}

		isInitialized = false
	})
}
