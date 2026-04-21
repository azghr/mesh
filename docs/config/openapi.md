# OpenAPI

OpenAPI 3.0 specification generator for HTTP APIs.

## Overview

This package helps generate OpenAPI 3.0 specifications for HTTP APIs. It provides a builder for defining routes, parameters, request bodies, and responses.

## Features

- OpenAPI 3.0 spec generation
- Route and endpoint definitions
- Schema definitions
- JSON and YAML output
- Parameter and request body support
- Response definitions
- Security schemes

## Installation

```go
import "github.com/azghr/mesh/openapi"
```

## Usage

### Basic Usage

```go
spec := openapi.New("User API", "1.0.0")

spec.AddRoute("GET", "/users", "List all users")
spec.AddRoute("POST", "/users", "Create a user")
spec.AddRoute("GET", "/users/{id}", "Get user by ID")
spec.AddRoute("PUT", "/users/{id}", "Update user")
spec.AddRoute("DELETE", "/users/{id}", "Delete user")

json, _ := spec.ToJSON()
fmt.Println(string(json))
```

### With Parameters

```go
spec := openapi.New("User API", "1.0.0")

op := spec.AddRoute("GET", "/users", "List users")
op.AddParam("limit", "query", "Max results", false, openapi.IntSchema())
op.AddParam("offset", "query", "Result offset", false, openapi.IntSchema())
op.AddParam("status", "query", "Filter by status", false, openapi.StringSchema(""))
```

### With Request Body

```go
spec := openapi.New("User API", "1.0.0")

userSchema := openapi.ObjectSchema(map[string]openapi.Schema{
    "name":  openapi.StringSchema(""),
    "email": openapi.StringSchema("email"),
}, []string{"name", "email"})

op := spec.AddRoute("POST", "/users", "Create user")
op.SetRequestBody("User data", true, userSchema)
op.AddResponse(201, "User created")
op.AddResponse(400, "Validation error")
```

### With Responses and Schemas

```go
spec := openapi.New("User API", "1.0.0")

spec.AddSchema("User", openapi.ObjectSchema(map[string]openapi.Schema{
    "id":    openapi.IntSchema(),
    "name":  openapi.StringSchema(""),
    "email": openapi.StringSchema("email"),
}, []string{"id", "name"}))

op := spec.AddRoute("GET", "/users/{id}", "Get user")
op.AddParam("id", "path", "User ID", true, openapi.IntSchema())
op.AddResponseWithSchema(200, "User found", openapi.Schema{Ref: "#/components/schemas/User"})
op.AddResponse(404, "User not found")
```

### With Tags and Operation ID

```go
spec := openapi.New("User API", "1.0.0")

op := spec.AddRoute("GET", "/users", "List users")
op.AddTag("users")
op.SetOperationID("listUsers")
```

### With Servers

```go
spec := openapi.New("User API", "1.0.0")
spec.AddServer("https://api.example.com", "Production server")
spec.AddServer("https://staging.example.com", "Staging server")
```

### With Security

```go
spec := openapi.New("User API", "1.0.0")
spec.AddSecurityScheme("bearerAuth", openapi.SecurityScheme{
    Type:         "http",
    Scheme:       "bearer",
    BearerFormat: "JWT",
})
```

### Output Formats

```go
// JSON
json, _ := spec.ToJSON()

// YAML
yaml, _ := spec.ToYAML()
```

## API Reference

### `New(title, version string) *Spec`

Creates a new OpenAPI specification.

### `spec.SetDescription(description string)`

Sets API description.

### `spec.AddRoute(method, path, summary string) *Operation`

Adds a route. Returns operation for further configuration.

### Operation Methods

| Method | Description |
|--------|-------------|
| `op.AddParam(...)` | Add parameter |
| `op.SetRequestBody(...)` | Set request body |
| `op.AddResponse(...)` | Add response |
| `op.AddResponseWithSchema(...)` | Add response with schema |
| `op.AddTag(...)` | Add tag |
| `op.SetOperationID(...)` | Set operation ID |

### Schema Helpers

| Function | Description |
|----------|-------------|
| `StringSchema(format)` | String schema |
| `IntSchema()` | Integer schema |
| `BoolSchema()` | Boolean schema |
| `ArraySchema(items)` | Array schema |
| `ObjectSchema(props, required)` | Object schema |

### Output

| Method | Description |
|--------|-------------|
| `spec.ToJSON()` | Convert to JSON |
| `spec.ToYAML()` | Convert to YAML |

## Example: Complete API

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/azghr/mesh/openapi"
)

func main() {
	spec := openapi.New("Pet Store API", "1.0.0")
	spec.SetDescription("A sample pet store API")

	// Add servers
	spec.AddServer("https://api.petstore.com", "Production")
	spec.AddServer("https://staging.petstore.com", "Staging")

	// Define schemas
	petSchema := openapi.ObjectSchema(map[string]openapi.Schema{
		"id":    openapi.IntSchema(),
		"name":  openapi.StringSchema(""),
		"status": openapi.StringSchema(""),
	}, []string{"name"})

	spec.AddSchema("Pet", petSchema)

	// List pets
	listOp := spec.AddRoute("GET", "/pets", "List all pets")
	listOp.AddParam("limit", "query", "Max pets to return", false, openapi.IntSchema())
	listOp.AddParam("status", "query", "Filter by status", false, openapi.StringSchema(""))
	listOp.AddResponseWithSchema(200, "Success", openapi.ArraySchema(petSchema))

	// Get pet
	getOp := spec.AddRoute("GET", "/pets/{id}", "Get a pet")
	getOp.AddParam("id", "path", "Pet ID", true, openapi.IntSchema())
	getOp.AddResponseWithSchema(200, "Success", petSchema)
	getOp.AddResponse(404, "Pet not found")

	// Create pet
	createOp := spec.AddRoute("POST", "/pets", "Create a pet")
	createOp.SetRequestBody("Pet data", true, petSchema)
	createOp.AddResponseWithSchema(201, "Created", petSchema)
	createOp.AddResponse(400, "Validation error")

	// Output
	json, _ := json.MarshalIndent(spec, "", "  ")
	fmt.Println(string(json))
}
```

This generates a complete OpenAPI 3.0 specification that can be used with Swagger UI or other OpenAPI tools.