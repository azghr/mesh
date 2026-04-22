# result

A generic Result type for explicit error handling, similar to Rust's Result type.

## What It Does

Provides a cleaner way to handle errors without panics. Encourages explicit error checking with chainable methods for common patterns.

## Usage

### Basic Usage

```go
result := result.Ok(42)
value, err := result.Unwrap()

result := result.Err[int](errors.New("failed"))
value := result.UnwrapOr(0)
```

### Creating Results

```go
// Success result
r := result.Ok(42)

// Error result
r := result.Err[int](errors.New("failed"))
```

### Checking Status

```go
if r.IsOk() {
    fmt.Println(r.Value())
}

if r.IsErr() {
    fmt.Println(r.UnwrapErr())
}
```

### Unwrapping

```go
// Unwrap or panic
value := r.Unwrap()

// Unwrap or default
value := r.UnwrapOr(0)

// Unwrap or compute
value := r.UnwrapOrElse(func() int { return computeDefault() })
```

### Transformation

```go
// Map success value
mapped := r.Map(func(v int) int { return v * 2 })

// Map error
remapped := r.MapErr(func(e error) error { return fmt.Errorf("wrapped: %w", e) })
```

### Chaining

```go
r := getUser(ctx, id).
    And(func() result.Result[Order] { return getOrder(ctx, user.OrderID) }).
    Map(func(o Order) Order { o.Total = calculateTotal(o.Items); return o })
```

### Async Operations

```go
// Non-blocking async execution
r := result.Async(func() (User, error) {
    return findUser(ctx, id)
})

// Check if ready
if r.IsReady() {
    user, err := r.Get()
}
```

## Type Parameters

Works with any type:

```go
result.Ok[string]("hello")
result.Err[User](errors.New("not found"))
result.Ok[map[string]int]{"a": 1}
```