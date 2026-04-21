# gRPC Gateway

HTTP to gRPC transcoding gateway.

## Overview

This package provides HTTP to gRPC transcoding. It translates HTTP/JSON requests to gRPC calls.

## Installation

```go
import "github.com/azghr/mesh/grpcgateway"
```

## Usage

```go
gw := grpcgateway.New(grpcgateway.WithEndpoint("localhost:9090"))
gw.RegisterMethod("/YourService/GetUser", "GET", "Get user")
http.ListenAndServe(":8080", gw)
```

## API

- `New(opts...) *Gateway` - Creates gateway
- `gw.RegisterMethod(path, method, desc)` - Registers method
- `gw.WithEndpoint(addr)` - Sets backend

## Example

```go
package main

import (
    "net/http"

    "github.com/azghr/mesh/grpcgateway"
)

func main() {
    gw := grpcgateway.New(
        grpcgateway.WithEndpoint("localhost:9090"),
    )
    
    // Register methods
    gw.RegisterMethod("/user.UserService/GetUser", "GET", "Get user")
    gw.RegisterMethod("/user.UserService/CreateUser", "POST", "Create user")
    
    http.ListenAndServe(":8080", gw)
}
```