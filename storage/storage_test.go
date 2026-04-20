package storage

import (
	"context"
	"os"
	"testing"
)

func TestNewLocal(t *testing.T) {
	store := NewLocal("/tmp/test-storage")
	if store == nil {
		t.Fatal("NewLocal returned nil")
	}
}

func TestLocal_Upload(t *testing.T) {
	store := NewLocal("/tmp/test-storage-upload")
	defer os.RemoveAll("/tmp/test-storage-upload")

	err := store.Upload(context.Background(), "test.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("Upload error = %v", err)
	}
}

func TestLocal_Download(t *testing.T) {
	store := NewLocal("/tmp/test-storage-download")
	os.MkdirAll("/tmp/test-storage-download", 0755)
	os.WriteFile("/tmp/test-storage-download/test.txt", []byte("hello"), 0644)
	defer os.RemoveAll("/tmp/test-storage-download")

	data, err := store.Download(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("Download error = %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("data = %s, want hello", string(data))
	}
}

func TestLocal_Delete(t *testing.T) {
	store := NewLocal("/tmp/test-storage-delete")
	os.MkdirAll("/tmp/test-storage-delete", 0755)
	os.WriteFile("/tmp/test-storage-delete/test.txt", []byte("hello"), 0644)
	defer os.RemoveAll("/tmp/test-storage-delete")

	err := store.Delete(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestLocal_Exists(t *testing.T) {
	store := NewLocal("/tmp/test-storage-exists")
	os.MkdirAll("/tmp/test-storage-exists", 0755)
	os.WriteFile("/tmp/test-storage-exists/test.txt", []byte("hello"), 0644)
	defer os.RemoveAll("/tmp/test-storage-exists")

	exists, err := store.Exists(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}
	if !exists {
		t.Error("file should exist")
	}
}

func TestLocal_NotExists(t *testing.T) {
	store := NewLocal("/tmp/test-storage-notexists")
	defer os.RemoveAll("/tmp/test-storage-notexists")

	exists, err := store.Exists(context.Background(), "nonexistent.txt")
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}
	if exists {
		t.Error("file should not exist")
	}
}

func TestLocal_List(t *testing.T) {
	store := NewLocal("/tmp/test-storage-list")
	os.MkdirAll("/tmp/test-storage-list", 0755)
	os.WriteFile("/tmp/test-storage-list/file1.txt", []byte("hello"), 0644)
	os.WriteFile("/tmp/test-storage-list/file2.txt", []byte("world"), 0644)
	defer os.RemoveAll("/tmp/test-storage-list")

	keys, err := store.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List error = %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("len(keys) = %d, want 2", len(keys))
	}
}
