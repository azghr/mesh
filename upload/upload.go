// Package upload provides file upload and download functionality.
//
// This package handles multipart file uploads with local and S3 storage backends.
//
// # Quick Start
//
// Local storage:
//
//	storage := upload.NewLocalStorage("/uploads")
//	uploader := upload.NewUploader(storage)
//
//	handler := func(w http.ResponseWriter, r *http.Request) {
//	    file, err := uploader.Upload(r, "file")
//	    if err != nil {
//	        http.Error(w, err.Error(), 400)
//	        return
//	    }
//	    fmt.Fprintf(w, "Uploaded: %s", file.URL)
//	}
//
// S3 storage:
//
//	storage, err := upload.NewS3Storage(upload.S3Config{
//	    Bucket:           "my-bucket",
//	    Region:          "us-east-1",
//	    AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
//	    SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
//	})
//	uploader := upload.NewUploader(storage)
package upload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/azghr/mesh/storage"
	"github.com/google/uuid"
)

// FileInfo represents an uploaded file
type FileInfo struct {
	Name        string
	Path        string
	URL         string
	Size        int64
	ContentType string
	UploadedAt  time.Time
}

// Storage interface for file storage backends
type Storage interface {
	Save(ctx context.Context, name string, reader io.Reader) (*FileInfo, error)
	Load(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
	GetURL(ctx context.Context, name string) (string, error)
	Download(ctx context.Context, key string) ([]byte, error)
	SetBaseURL(baseURL string)
}

// Uploader handles file uploads
type Uploader struct {
	maxSize    int64
	allowedExt []string
	storage    Storage
	baseURL    string
}

// UploaderOption configures the uploader
type UploaderOption func(*Uploader)

// WithMaxSize sets maximum upload size
func WithMaxSize(maxSize int64) UploaderOption {
	return func(u *Uploader) {
		u.maxSize = maxSize
	}
}

// WithAllowedExtensions sets allowed file extensions
func WithAllowedExtensions(exts []string) UploaderOption {
	return func(u *Uploader) {
		u.allowedExt = exts
	}
}

// NewUploader creates a new uploader
func NewUploader(storage Storage, opts ...UploaderOption) *Uploader {
	u := &Uploader{
		maxSize:    10 << 20, // 10MB default
		allowedExt: []string{},
		storage:    storage,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

// Upload handles multipart file upload
func (u *Uploader) Upload(r *http.Request, fieldName string) (*FileInfo, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("FormFile: %w", err)
	}
	defer file.Close()

	// Check file size
	if u.maxSize > 0 && header.Size > u.maxSize {
		return nil, fmt.Errorf("file too large: max %d bytes", u.maxSize)
	}

	// Check extension
	if len(u.allowedExt) > 0 {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		allowed := false
		for _, e := range u.allowedExt {
			if ext == strings.ToLower(e) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("file type not allowed: %s", ext)
		}
	}

	// Generate unique filename
	filename := u.generateFilename(header.Filename)

	// Save to storage
	info, err := u.storage.Save(r.Context(), filename, file)
	if err != nil {
		return nil, fmt.Errorf("Save: %w", err)
	}

	info.Name = header.Filename
	info.ContentType = header.Header.Get("Content-Type")
	info.Size = header.Size

	return info, nil
}

// UploadMultiple handles multiple file uploads
func (u *Uploader) UploadMultiple(r *http.Request, fieldName string) ([]*FileInfo, error) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("ParseMultipartForm: %w", err)
	}

	files := r.MultipartForm.File[fieldName]
	infos := make([]*FileInfo, 0, len(files))

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		filename := u.generateFilename(fileHeader.Filename)
		info, err := u.storage.Save(r.Context(), filename, file)
		if err != nil {
			continue
		}

		info.Name = fileHeader.Filename
		info.Size = fileHeader.Size
		infos = append(infos, info)
	}

	return infos, nil
}

func (u *Uploader) generateFilename(original string) string {
	ext := filepath.Ext(original)
	name := strings.TrimSuffix(original, ext)

	// Create safe name
	safeName := strings.ReplaceAll(name, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")

	// Add unique ID
	unique := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s%s", safeName, unique, ext)
}

// LocalStorage returns a local storage backend
func NewLocalStorage(basePath string) Storage {
	s := &localStorageAdapter{
		basePath: basePath,
	}
	s.Storage = storage.NewLocal(basePath)
	return s
}

type localStorageAdapter struct {
	storage.Storage
	basePath string
	baseURL  string
}

func (s *localStorageAdapter) GetURL(ctx context.Context, name string) (string, error) {
	if s.baseURL != "" {
		return s.baseURL + "/" + name, nil
	}
	return s.Storage.GetURL(ctx, name)
}

func (s *localStorageAdapter) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	data, err := s.Storage.Download(ctx, name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *localStorageAdapter) Save(ctx context.Context, name string, reader io.Reader) (*FileInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}
	if err := s.Storage.Upload(ctx, name, data); err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	return &FileInfo{
		Name:       name,
		Path:       name,
		URL:        name,
		Size:       int64(len(data)),
		UploadedAt: time.Now(),
	}, nil
}

func (s *localStorageAdapter) Download(ctx context.Context, key string) ([]byte, error) {
	return s.Storage.Download(ctx, key)
}

func (s *localStorageAdapter) SetBaseURL(baseURL string) {
	s.baseURL = baseURL
}

// NewS3Storage returns an S3 storage backend
func NewS3Storage(cfg storage.S3Config) (Storage, error) {
	s3Store, err := storage.NewS3(cfg)
	if err != nil {
		return nil, err
	}
	return &s3StorageAdapter{Storage: s3Store}, nil
}

type s3StorageAdapter struct {
	storage.Storage
	baseURL string
}

func (s *s3StorageAdapter) GetURL(ctx context.Context, name string) (string, error) {
	if s.baseURL != "" {
		return s.baseURL + "/" + name, nil
	}
	return s.Storage.GetURL(ctx, name)
}

func (s *s3StorageAdapter) SetBaseURL(baseURL string) {
	s.baseURL = baseURL
}

func (s *s3StorageAdapter) Download(ctx context.Context, key string) ([]byte, error) {
	return s.Storage.Download(ctx, key)
}

func (s *s3StorageAdapter) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	data, err := s.Storage.Download(ctx, name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *s3StorageAdapter) Save(ctx context.Context, name string, reader io.Reader) (*FileInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}
	if err := s.Storage.Upload(ctx, name, data); err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	return &FileInfo{
		Name:       name,
		Path:       name,
		URL:        name,
		Size:       int64(len(data)),
		UploadedAt: time.Now(),
	}, nil
}

// NewFileHandler returns an HTTP handler for file uploads
func NewFileHandler(store Storage) http.HandlerFunc {
	uploader := NewUploader(store)

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			name := r.URL.Query().Get("name")
			if name == "" {
				http.Error(w, "name required", 400)
				return
			}

			data, err := store.Download(r.Context(), name)
			if err != nil {
				http.Error(w, "file not found", 404)
				return
			}

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
			w.Write(data)

		case http.MethodPost:
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}

			info, err := uploader.Upload(r, "file")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}

			fmt.Fprintf(w, `{"url":"%s","size":%d}`, info.URL, info.Size)

		default:
			http.Error(w, "method not allowed", 405)
		}
	}
}
