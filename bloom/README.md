# Bloom Filter

Space-efficient probabilistic set membership using Redis.

## Overview

A Bloom filter is a space-efficient probabilistic data structure that can tell you:
- If an element is **definitely not** in a set
- If an element is **possibly** in a set

## Installation

```go
import "github.com/azghr/mesh/bloom"
```

## Usage

```go
// Create filter: name, expected items, false positive rate
bf := bloom.New(redisClient, "users", 100000, 0.01)

// Add items
bf.Add(ctx, "user:123")

// Check existence
exists, _ := bf.Exists(ctx, "user:123") // returns true

exists, _ := bf.Exists(ctx, "user:456") // returns false
```

## API

### `New(client, name, expectedItems, falsePositive) *Filter`

Creates a filter.

### `bf.Add(ctx, item) error`

Adds an item.

### `bf.Exists(ctx, item) (bool, error)`

Checks if item might exist.

### `bf.ExistsMany(ctx, items) (map[string]bool, error)`

Checks multiple items.

### `bf.Reset(ctx) error`

Clears the filter.

### `bf.Stats(ctx) (Stats, error)`

Returns statistics.

## Example

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/bloom"
    "github.com/redis/go-redis/v9"
)

func main() {
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    bf := bloom.New(client, "cache-keys", 10000, 0.01)

    ctx := context.Background()

    // Add cache keys
    bf.AddMany(ctx, []string{"user:1", "user:2", "user:3"})

    // Check before querying database
    exists, _ := bf.Exists(ctx, "user:1")
    if exists {
        log.Println("Might be in cache")
    }
}
```