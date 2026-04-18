# GraphQL

GraphQL schema and resolver helpers for building GraphQL APIs.

## What It Does

Provides a simple GraphQL execution layer:

- Define schema with Query/Mutation types
- Resolve fields with functions
- Execute queries

## Quick Start

### Schema Definition

```go
import "github.com/azghr/mesh/graphql"

schema := graphql.NewSchema(graphql.SchemaConfig{
    Query: graphql.NewObject("Query", []graphql.Field{
        {Name: "user", Type: "User", Resolve: getUser},
    }),
    Mutation: graphql.NewObject("Mutation", []graphql.Field{
        {Name: "createUser", Type: "User", Resolve: createUser},
    }),
})
```

### Execute Query

```go
result := schema.Exec(ctx, `
    query {
        user(id: "1") {
            id
            name
        }
    }
`, map[string]any{"id": "1"})

if result.HasErrors() {
    log.Error(result.Error())
}

data := result.Data
```

## Schema

### Define Query

```go
queryType := graphql.NewObject("Query", []graphql.Field{
    graphql.Resolve("user", "User", getUser,
        graphql.Arg("id", "ID"),
    ),
    graphql.Resolve("users", "[User!]!", listUsers),
})
```

### Define Mutation

```go
mutationType := graphql.NewObject("Mutation", []graphql.Field{
    graphql.Resolve("createUser", "User", createUser,
        graphql.Arg("name", "String!"),
        graphql.Arg("email", "String"),
    ),
})
```

### Full Schema

```go
schema := graphql.NewSchema(graphql.SchemaConfig{
    Query:    queryType,
    Mutation: mutationType,
})
```

## Resolvers

### Basic Resolver

```go
func getUser(ctx context.Context, source any, args map[string]any) (any, error) {
    id := args["id"].(string)
    return db.FindUser(id)
}
```

### With Arguments

```go
resolver := func(ctx context.Context, parent any, args map[string]any) (any, error) {
    id, ok := args["id"].(string)
    if !ok {
        return nil, fmt.Errorf("id required")
    }
    return db.FindUser(id)
}
```

### Field Resolver

```go
field := graphql.Resolve("user", "User", resolver,
    graphql.Arg("id", "ID", "default-id"),
)
```

### Nested Resolver

```go
// For nested fields
type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

userResolver := func(ctx context.Context, source any, args map[string]any) (any, error) {
    user := source.(*User)
    return user.Name, nil
}
```

## Types

### Scalars

```go
graphql.Scalar("String")
graphql.Scalar("Int")
graphql.Scalar("Float")
graphql.Scalar("Boolean")
graphql.Scalar("ID")
```

### Non-Null

```go
graphql.NonNull("String")  // String!
```

### List

```go
graphql.List("String")    // [String]
```

### Input Values

```go
graphql.Arg("name", "String!")
graphql.Arg("limit", "Int", 10)
```

## Execution

### Execute Query

```go
result := schema.Exec(ctx, "{ user(id: \"1\") { id name } }", nil)
```

### With Variables

```go
vars := map[string]any{
    "id": "1",
}
result := schema.Exec(ctx, "query($id: ID!) { user(id: $id) { id name } } }", vars)
```

### Execute Mutation

```go
result := schema.Exec(ctx, "mutation { createUser(name: \"John\") { id } }", nil)
```

## Introspection

```go
intro := schema.Introspect()
// Returns schema types and fields
```

## Type Helpers

### Check Non-Null

```go
graphql.IsNonNullType("String!")  // true
graphql.IsNonNullType("String")  // false
```

### Check List

```go
graphql.IsListType("[String]")     // true
graphql.IsListType("String")       // false
```

### Get Base Type

```go
graphql.GetTypeName("String!")  // String
graphql.GetTypeName("[User]")  // User
```

## Full Example

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/graphql"
    "github.com/gofiber/fiber/v2"
)

func main() {
    app := fiber.New()

    schema := graphql.NewSchema(graphql.SchemaConfig{
        Query: graphql.NewObject("Query", []graphql.Field{
            graphql.Resolve("user", "*User", getUser,
                graphql.Arg("id", "ID"),
            ),
            graphql.Resolve("users", "[]*User", listUsers,
                graphql.Arg("limit", "Int", 10),
            ),
        }),
    })

    app.Post("/graphql", func(c *fiber.Ctx) error {
        var req struct {
            Query     string         `json:"query"`
            Variables map[string]any `json:"variables"`
        }
        c.BodyParser(&req)

        result := schema.Exec(c.Context(), req.Query, req.Variables)

        return c.JSON(result)
    })

    app.Listen(":8080")
}

func getUser(ctx context.Context, src any, args map[string]any) (any, error) {
    id := args["id"].(string)
    return &User{ID: id, Name: "John"}, nil
}

func listUsers(ctx context.Context, src any, args map[string]any) (any, error) {
    limit := args["limit"].(int)
    users := make([]*User, limit)
    for i := 0; i < limit; i++ {
        users[i] = &User{ID: "1", Name: "John"}
    }
    return users, nil
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

## HTTP Handler

```go
app.Post("/graphql", handleGraphQL(schema))

func handleGraphQL(schema *graphql.Schema) fiber.Handler {
    return func(c *fiber.Ctx) error {
        var req struct {
            Query     string         `json:"query"`
            Variables map[string]any `json:"variables"`
        }
        c.BodyParser(&req)

        result := schema.Exec(c.Context(), req.Query, req.Variables)
        return c.JSON(result)
    }
}
```

## Best Practices

### 1. Use Strong Types

```go
// Instead of any, define return types
type User struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Email *string `json:"email"`  // Pointer for nullable
}
```

### 2. Error Handling

```go
resolver := func(ctx context.Context, src any, args map[string]any) (any, error) {
    user, err := db.FindUser(args["id"].(string))
    if err != nil {
        return nil, fmt.Errorf("user not found: %w", err)
    }
    return user, nil
}
```

### 3. Validate Arguments

```go
resolver := func(ctx context.Context, src any, args map[string]any) (any, error) {
    id, ok := args["id"].(string)
    if !ok || id == "" {
        return nil, fmt.Errorf("invalid id")
    }
    return db.FindUser(id)
}
```

### 4. Use Context for Auth

```go
resolver := func(ctx context.Context, src any, args map[string]any) (any, error) {
    userID := ctx.Value("user_id").(string)
    if userID == "" {
        return nil, fmt.Errorf("unauthorized")
    }
    return db.FindUser(args["id"].(string))
}
```

## Limitations

This is a simple GraphQL implementation. For production use with complex schemas, consider:

- [99designs/gqlgen](https://github.com/99designs/gqlgen) - More complete implementation
- [graph-gql/graphql-go](https://github.com/graph-gql/graphql-go) - Standards-compliant

This package is ideal for:
- Simple APIs
- Prototyping
- Lightweight microservices