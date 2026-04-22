# Storage

File storage abstraction for local filesystem and S3.

## Overview

This package provides file storage abstractions. Supports both local filesystem and S3-compatible storage backends.

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

### S3 Storage

```go
store, err := storage.NewS3(storage.S3Config{
    Bucket:           "my-bucket",
    Region:          "us-east-1",
    AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
    SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
})

// Upload
store.Upload(ctx, "file.txt", []byte("hello"))

// Download
data, _ := store.Download(ctx, "file.txt")

// Delete
store.Delete(ctx, "file.txt")
```

### Presigned URLs (S3 only)

```go
// Get a presigned URL for downloads (valid for 1 hour)
getURL, _ := store.PresignGet(ctx, "file.txt", time.Hour)

// Get a presigned URL for uploads
putURL, _ := store.PresignUpload(ctx, "upload.txt", time.Hour)
```

### S3 Compatible Storage (MinIO, LocalStack)

```go
store, err := storage.NewS3(storage.S3Config{
    Bucket:       "my-bucket",
    Region:      "us-east-1",
    Endpoint:    "http://localhost:9000",
    UsePathStyle: true,
})
```

## API

### `NewLocal(root string) *LocalStorage`

Creates local filesystem storage.

### `NewS3(cfg S3Config) (*S3Storage, error)`

Creates S3 storage. Returns error if config is invalid.

### S3Config

| Field | Description |
|-------|-------------|
| `Bucket` | S3 bucket name |
| `Region` | AWS region (e.g., "us-east-1") |
| `Endpoint` | Custom endpoint (optional, for S3-compatible services) |
| `AccessKeyID` | AWS access key (optional, uses default credentials) |
| `SecretAccessKey` | AWS secret key (optional, uses default credentials) |
| `UsePathStyle` | Use path-style URLs (required for MinIO) |

### `store.Upload(ctx, key, data) error`

Uploads data to storage.

### `store.Download(ctx, key) ([]byte, error)`

Downloads data from storage.

### `store.Delete(ctx, key) error`

Deletes key from storage.

### `store.Exists(ctx, key) (bool, error)`

Checks if key exists.

### `store.List(ctx, prefix) ([]string, error)`

Lists keys with given prefix.

### `store.GetURL(ctx, key) (string, error)`

Returns URL for key (presigned for S3, local path for filesystem).

### `store.PresignGet(ctx, key, expiry) (string, error)`

Returns presigned URL for downloading (S3 only).

### `store.PresignUpload(ctx, key, expiry) (string, error)`

Returns presigned URL for uploading (S3 only).

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