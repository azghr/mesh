# Upload

HTTP file upload handler with local and S3 storage backends.

## Overview

This package handles multipart file uploads with support for local filesystem and S3 storage. It provides an HTTP handler for uploading and downloading files.

## Installation

```go
import "github.com/azghr/mesh/upload"
```

## Usage

### Local Storage

```go
storage := upload.NewLocalStorage("/uploads")
uploader := upload.NewUploader(storage)

handler := func(w http.ResponseWriter, r *http.Request) {
    file, err := uploader.Upload(r, "file")
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    fmt.Fprintf(w, "Uploaded: %s", file.URL)
}
```

### S3 Storage

```go
storage, err := upload.NewS3Storage(upload.S3Config{
    Bucket:           "my-bucket",
    Region:          "us-east-1",
    AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
    SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
})
if err != nil {
    log.Fatal(err)
}
uploader := upload.NewUploader(storage)
```

### HTTP Handler

```go
store := upload.NewLocalStorage("/uploads")
handler := upload.NewFileHandler(store)
http.HandleFunc("/files", handler)
```

### S3 Compatible Storage

```go
storage, err := upload.NewS3Storage(upload.S3Config{
    Bucket:       "my-bucket",
    Region:      "us-east-1",
    Endpoint:    "http://localhost:9000",
    UsePathStyle: true,
})
```

## Uploader Options

### Maximum File Size

```go
uploader := upload.NewUploader(storage,
    upload.WithMaxSize(10<<20), // 10MB
)
```

### Allowed Extensions

```go
uploader := upload.NewUploader(storage,
    upload.WithAllowedExtensions([]string{".jpg", ".png", ".pdf"}),
)
```

### Custom Base URL

```go
store := upload.NewLocalStorage("/uploads")
store.SetBaseURL("https://cdn.example.com")
uploader := upload.NewUploader(store)
```

## API

### `NewLocalStorage(basePath string) Storage`

Creates local filesystem storage.

### `NewS3Storage(cfg S3Config) (Storage, error)`

Creates S3 storage. Returns error if config is invalid.

### S3Config

| Field | Description |
|-------|-------------|
| `Bucket` | S3 bucket name |
| `Region` | AWS region (e.g., "us-east-1") |
| `Endpoint` | Custom endpoint (optional) |
| `AccessKeyID` | AWS access key (optional) |
| `SecretAccessKey` | AWS secret key (optional) |
| `UsePathStyle` | Use path-style URLs |

### `NewUploader(storage Storage, opts ...UploaderOption) *Uploader`

Creates uploader with optional configuration.

### `WithMaxSize(maxSize int64) UploaderOption`

Sets maximum upload size in bytes.

### `WithAllowedExtensions(exts []string) UploaderOption`

Sets allowed file extensions (include the dot, e.g., ".jpg").

### `uploader.Upload(r, fieldName) (*FileInfo, error)`

Handles multipart file upload from HTTP request.

### `uploader.UploadMultiple(r, fieldName) ([]*FileInfo, error)`

Handles multiple file uploads from HTTP request.

### FileInfo

| Field | Description |
|-------|-------------|
| `Name` | Original filename |
| `Path` | Storage path/key |
| `URL` | Public or presigned URL |
| `Size` | File size in bytes |
| `ContentType` | MIME type |
| `UploadedAt` | Upload timestamp |

### `NewFileHandler(storage Storage) http.HandlerFunc`

Returns HTTP handler for file upload/download.

### `storage.SetBaseURL(baseURL string)`

Sets base URL for generated URLs.

## Example

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/azghr/mesh/upload"
)

func main() {
    storage := upload.NewLocalStorage("/tmp/uploads")
    storage.SetBaseURL("https://cdn.example.com/files")

    uploader := upload.NewUploader(storage,
        upload.WithMaxSize(10<<20),
        upload.WithAllowedExtensions([]string{".jpg", ".png", ".gif"}),
    )

    handler := upload.NewFileHandler(storage)
    http.HandleFunc("/upload", handler)

    fmt.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```