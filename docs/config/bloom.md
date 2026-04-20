# Bloom Filter

Space-efficient probabilistic set membership using Redis.

## Overview

A Bloom filter is a space-efficient probabilistic data structure that can tell you:
- If an element is **definitely not** in a set
- If an element is **possibly** in a set

This is useful for:
- **Cache de-duplication**: Check if data is already cached before querying database
- **Spam filtering**: Check if email was already processed
- **Fraud detection**: Check if transaction was seen before
- **Membership testing**: Check if user has already seen content

## Installation

```go
import "github.com/azghr/mesh/bloom"
```

## Usage

### Basic Usage

```go
bf := bloom.New(redisClient, "cache-keys", 10000, 0.01)

// Add cache keys after fetching from database
bf.Add(ctx, "user:123")

// Check before querying database
exists, _ := bf.Exists(ctx, "user:123")
if exists {
    log.Println("Might be cached, check database")
}
```

### Batch Operations

```go
// Add multiple items
items := []string{"user:1", "user:2", "user:3"}
bf.AddMany(ctx, items)

// Check multiple items
result, _ := bf.ExistsMany(ctx, []string{"user:1", "user:4"})
for item, exists := range result {
    fmt.Printf("%s: %v\n", item, exists)
}
```

### Reset

```go
bf.Reset(ctx) // Clear all items
```

### Statistics

```go
stats, _ := bf.Stats(ctx)
fmt.Printf("Items: %d, Bits: %d, Hashes: %d\n", 
    stats.ItemCount, stats.NumBits, stats.NumHashes)
```

## Configuration

| Parameter | Description | Example |
|-----------|-------------|----------|
| `name` | Filter identifier (Redis key) | `"users"` |
| `expectedItems` | Items expected to store | `100000` |
| `falsePositive` | False positive rate (0-1) | `0.01` (1%) |

### False Positive Rate

| Rate | Memory per item | Use case |
|------|----------------|----------|
| `0.1` (10%) | ~0.5 bits | Cache de-duplication |
| `0.01` (1%) | ~2 bits | General use |
| `0.001` (0.1%) | ~3 bits | Spam filtering |

## API Reference

### `New(client *redis.Client, name string, expectedItems int, falsePositive float64) *Filter`

Creates a new Bloom filter with optimized bit size and hash functions.

### `bf.Add(ctx context.Context, item string) error`

Adds an item to the filter. This is idempotent.

### `bf.Exists(ctx context.Context, item string) (bool, error)`

Checks if an item might exist in the filter.

- **Returns `false`**: Item is definitely not in filter
- **Returns `true`**: Item might be in filter (could be false positive)

### `bf.AddMany(ctx context.Context, items []string) error`

Adds multiple items efficiently using pipeline.

### `bf.ExistsMany(ctx context.Context, items []string) (map[string]bool, error)`

Checks multiple items at once.

### `bf.Reset(ctx context.Context) error`

Clears all items from the filter.

### `bf.Stats(ctx context.Context) (Stats, error)`

Returns filter statistics:
- `Name`: Filter name
- `ExpectedItems`: Configured capacity
- `FalsePositiveRate`: Configured FPR
- `NumBits`: Memory allocated
- `NumHashes`: Hash functions
- `ItemCount`: Approximate items

## Example: Cache De-duplication

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
    bf := bloom.New(client, "user-cache", 100000, 0.01)

    ctx := context.Background()

    // Before querying for user
    userID := "12345"
    exists, err := bf.Exists(ctx, "user:"+userID)

    if err != nil {
        log.Fatal(err)
    }

    if exists {
        // Might be in cache, check
        // fetch from cache or database
        log.Printf("user:%s might be cached", userID)
    } else {
        // Definitely not in cache
        // fetch from database, then add to cache and bloom filter
        log.Printf("user:%s not in cache, fetching", userID)
        
        // After adding to cache
        bf.Add(ctx, "user:"+userID)
    }
}
```

## Example: Spam Detection

```go
package main

import (
    "context"
    "fmt"

    "github.com/azghr/mesh/bloom"
    "github.com/redis/go-redis/v9"
)

func main() {
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    bf := bloom.New(client, "seen-emails", 1000000, 0.001)

    ctx := context.Background()
    email := "spam@bad.com"

    exists, _ := bf.Exists(ctx, email)
    if exists {
        fmt.Println("Email already seen, might be spam")
    } else {
        fmt.Println("New email")
        // Add after processing
        bf.Add(ctx, email)
    }
}
```

## Performance

| Operation | Time Complexity |
|-----------|----------------|
| Add | O(k) |
| Exists | O(k) |
| Reset | O(1) |

Where `k` is the number of hash functions (typically 7).