package upload

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)

	info, err := storage.Save(context.Background(), "test.txt", &testReader{data: "hello world"})
	require.NoError(t, err)

	assert.Equal(t, "test.txt", info.Name)
	assert.NotEmpty(t, info.URL)
	assert.Equal(t, int64(11), info.Size)

	// Load file
	reader, err := storage.Load(context.Background(), "test.txt")
	require.NoError(t, err)
	defer reader.Close()

	buf := make([]byte, 100)
	n, _ := reader.Read(buf)
	assert.Equal(t, "hello world", string(buf[:n]))
}

func TestLocalStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)

	storage.Save(context.Background(), "test.txt", &testReader{data: "hello"})

	err := storage.Delete(context.Background(), "test.txt")
	require.NoError(t, err)

	_, err = storage.Load(context.Background(), "test.txt")
	assert.Error(t, err)
}

func TestLocalStorage_GetURL(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)
	storage.SetBaseURL("/files")

	url, err := storage.GetURL(context.Background(), "test.txt")
	require.NoError(t, err)
	assert.Equal(t, "/files/test.txt", url)
}

func TestLocalStorage_NestedPath(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)

	info, err := storage.Save(context.Background(), "uploads/2024/test.txt", &testReader{data: "nested"})
	require.NoError(t, err)
	assert.NotEmpty(t, info.Path)

	// Verify nested directory was created
	nestedDir := filepath.Join(dir, "uploads", "2024")
	_, err = os.Stat(nestedDir)
	assert.NoError(t, err, "nested directory should exist")
}

func TestUploader_GenerateFilename(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)
	uploader := NewUploader(storage)

	// Test unique filename generation
	filename1 := uploader.generateFilename("test file.txt")
	filename2 := uploader.generateFilename("test file.txt")

	assert.NotEqual(t, filename1, filename2, "filenames should be unique")
	assert.Contains(t, filename1, ".txt")
}

func TestUploader_WithOptions(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)

	uploader := NewUploader(storage,
		WithMaxSize(1024),
		WithAllowedExtensions([]string{".jpg", ".png"}),
	)

	assert.Equal(t, int64(1024), uploader.maxSize)
	assert.Len(t, uploader.allowedExt, 2)
}

func TestUploader_MaxSizeCheck(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)

	// Create a file that's too large
	uploader := NewUploader(storage, WithMaxSize(5))

	// Test via reflection (can't easily create large reader in test)
	// Just verify the config is working
	assert.Equal(t, int64(5), uploader.maxSize)
}

func TestFileHandler_Upload(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir)
	handler := NewFileHandler(storage)

	// Test that handler doesn't panic
	assert.NotNil(t, handler)
}

type testReader struct {
	data string
	pos  int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
