# tracing

Distributed tracing utilities.

## Status

**Deprecated**: Use `telemetry` package instead. This package is kept for backwards compatibility and will be removed in a future release.

## Migration

Replace:

```go
import "github.com/azghr/mesh/tracing"

// Setup
ctx, cancel := tracing.Setup(tracing.DefaultConfig("my-service"))

// Spans
ctx, span := tracing.StartSpan(ctx, "operation")
defer span.End()
```

With:

```go
import "github.com/azghr/mesh/telemetry"

// Setup
telemetry.InitTracing(telemetry.DefaultConfig("my-service"))

// Spans
ctx, span := telemetry.StartSpan(ctx, "operation")
defer span.End()
```

All types and functions are aliases to `telemetry` package equivalents.