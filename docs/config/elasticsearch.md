# Elasticsearch

Simple client for Elasticsearch full-text search.

## Overview

This package provides a lightweight client for Elasticsearch operations including indexing, searching, and managing documents.

## Features

- Simple document indexing
- Full-text search
- Bulk operations
- Document deletion
- Basic authentication

## Installation

```go
import "github.com/azghr/mesh/elasticsearch"
```

## Usage

### Connect to Elasticsearch

```go
client, err := elasticsearch.New(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "elastic",
    Password:  "password",
})
if err != nil {
    log.Fatal(err)
}
```

### Index a Document

```go
err := client.Index(ctx, "users", "1", map[string]interface{}{
    "name":  "John Doe",
    "email": "john@example.com",
    "age":   30,
})
if err != nil {
    log.Fatal(err)
}
```

### Get a Document

```go
doc, err := client.Get(ctx, "users", "1")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(doc.Source))
```

### Search Documents

```go
result, err := client.Search(ctx, "users", "John")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d results\n", result.Total)
for _, hit := range result.Hits {
    fmt.Println(string(hit.Source))
}
```

### Delete a Document

```go
err := client.Delete(ctx, "users", "1")
if err != nil {
    log.Fatal(err)
}
```

### Bulk Operations

```go
ops := []elasticsearch.BulkOperation{
    {
        Metadata: map[string]string{"index": "_doc", "_index": "users", "_id": "1"},
        Document: map[string]string{"name": "John"},
    },
    {
        Metadata: map[string]string{"index": "_doc", "_index": "users", "_id": "2"},
        Document: map[string]string{"name": "Jane"},
    },
}

err := client.Bulk(ctx, ops)
if err != nil {
    log.Fatal(err)
}
```

### Check Connection

```go
err := client.Ping(ctx)
if err != nil {
    log.Fatal("Elasticsearch not reachable")
}
```

## API Reference

### `New(cfg Config) (*Client, error)`

Creates a new Elasticsearch client.

### `client.Index(ctx context.Context, index, id string, document interface{}) error`

Indexes a document. The document is JSON-encoded.

### `client.Get(ctx context.Context, index, id string) (*Document, error)`

Retrieves a document by ID.

### `client.Search(ctx context.Context, index, query string) (*SearchResult, error)`

Searches for documents matching the query.

### `client.Delete(ctx context.Context, index, id string) error`

Deletes a document by ID.

### `client.DeleteIndex(ctx context.Context, index string) error`

Deletes an entire index.

### `client.Bulk(ctx context.Context, operations []BulkOperation) error`

Performs bulk operations.

### `client.Ping(ctx context.Context) error`

Checks if Elasticsearch is reachable.

## Configuration

| Parameter | Description | Required |
|-----------|-------------|----------|
| `Addresses` | Elasticsearch URLs | Yes |
| `Username` | Basic auth username | No |
| `Password` | Basic auth password | No |

## Example: User Search

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/elasticsearch"
)

type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    client, _ := elasticsearch.New(elasticsearch.Config{
        Addresses: []string{"http://localhost:9200"},
    })

    ctx := context.Background()

    // Index users
    users := []User{
        {Name: "John Doe", Email: "john@example.com"},
        {Name: "Jane Smith", Email: "jane@example.com"},
    }

    for i, user := range users {
        client.Index(ctx, "users", string(rune('1'+i)), user)
    }

    // Search
    result, _ := client.Search(ctx, "users", "John")
    log.Printf("Found %d users", result.Total)

    for _, hit := range result.Hits {
        log.Printf("User: %s", string(hit.Source))
    }
}
```
