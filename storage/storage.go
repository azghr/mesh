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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

	// GetURL returns a URL for the key (for S3, returns presigned URL)
	GetURL(ctx context.Context, key string) (string, error)
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

func (s *LocalStorage) GetURL(ctx context.Context, key string) (string, error) {
	return path.Join(s.root, key), nil
}

var _ Storage = (*S3Storage)(nil)

type S3Storage struct {
	client *s3.Client
	bucket string
}

type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

func NewS3(cfg S3Config) (*S3Storage, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	var endpoint string
	if cfg.Endpoint != "" {
		endpoint = cfg.Endpoint
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("S3 put object: %w", err)
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 get object: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("S3 delete object: %w", err)
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 list objects: %w", err)
	}

	var keys []string
	for _, obj := range resp.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}

func (s *S3Storage) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("S3 presign get: %w", err)
	}
	return req.URL, nil
}

func (s *S3Storage) PresignUpload(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("S3 presign upload: %w", err)
	}
	return req.URL, nil
}

func (s *S3Storage) GetURL(ctx context.Context, key string) (string, error) {
	return s.PresignGet(ctx, key, time.Hour)
}
