# result

Generic Result type for explicit error handling.

## What It Does

Provides a Result type similar to Rust's Option/Result - makes error handling explicit rather than panicking. Useful for:
- Cleaner error handling without try/catch
- Chainable operations
- Async operations with concurrent execution

## Basic Usage

```go
// Create success result
r := result.Ok(42)

// Create error result
r := result.Err[int](errors.New("failed"))

// Check status
if r.IsOk() {
    value := r.Value()
}

// Get value or default
value := r.UnwrapOr(0)

// Get value or error
value, err := r.Unwrap()
```

## Chaining

```go
// Map transforms value
r.Map(func(v int) int { return v * 2 })

// ChainResult with another operation
result.AndThen(func(u User) Result[Order] {
    return fetchOrder(ctx, u.OrderID)
})
```

## Async Operations

```go
// Start async operation
async := result.Async(func() (User, error) {
    return findUser(ctx, id)
})

// Non-blocking check
if async.IsReady() {
    user, err := async.Get()
}

// Or block with timeout
user, err := async.GetWithTimeout(ctx, 5*time.Second)

// Run multiple concurrently
results := result.AsyncAll(func() (User, error) { return findUser(ctx, id1) },
                            func() (User, error) { return findUser(ctx, id2) })

users, err := result.Collect(ctx, results)
```

## Error Handling

```go
// Tap executes function if ok (logging, etc.)
r.Tap(func(v int) { log.Info("value", v) })

// TapErr executes function if error
r.TapErr(func(err error) { log.Error(err) })

// MapErr transforms error
r.MapErr(func(err error) error {
    return fmt.Errorf("failed: %w", err)
})
```