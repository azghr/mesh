# apiversion

API version negotiation and routing for HTTP APIs.

## What It Does

Provides API versioning with multiple strategies:
- URL path versioning (/v1/, /v2/)
- Accept header versioning
- Custom header versioning
- Version-specific route handlers

## Usage

### URL Path Versioning

```go
r := apiversion.New()

// Register routes for each version
r.V1().Get("/users", handlerV1)
r.V2().Get("/users", handlerV2)

app := r.App()
```

### Middleware Version Detection

```go
app := fiber.New()
app.Use(apiversion.Middleware("v1"))

app.Get("/users", func(c *fiber.Ctx) error {
    version := apiversion.GetVersion(c)
    // Use version to determine response format
    return c.JSON(fiber.Map{"version": version})
})
```

### Version Detection Order

1. X-API-Version header
2. Accept header (application/vnd.mesh.v1+json)
3. URL path (/v1/users)
4. Default version

## Accept Header Format

```
Accept: application/vnd.mesh.v1+json
Accept: application/json;version=v2
```

## X-API-Version Header

```
X-API-Version: v1
X-API-Version: v2
```

## Get Version in Handler

```go
app.Get("/resource", func(c *fiber.Ctx) error {
    version := apiversion.GetVersion(c)
    
    switch version {
    case "v1":
        return handleV1(c)
    case "v2":
        return handleV2(c)
    default:
        return handleV1(c)
    }
})
```

## With Standard Library

```go
func handler(w http.ResponseWriter, r *http.Request) {
    version := apiversion.FromRequest(r, "v1")
    
    fmt.Fprintf(w, "Version: %s", version)
}
```

## Versioned Handler

```go
app.Use(apiversion.Middleware())

app.Get("/users", apiversion.VersionedHandler(func(c *fiber.Ctx, version string) error {
    if version == "v2" {
        return handleV2(c)
    }
    return handleV1(c)
}))
```

## Is Latest Version

```go
available := apiversion.SupportedVersions("v1", "v2", "v3")

if apiversion.IsLatest("v3", available) {
    // Use latest features
}
```

## Configuration

```go
r := apiversion.New(
    apiversion.WithDefault("v1"),
    apiversion.WithPrefix("/api/v"),
)
```

| Option | Default | Description |
|--------|---------|-------------|
| WithDefault | v1 | Default version |
| WithPrefix | /v | URL prefix |

## Example: Full Router Setup

```go
func main() {
    r := apiversion.New()
    
    // V1 routes
    r.V1().Get("/users", listUsersV1)
    r.V1().Post("/users", createUserV1)
    
    // V2 routes
    r.V2().Get("/users", listUsersV2)
    r.V2().Post("/users", createUserV2)
    r.V2().Get("/users/:id", getUserV2)
    
    // Shared routes
    r.Get("/health", healthCheck)
    
    log.Fatal(r.App().Listen(":3000"))
}
```

## Example: Conditional Response

```go
func GetUser(c *fiber.Ctx) error {
    version := apiversion.GetVersion(c)
    user, _ := db.GetUser(c.Params("id"))
    
    if version == "v2" {
        return c.JSON(fiber.Map{
            "data": fiber.Map{
                "id": user.ID,
                "name": user.Name,
                "email": user.Email,
                "created_at": user.CreatedAt,
            },
        })
    }
    
    // v1 response
    return c.JSON(fiber.Map{
        "id": user.ID,
        "name": user.Name,
    })
}
```

## Constants

```go
apiversion.DefaultVersion   // "v1"
apiversion.HeaderAccept     // "Accept"
apiversion.HeaderAPIVersion // "X-API-Version"
```
