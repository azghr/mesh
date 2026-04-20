# Storage

File storage abstraction for local filesystem.

## Overview

This package provides file storage abstractions. Currently supports local filesystem. S3 support requires additional AWS SDK dependencies.

## Features

- Local filesystem storage
- Upload, download, delete operations
- Exists and list operations

## Installation

```go
import "github.com/azghr/mesh/storage"
```

## Usage

### Local Storage

```go
store := storage.NewLocal("/tmp/uploads")

// Upload
err := store.Upload(ctx, "file.txt", []byte("hello"))
if err != nil {
    log.Fatal(err)
}

// Download
data, err := store.Download(ctx, "file.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(data))

// Delete
err := store.Delete(ctx, "file.txt")

// Check exists
exists, _ := store.Exists(ctx, "file.txt")
fmt.Printf("exists: %v\n", exists)

// List all files
keys, _ := store.List(ctx, "")
for _, key := range keys {
    fmt.Println(key)
}
```

## API Reference

### `NewLocal(root string) *LocalStorage`

Creates a new local storage backend at the specified root directory.

### `store.Upload(ctx context.Context, key string, data []byte) error`

Uploads data to the specified key. Creates directories as needed.

### `store.Download(ctx context.Context, key string) ([]byte, error)`

Downloads data from the specified key. Returns error if not found.

### `store.Delete(ctx context.Context, key string) error`

Deletes the specified key. Returns error if not found.

### `store.Exists(ctx context.Context, key string) (bool, error)`

Checks if the key exists.

### `store.List(ctx context.Context, prefix string) ([]string, error)`

Lists keys with the given prefix. Returns empty slice if prefix doesn't exist.

## Example: User Uploads

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/azghr/mesh/storage"
)

var store *storage.LocalStorage

func init() {
    store = storage.NewLocal("/tmp/uploads")
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    defer file.Close()

    data, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    key := "uploads/" + header.Filename
    if err := store.Upload(r.Context(), key, data); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    w.Write([]byte("uploaded: " + key))
}

func main() {
    http.HandleFunc("/upload", handleUpload)
    log.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Example: Cache Files

```go
package main

import (
    "context"
    "log"
    "os"
    "path/filepath"

    "github.com/azghr/mesh/storage"
)

func main() {
    store := storage.NewLocal("/tmp/cache")
    ctx := context.Background()

    // Cache function
    func GetOrStore(key string, fetcher func() ([]byte, error)) ([]byte, error) {
        data, err := store.Download(ctx, key)
        if err == nil {
            return data, nil
        }
        
        data, err = fetcher()
        if err != nil {
            return nil, err
        }
        
        store.Upload(ctx, key, data)
        return data, nil
    }

    // Usage
    data, err := GetOrStore("data.json", func() ([]byte, error) {
        return []byte(`{"key": "value"}`), nil
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Println(string(data))
}
```