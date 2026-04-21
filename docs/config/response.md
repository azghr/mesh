# response

Standardized HTTP response helpers for API endpoints.

## What It Does

Provides helpers for building consistent API responses:
- Generic Response[T] type for type-safe responses
- Success and error response helpers
- Pagination metadata support
- Integration with errors package

## Usage

### Basic Success

```go
response.Success(w, user)
// {"data":{"id":"1","name":"john"}}
```

### With Pagination Metadata

```go
users, _ := db.ListUsers(ctx, offset, limit)
total, _ := db.CountUsers(ctx)

response.SuccessWithMeta(w, users, total, page, perPage)
// {"data":[...],"meta":{"total":100,"page":1,"per_page":10}}
```

### Created (201)

```go
response.Created(w, newUser)
```

### No Content (204)

```go
response.NoContent(w)
```

### Error Responses

```go
// From errors package
err := errors.NotFoundError("user", "123")
response.Error(w, err)
// {"error":{"code":"not_found","message":"user not found"}}

// Custom message
response.BadRequest(w, "invalid input")
response.NotFound(w, "user not found")
response.Unauthorized(w, "authentication required")
response.Forbidden(w, "access denied")
response.Conflict(w, "resource already exists")
response.InternalError(w, "internal server error")
```

### With Options

```go
response.Success(w, data, 
    response.WithStatus(http.StatusAccepted),
    response.WithHeader("X-Request-ID", "123"))
```

## Response Format

### Success

```json
{
  "data": {...},
  "meta": {
    "total": 100,
    "page": 1,
    "per_page": 10
  }
}
```

### Error

```json
{
  "error": {
    "code": "not_found",
    "message": "user not found"
  }
}
```

## Response Types

### Generic Response[T]

```go
type Response[T any] struct {
    Data  T     `json:"data,omitempty"`
    Error *Err  `json:"error,omitempty"`
    Meta  *Meta `json:"meta,omitempty"`
}
```

### With AppError Integration

```go
err := errors.NotFoundError("user", "123")
response.Error(w, err)
// Automatically maps error type to HTTP status
```

### Error Types

| Error | HTTP Status |
|-------|-------------|
| ValidationError | 400 |
| NotFoundError | 404 |
| ConflictError | 409 |
| UnauthorizedError | 401 |
| ForbiddenError | 403 |
| InternalError | 500 |

## Example: Handler

```go
func GetUsers(w http.ResponseWriter, r *http.Request) {
    params, _ := paginator.FromRequest(r, 20)
    
    users, err := db.ListUsers(ctx, params.Offset(), params.Limit())
    if err != nil {
        response.Error(w, err)
        return
    }
    
    total, err := db.CountUsers(ctx)
    if err != nil {
        response.Error(w, errors.InternalError(err.Error()))
        return
    }
    
    response.SuccessWithMeta(w, users, total, params.Page(), params.Limit())
}
```

## Example: With Custom Headers

```go
response.Created(w, user, 
    response.WithHeader("Location", "/users/"+user.ID))
```

## Options

| Option | Description |
|--------|-------------|
| WithStatus(code) | Set custom HTTP status |
| WithHeader(key, value) | Add response header |

## Using with Fiber

```go
app.Get("/users", func(c *fiber.Ctx) error {
    users, err := db.ListUsers(c.UserContext())
    if err != nil {
        return response.Error(c, err)
    }
    
    return c.JSON(users) // Fiber handles JSON internally
})
```

## Custom Error Codes

```go
response.ErrorWithCode(w, http.StatusTooManyRequests, "rate limit exceeded")
```