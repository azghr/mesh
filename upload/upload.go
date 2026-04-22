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
package upload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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
}

// Uploader handles file uploads
type Uploader struct {
	maxSize    int64
	allowedExt []string
	storage    Storage
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

// LocalStorage implements local filesystem storage
type LocalStorage struct {
	basePath string
	baseURL  string
}

// NewLocalStorage creates local filesystem storage
func NewLocalStorage(basePath string) *LocalStorage {
	// Create directory if not exists
	os.MkdirAll(basePath, 0755)

	return &LocalStorage{
		basePath: basePath,
		baseURL:  "/uploads",
	}
}

// SetBaseURL sets the base URL for file access
func (ls *LocalStorage) SetBaseURL(url string) *LocalStorage {
	ls.baseURL = url
	return ls
}

// Save saves file to local filesystem
func (ls *LocalStorage) Save(ctx context.Context, name string, reader io.Reader) (*FileInfo, error) {
	filepath := filepath.Join(ls.basePath, name)

	// Create directory
	dir := path.Dir(filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("MkdirAll: %w", err)
	}

	// Create file
	file, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("Create: %w", err)
	}
	defer file.Close()

	// Copy content
	written, err := io.Copy(file, reader)
	if err != nil {
		return nil, fmt.Errorf("Copy: %w", err)
	}

	return &FileInfo{
		Name:       name,
		Path:       filepath,
		URL:        path.Join(ls.baseURL, name),
		Size:       written,
		UploadedAt: time.Now(),
	}, nil
}

// Load reads file from local filesystem
func (ls *LocalStorage) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	filepath := filepath.Join(ls.basePath, name)
	return os.Open(filepath)
}

// Delete removes file from local filesystem
func (ls *LocalStorage) Delete(ctx context.Context, name string) error {
	filepath := filepath.Join(ls.basePath, name)
	return os.Remove(filepath)
}

// GetURL returns the URL for a file
func (ls *LocalStorage) GetURL(ctx context.Context, name string) (string, error) {
	return path.Join(ls.baseURL, name), nil
}

// NewFileHandler returns an HTTP handler for file uploads
func NewFileHandler(storage Storage) http.HandlerFunc {
	uploader := NewUploader(storage)

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Download file
			name := r.URL.Query().Get("name")
			if name == "" {
				http.Error(w, "name required", 400)
				return
			}

			reader, err := storage.Load(r.Context(), name)
			if err != nil {
				http.Error(w, "file not found", 404)
				return
			}
			defer reader.Close()

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
			io.Copy(w, reader)

		case http.MethodPost:
			// Upload file
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
