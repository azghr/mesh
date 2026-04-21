# gRPC Gateway

HTTP to gRPC transcoding gateway.

## Overview

This package provides HTTP to gRPC transcoding. It translates HTTP/JSON requests to gRPC calls, allowing REST clients to access gRPC services.

## Features

- HTTP to gRPC transcoding
- JSON request/response
- Simple handler registration
- gRPC reflection support

## Installation

```go
import "github.com/azghr/mesh/grpcgateway"
```

## Usage

### Basic Usage

```go
gw := grpcgateway.New(grpcgateway.WithEndpoint("localhost:9090"))

// Register gRPC methods
gw.RegisterMethod("/user.UserService/GetUser", "GET", "Get user by ID")
gw.RegisterMethod("/user.UserService/CreateUser", "POST", "Create a new user")

// Start HTTP server
http.ListenAndServe(":8080", gw)
```

### Complete Example

```go
package main

import (
    "log"
    "net/http"

    "github.com/azghr/mesh/grpcgateway"
)

func main() {
    gw := grpcgateway.New(
        grpcgateway.WithEndpoint("localhost:9090"),
    )

    // Register user service methods
    gw.RegisterMethod("/user.UserService/GetUser", "GET", "Get user by ID")
    gw.RegisterMethod("/user.UserService/CreateUser", "POST", "Create user")
    gw.RegisterMethod("/user.UserService/UpdateUser", "PUT", "Update user")
    gw.RegisterMethod("/user.UserService/DeleteUser", "DELETE", "Delete user")

    // Register with HTTP mux
    http.Handle("/", gw)

    log.Println("gRPC Gateway started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## How It Works

1. Client sends HTTP request with JSON body
2. Gateway parses the HTTP method and path
3. Gateway maps to corresponding gRPC service and method
4. Gateway calls gRPC service
5. gRPC response is converted to JSON and returned to client

## API Reference

### `New(opts ...Option) *Gateway`

Creates a new gRPC gateway.

### `WithEndpoint(address string) Option`

Sets the gRPC backend endpoint.

```go
New(WithEndpoint("localhost:9090"))
```

### `gw.RegisterMethod(path, httpMethod, description string) *Mapping`

Registers an HTTP method that maps to a gRPC service method.

```go
gw.RegisterMethod("/your.Service/GetUser", "GET", "Get user")
```

### `gw.ListMethods() []*Mapping`

Returns all registered method mappings.

### `gw.HealthCheck(ctx context.Context) error`

Checks gateway health.

### `gw.Close() error`

Closes the gateway connections.

## HTTP to gRPC Mapping

| HTTP Method | gRPC Method Type | 
|------------|-----------------|
| GET | Unary |
| POST | Unary/Server Streaming |
| PUT | Unary |
| DELETE | Unary |

## Request/Response

### GET Request

```
GET /user.UserService/GetUser?id=123 HTTP/1.1
```

Converts to gRPC Unary call with request body containing query parameters.

### POST Request

```
POST /user.UserService/CreateUser HTTP/1.1
Content-Type: application/json

{"name": "John", "email": "john@example.com"}
```

Converts to gRPC Unary call with JSON body as request message.

## Configuration

| Option | Description |
|--------|-------------|
| `WithEndpoint` | gRPC backend address |

## Example: Complete Service

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/azghr/mesh/grpcgateway"
)

// User represents a user
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    gw := grpcgateway.New(
        grpcgateway.WithEndpoint("localhost:50051"),
    )

    // Register user service
    gw.RegisterMethod("/api.v1.UserService/Get", "GET", "Get user")
    gw.RegisterMethod("/api.v1.UserService/Create", "POST", "Create user")
    gw.RegisterMethod("/api.v1.UserService/Update", "PUT", "Update user")
    gw.RegisterMethod("/api.v1.UserService/Delete", "DELETE", "Delete user")
    gw.RegisterMethod("/api.v1.UserService/List", "GET", "List users")

    // Health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })

    // Gateway handler
    http.Handle("/", gw)

    log.Println("Starting gRPC Gateway on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }
}
```

## REST Client Example

```bash
# Get user
curl http://localhost:8080/api.v1.UserService/Get?id=123

# Create user
curl -X POST http://localhost:8080/api.v1.UserService/Create \
  -H "Content-Type: application/json" \
  -d '{"name": "John", "email": "john@example.com"}'

# List users
curl http://localhost:8080/api.v1.UserService/List

# Delete user
curl -X DELETE http://localhost:8080/api.v1.UserService/Delete?id=123
```

This provides a simple REST API that transparently calls gRPC services.