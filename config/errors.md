# errors

Structured error handling that maps cleanly to HTTP/gRPC responses.

## What It Does

Defines `AppError`, a rich error type that carries error type, code, message, metadata, and context. Automatically converts to HTTP status codes and gRPC status codes.

## Usage

### Creating Errors

```go
// Simple errors
err := errors.New(ErrorTypeNotFound, "NOT_FOUND", "user not found")
err := errors.NotFoundError("user", "123")

// With details
err := errors.NewWithDetails(ErrorTypeValidation, "INVALID_EMAIL", "Invalid email", "must contain @")

// Wrapping existing errors
err := errors.DatabaseError("SELECT users", originalErr)
err := errors.ExternalServiceError("stripe", "create_charge", originalErr)
```

### HTTP Mapping

```go
// In your HTTP handler
if err != nil {
    status := err.ToHTTPStatus()
    json.NewEncoder(w).Encode(map[string]any{
        "error": err.Message,
        "type":  err.Type,
        "code":  err.Code,
    })
    w.WriteHeader(status)
    return
}
```

Maps to these HTTP status codes:

| Error Type | HTTP Status |
|------------|-------------|
| Validation | 400 Bad Request |
| NotFound | 404 Not Found |
| Conflict | 409 Conflict |
| Unauthorized | 401 Unauthorized |
| Forbidden | 403 Forbidden |
| Timeout | 408 Request Timeout |
| RateLimit | 429 Too Many Requests |
| Database | 500 Internal Server Error |
| Internal | 500 Internal Server Error |
| External | 502 Bad Gateway |

### gRPC Mapping

```go
// For gRPC services
status := err.ToGRPCStatus()
// Returns google.golang.org/grpc/status.Status
```

| Error Type | gRPC Code |
|------------|-----------|
| Validation | InvalidArgument |
| NotFound | NotFound |
| Conflict | AlreadyExists |
| Unauthorized | Unauthenticated |
| Forbidden | PermissionDenied |
| Timeout | DeadlineExceeded |
| RateLimit | ResourceExhausted |
| Database | Internal |
| Internal | Internal |
| External | Unavailable |

### Adding Context

```go
err := errors.NotFoundError("order", "123").
    WithRequestID(requestID).
    WithUserID(userID).
    WithField("resource_type", "order")
```

### Error Checking

```go
// Check if an error is an AppError
if errors.IsAppError(err) {
    appErr, _ := errors.GetAppError(err)
    log.Println(appErr.Type, appErr.Code)
}

// Check error type in handlers
if err, ok := err.(*errors.AppError); ok && err.Type == errors.ErrorTypeNotFound {
    // Handle not found specifically
}
```

### Context Extraction

```go
// Create error with context from request
err := errors.FromContext(ctx, ErrorTypeNotFound, "NOT_FOUND", "resource not found")
// Automatically pulls request_id and user_id from context if present
```

### Error Logging

```go
// Log error with appropriate level based on type
err.LogError(logger)
// Validation/NotFound/Warn -> Warn
// Database/Internal/External -> Error
```