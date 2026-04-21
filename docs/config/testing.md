# testing

Testing utilities for HTTP handlers and common test patterns.

## What It Does

Provides helpers for testing HTTP handlers:
- Response status and body assertions
- JSON comparison (order-independent)
- HTTP handler testing helpers
- Stub helpers for DB and cache

## Usage

### Testing HTTP Handlers

```go
func TestCreateUser(t *testing.T) {
    handler := func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusCreated)
        w.Write([]byte(`{"id":"123"}`))
    }

    rr := testing.RunHandler(t, handler, http.MethodPost, "/users", `{"name":"john"}`)

    testing.AssertStatus(t, rr.Code, http.StatusCreated)
}
```

### Status Assertions

```go
testing.AssertStatus(t, rr.Code, http.StatusOK)
testing.AssertStatusOK(t, rr.Code)
testing.AssertStatusCreated(t, rr.Code)
testing.AssertStatusBadRequest(t, rr.Code)
testing.AssertStatusNotFound(t, rr.Code)
testing.AssertStatusUnauthorized(t, rr.Code)
testing.AssertStatusForbidden(t, rr.Code)
testing.AssertStatusInternalError(t, rr.Code)
```

### JSON Assertions

```go
// Order-independent JSON comparison
testing.AssertJSON(t, rr.Body.String(), `{"name":"john","age":30}`)

// Assert JSON contains specific fields
testing.AssertJSONContains(t, rr.Body.String(), `{"name":"john"}`)
```

### Request Helpers

```go
// Basic request
req := testing.NewRequest("POST", "/users", `{"name":"john"}`)

// With custom headers
req := testing.NewRequestWithHeaders("GET", "/", "", map[string]string{
    "Authorization": "Bearer token",
})

// From value (auto marshals to JSON)
req := testing.NewJSONRequest("POST", "/users", map[string]string{
    "name": "john",
})
```

### Error Assertions

```go
testing.AssertError(t, err)      // err != nil
testing.AssertNoError(t, err)    // err == nil
```

### Stub Helpers

#### StubDB

```go
db := &testing.StubDB{
    FnGet: func() (any, error) {
        return &User{Name: "john"}, nil
    },
    FnQuery: func() error {
        return nil
    },
}
```

#### StubCache

```go
cache := &testing.StubCache{}
cache.Set("user:1", &User{Name: "john"})

val, err := cache.Get("user:1")
// val: &User{Name: "john"}, err: nil

cache.Del("user:1")
cache.Clear()
```

#### StubWriter

```go
w := testing.NewStubWriter()
handler.ServeHTTP(w, req)

// Check status, body, headers
w.Code    // int
w.Body    // string
w.Hdr     // http.Header
```

### Helper Functions

```go
// Create JSON string
jsonStr := testing.JSONBody(map[string]string{"name": "john"})
// Result: {"name":"john"}
```

## Example: Full Handler Test

```go
func TestGetUser(t *testing.T) {
    // Setup
    db := &testing.StubDB{
        FnGet: func() (any, error) {
            return &User{ID: "1", Name: "john"}, nil
        },
    }

    // Handler under test
    handler := func(w http.ResponseWriter, r *http.Request) {
        user, _ := db.Get()
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        
        data, _ := json.Marshal(user)
        w.Write(data)
    }

    // Execute
    rr := testing.RunHandler(t, handler, http.MethodGet, "/users/1", "")

    // Assert
    testing.AssertStatusOK(t, rr.Code)
    testing.AssertHeader(t, rr.Header(), "Content-Type", "application/json")
    testing.AssertJSONContains(t, rr.Body.String(), `{"id":"1","name":"john"}`)
}
```

## Example: Table-Driven Tests

```go
func TestStatusCodes(t *testing.T) {
    tests := []struct {
        name       string
        statusCode int
        want       int
    }{
        {"ok", http.StatusOK, http.StatusOK},
        {"created", http.StatusCreated, http.StatusCreated},
        {"not found", http.StatusNotFound, http.StatusNotFound},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(tt.statusCode)
            }
            rr := testing.RunHandler(t, handler, http.MethodGet, "/", "")
            testing.AssertStatus(t, rr.Code, tt.want)
        })
    }
}
```