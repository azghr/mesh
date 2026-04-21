# Elasticsearch

Simple client for Elasticsearch full-text search.

## Overview

This package provides a lightweight client for Elasticsearch operations.

## Installation

```go
import "github.com/azghr/mesh/elasticsearch"
```

## Usage

```go
client, _ := elasticsearch.New(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})

// Index document
client.Index(ctx, "users", "1", map[string]string{
    "name": "John",
    "email": "john@example.com",
})

// Search
result, _ := client.Search(ctx, "users", "John")
for _, hit := range result.Hits {
    fmt.Println(string(hit.Source))
}
```

## API

- `New(cfg Config) (*Client, error)` - Creates client
- `client.Index(ctx, index, id, doc)` - Indexes document
- `client.Get(ctx, index, id)` - Retrieves document
- `client.Search(ctx, index, query)` - Searches documents
- `client.Delete(ctx, index, id)` - Deletes document
- `client.Bulk(ctx, ops)` - Bulk operations