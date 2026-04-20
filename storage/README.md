# Storage

File storage abstraction for local filesystem.

## Overview

This package provides file storage abstractions. Currently supports local filesystem. S3 support requires AWS SDK dependencies.

## Installation

```go
import "github.com/azghr/mesh/storage"
```

## Usage

### Local Storage

```go
store := storage.NewLocal("/tmp/uploads")

// Upload
store.Upload(ctx, "file.txt", []byte("hello"))

// Download
data, _ := store.Download(ctx, "file.txt")

// Delete
store.Delete(ctx, "file.txt")

// Exists
exists, _ := store.Exists(ctx, "file.txt")

// List
keys, _ := store.List(ctx, "")
```

## API

### `NewLocal(root string) *LocalStorage`

Creates local storage.

### `store.Upload(ctx, key, data) error`

Uploads data.

### `store.Download(ctx, key) ([]byte, error)`

Downloads data.

### `store.Delete(ctx, key) error`

Deletes key.

### `store.Exists(ctx, key) (bool, error)`

Checks if key exists.

### `store.List(ctx, prefix) ([]string, error)`

Lists keys.

## Example

```go
package main

import (
    "context"
    "log"

    "github.com/azghr/mesh/storage"
)

func main() {
    store := storage.NewLocal("/tmp/uploads")

    ctx := context.Background()

    // Store user avatar
    store.Upload(ctx, "avatars/user123.png", imageData)

    // Retrieve
    data, err := store.Download(ctx, "avatars/user123.png")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Downloaded %d bytes", len(data))
}
```

Note: S3 storage requires `github.com/aws/aws-sdk-go-v2` dependencies.