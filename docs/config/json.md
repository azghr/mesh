# JSON

Fast JSON encoding and decoding using goccy/go-json.

## What It Does

Provides 2-10x faster JSON encoding compared to standard library. Drop-in replacement for encoding/json.

## Quick Start

```go
import "github.com/azghr/mesh/json"

// Replace encoding/json usage
data, err := json.Marshal(v)
err := json.Unmarshal(data, &v)
```

## Performance

Benchmarks show significant improvements:

| Operation | Standard | Fast JSON | Speedup |
|----------|----------|---------|--------|
| Marshal | 1.0x | 2-5x | Faster |
| Unmarshal | 1.0x | 3-10x | Faster |

Benefits:
- No reflection used - consistent performance
- Deterministic output - stable ordering
- Lower CPU usage in high-throughput services

## Usage

### Basic Operations

```go
// Marshal to bytes
data, err := json.Marshal(user)

// Marshal to string
str, err := json.MarshalToString(user)

// Unmarshal
err := json.Unmarshal(data, &user)

// Marshal with indentation
data, err := json.MarshalIndent(user, "", "  ")
```

### Streaming

```go
// Encoder for streaming output
enc := json.NewEncoder(os.Stdout)
err := enc.Encode(data)

// Decoder for streaming input
dec := json.NewDecoder(os.Stdin)
err := dec.Decode(&data)
```

### Fiber Integration

Works with Fiber's JSON methods:

```go
// Use in HTTP handlers
app.Get("/users", func(c *fiber.Ctx) error {
    users, err := getUsers()
    if err != nil {
        return err
    }
    return c.JSON(users) // Fiber uses json.Marshal internally
})
```

## Compatibility

API is compatible with encoding/json. Replace imports:

```go
// Before (standard library)
import "encoding/json"

// After (fast JSON)
import "github.com/azghr/mesh/json"
```

Most code works without modification.

## Best Practices

- Use for high-throughput services
- Replace encoding/json in cache serialization
- Use for eventbus message encoding
- Monitor CPU usage after switching - should decrease