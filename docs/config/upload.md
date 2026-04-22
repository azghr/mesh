# File Upload/Download

File upload and download functionality with local and cloud storage backends.

## Overview

This package handles multipart file uploads with pluggable storage backends.

## Features

- Multipart file uploads
- Local filesystem storage
- S3 storage (stub for AWS SDK integration)
- HTTP handlers for quick integration

## Usage

### Local Storage

```go
import "github.com/azghr/mesh/upload"

// Create local storage
storage := upload.NewLocalStorage("/uploads")

// Create uploader with options
uploader := upload.NewUploader(storage,
    upload.WithMaxSize(10<<20),          // 10MB max
    upload.WithAllowedExtensions([]string{".jpg", ".png", ".pdf"}),
)

// In HTTP handler
func uploadHandler(w http.ResponseWriter, r *http.Request) {
    info, err := uploader.Upload(r, "file")
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    fmt.Fprintf(w, `{"url":"%s","size":%d}`, info.URL, info.Size)
}
```

### HTTP Server Setup

```go
storage := upload.NewLocalStorage("/uploads")
handler := upload.NewFileHandler(storage)

http.Handle("/upload", handler)

// Endpoints:
// POST /upload - Upload file (multipart/form-data)
// GET /upload?name=file.txt - Download file
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `WithMaxSize(bytes)` | Maximum file size |
| `WithAllowedExtensions([]string)` | Allowed file extensions |

## Storage Interfaces

Implement the `Storage` interface for custom backends:

```go
type Storage interface {
    Save(ctx context.Context, name string, reader io.Reader) (*FileInfo, error)
    Load(ctx context.Context, name string) (io.ReadCloser, error)
    Delete(ctx context.Context, name string) error
    GetURL(ctx context.Context, name string) (string, error)
}
```

## FileInfo

```go
type FileInfo struct {
    Name        string
    Path       string
    URL        string
    Size       int64
    ContentType string
    UploadedAt time.Time
}
```

## Examples

### Single File Upload

```go
info, err := uploader.Upload(r, "avatar")
if err != nil {
    return err
}
fmt.Println("Uploaded to:", info.URL)
```

### Multiple File Upload

```go
files, err := uploader.UploadMultiple(r, "attachments")
for _, file := range files {
    fmt.Println("Uploaded:", file.URL)
}
```

### Custom Storage Backend

```go
type MyStorage struct{}

func (s *MyStorage) Save(ctx context.Context, name string, reader io.Reader) (*upload.FileInfo, error) {
    // Custom save logic
}

func (s *MyStorage) Load(ctx context.Context, name string) (io.ReadCloser, error) {
    // Custom load logic
}

func (s *MyStorage) Delete(ctx context.Context, name string) error {
    // Custom delete logic
}

func (s *MyStorage) GetURL(ctx context.Context, name string) (string, error) {
    return fmt.Sprintf("https://myservice.com/files/%s", name), nil
}
```

## API Reference

| Function | Description |
|-----------|-------------|
| `NewLocalStorage(path)` | Create local filesystem storage |
| `NewUploader(storage, opts...)` | Create uploader |
| `uploader.Upload(r, field)` | Handle multipart upload |
| `uploader.UploadMultiple(r, field)` | Handle multiple uploads |
| `NewFileHandler(storage)` | HTTP handler for upload/download |
| `storage.SetBaseURL(url)` | Set base URL for file access |