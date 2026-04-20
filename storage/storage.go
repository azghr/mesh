// Package storage provides file storage abstractions for local filesystem and S3.
//
// This package provides a unified interface for storing and retrieving files
// from either local filesystem or S3-compatible storage.
//
// # Features
//
//   - Local filesystem or S3 backend
//   - Upload, download, delete operations
//   - Presigned URLs for secure access (S3 only)
//   - Multiple storage backends
//
// # Usage
//
//	// Local storage
//	store := storage.NewLocal("/tmp/uploads")
//	store.Upload(ctx, "file.txt", data)
//
//	// S3 storage
//	store, _ := storage.NewS3(storage.S3Config{
//	    Bucket: "my-bucket",
//	    Region: "us-east-1",
//	})
//	store.Upload(ctx, "file.txt", data)
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Storage defines the interface for file storage
type Storage interface {
	// Upload uploads data to the specified key
	Upload(ctx context.Context, key string, data []byte) error

	// Download downloads data from the specified key
	Download(ctx context.Context, key string) ([]byte, error)

	// Delete deletes the specified key
	Delete(ctx context.Context, key string) error

	// Exists checks if the key exists
	Exists(ctx context.Context, key string) (bool, error)

	// List lists keys with the given prefix
	List(ctx context.Context, prefix string) ([]string, error)
}

// ensure LocalStorage implements Storage
var _ Storage = (*LocalStorage)(nil)

// LocalStorage provides local filesystem storage
type LocalStorage struct {
	root string
}

// NewLocal creates a new local storage backend
//
//	store := storage.NewLocal("/tmp/uploads")
func NewLocal(root string) *LocalStorage {
	os.MkdirAll(root, 0755)
	return &LocalStorage{root: root}
}

// Upload uploads data to local storage
func (s *LocalStorage) Upload(ctx context.Context, key string, data []byte) error {
	path := filepath.Join(s.root, key)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Download downloads data from local storage
func (s *LocalStorage) Download(ctx context.Context, key string) ([]byte, error) {
	path := filepath.Join(s.root, key)
	return os.ReadFile(path)
}

// Delete deletes a file from local storage
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path := filepath.Join(s.root, key)
	return os.Remove(path)
}

// Exists checks if file exists
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(s.root, key)
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// List lists files with prefix
func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	search := filepath.Join(s.root, prefix)

	entries, err := os.ReadDir(search)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			rel, _ := filepath.Rel(s.root, filepath.Join(search, entry.Name()))
			keys = append(keys, rel)
		}
	}

	return keys, nil
}

// PresignGet is not supported for local storage
func (s *LocalStorage) PresignGet(ctx context.Context, key string, expiry interface{}) (string, error) {
	return "", errors.New("presigned URLs not supported for local storage")
}

// PresignUpload is not supported for local storage
func (s *LocalStorage) PresignUpload(ctx context.Context, key string, expiry interface{}) (string, error) {
	return "", errors.New("presigned URLs not supported for local storage")
}
