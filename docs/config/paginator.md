# paginator

Standardized pagination for HTTP list endpoints.

## What It Does

Provides a clean way to implement consistent pagination across your API endpoints:
- Parses query parameters (`page`, `per_page`)
- Calculates offset/limit for database queries
- Generates pagination metadata for responses
- Handles default values and bounds checking

## Usage

### Basic Setup

```go
params, err := paginator.FromRequest(r, 20) // 20 is default limit
if err != nil {
    return errors.BadRequest(err)
}
```

### With Database Queries

```go
// Parse pagination params
params, err := paginator.FromRequest(r, 20)
if err != nil {
    return errors.BadRequest(err)
}

// Fetch paginated data
users, err := db.ListUsers(ctx, params.Offset(), params.Limit())
if err != nil {
    return err
}

// Get total count for metadata
total, err := db.CountUsers(ctx)
if err != nil {
    return err
}

// Return with metadata
return response.Success(w, users, paginator.NewMeta(total, params.Page(), params.Limit()))
```

### Query Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | 1 | Page number (1-indexed) |
| `per_page` | 20 | Items per page (max 100) |

Example: `/users?page=2&per_page=25`

## Response Format

```json
{
  "data": [...],
  "meta": {
    "total": 150,
    "page": 2,
    "per_page": 25,
    "total_pages": 6
  }
}
```

### Metadata Helpers

```go
meta := paginator.NewMeta(total, page, perPage)

// Check navigation
meta.HasNext()     // true if not on last page
meta.HasPrevious() // true if not on first page
```

### Using with Fiber

```go
app.Get("/users", func(c *fiber.Ctx) error {
    params, err := paginator.FromURL(c.Request().URI())
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }
    
    users, total := getUsers(params.Offset(), params.Limit())
    
    return c.JSON(fiber.Map{
        "data": users,
        "meta": paginator.NewMeta(total, params.Page(), params.Limit()),
    })
})
```

## Configuration

### Default Limit

```go
// Use default (20)
params, _ := paginator.FromRequest(r)

// Custom default
params, _ := paginator.FromRequest(r, 50)
```

### Constants

```go
paginator.DefaultPage  // 1
paginator.DefaultLimit // 20
paginator.MaxLimit    // 100
```

## Error Handling

```go
params, err := paginator.FromRequest(r)
if errors.Is(err, paginator.ErrInvalidPage) {
    return errors.BadRequest("page must be >= 1")
}
if errors.Is(err, paginator.ErrInvalidLimit) {
    return errors.BadRequest("invalid per_page value")
}
```
