# retry

Retry logic with exponential backoff for database operations and external services.

## What It Does

Provides automatic retry with configurable backoff:
- Configurable max attempts
- Multiple backoff strategies (constant, linear, exponential)
- Jitter support for distributed systems
- Context cancellation support
- Error filtering

## Usage

### Basic Retry

```go
err := retry.Do(ctx, func() error {
    return db.Tx(ctx, func(tx *sql.Tx) error {
        // operation
    })
}, retry.WithMaxAttempts(3))
```

### Exponential Backoff (Default)

```go
err := retry.Do(ctx, fn, retry.WithMaxAttempts(5), 
    retry.WithBaseDelay(100*time.Millisecond))
```

### Linear Backoff

```go
err := retry.Do(ctx, fn, 
    retry.WithBackoff(retry.Linear),
    retry.WithBaseDelay(50*time.Millisecond))
```

### With Jitter

Prevents "thundering herd" when multiple clients retry:

```go
err := retry.Do(ctx, fn, retry.WithJitter())
```

### Custom Retry Condition

```go
err := retry.Do(ctx, fn, retry.WithRetryIf(func(e error) bool {
    // Only retry on network errors
    return strings.Contains(e.Error(), "connection")
}))
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxAttempts` | 3 | Max retry attempts |
| `WithBaseDelay` | 100ms | Base delay |
| `WithMaxDelay` | 30s | Cap delay |
| `WithBackoff` | Exponential | Strategy |
| `WithJitter` | true | Enable jitter |
| `WithRetryIf` | DefaultRetryIf | Retry filter |

## Backoff Strategies

### Constant

```go
// Always delays by base amount
retry.WithBackoff(retry.Constant)
// Delays: 100ms, 100ms, 100ms
```

### Linear

```go
// Increases linearly
retry.WithBackoff(retry.Linear)
// Delays: 100ms, 200ms, 300ms
```

### Exponential (Default)

```go
// Increases exponentially
retry.WithBackoff(retry.Exponential)
// Delays: 100ms, 200ms, 400ms
```

## With Result

For functions that return a value:

```go
user, err := retry.DoWithResult(ctx, func() (User, error) {
    return db.GetUser(ctx, id)
}, retry.WithMaxAttempts(3))
```

## Shortcuts

### Simple Retry

```go
// Retry up to N times
err := retry.Retry(ctx, 3, fn)
```

### Forever

```go
// Retry until success (until context cancels)
err := retry.Forever(ctx, fn)
```

## Context Support

The retry respects context cancellation:

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

err := retry.Do(ctx, fn, retry.WithMaxAttempts(10))
// Returns context.DeadlineExceeded if timeout
```

## Error Handling

### Default Behavior

- Retries on any error except:
  - `context.Canceled`
  - `context.DeadlineExceeded`

### Custom Filter

```go
err := retry.Do(ctx, fn, retry.WithRetryIf(func(e error) bool {
    var perr *mysql.MySQLError
    if errors.As(e, &perr) {
        // Retry on deadlock (1213) or lost connection (2006)
        return perr.Number == 1213 || perr.Number == 2006
    }
    return true
}))
```

## Examples

### Database Transaction

```go
func withRetry(ctx context.Context, fn func(*sql.Tx) error) error {
    return retry.Do(ctx, func() error {
        return db.WithTx(ctx, fn)
    }, retry.WithMaxAttempts(3), 
        retry.WithBaseDelay(50*time.Millisecond),
        retry.WithMaxAttempts(3))
}
```

### HTTP Call

```go
err := retry.Do(ctx, func() error {
    resp, err := httpclient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 500 {
        return errors.New("server error")
    }
    return nil
}, retry.WithMaxAttempts(3), retry.WithExponential())
```

### External API

```go
err := retry.Do(ctx, func() error {
    return callAPI(ctx, endpoint, payload)
}, retry.WithMaxAttempts(5),
    retry.WithBaseDelay(200*time.Millisecond),
    retry.WithMaxDelay(10*time.Second),
    retry.WithJitter())
```