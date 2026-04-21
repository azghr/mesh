# OpenAPI

OpenAPI 3.0 specification generator.

## Overview

This package generates OpenAPI 3.0 specifications for HTTP APIs.

## Installation

```go
import "github.com/azghr/mesh/openapi"
```

## Usage

```go
spec := openapi.New("My API", "1.0.0")

spec.AddRoute("GET", "/users", "List users")
spec.AddRoute("POST", "/users", "Create user")

json, _ := spec.ToJSON()
```

## API

- `New(title, version) *Spec` - Creates spec
- `spec.AddRoute(method, path, summary)` - Adds route
- `spec.ToJSON()` - Outputs JSON
- `spec.ToYAML()` - Outputs YAML

See docs for full API.