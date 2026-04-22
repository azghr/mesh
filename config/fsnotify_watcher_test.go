package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSWatcher(t *testing.T) {
	cfgFile := t.TempDir() + "/config.yaml"
	content := []byte(`server:
  port: "8080"
  host: "localhost"
  environment: "development"
database:
  host: "localhost"
  port: "5432"
  name: "testdb"
  user: "testuser"
  ssl_mode: "disable"`)

	err := os.WriteFile(cfgFile, content, 0644)
	require.NoError(t, err)

	t.Run("Start and Stop", func(t *testing.T) {
		watcher, err := NewFSWatcher(cfgFile)
		require.NoError(t, err)
		assert.False(t, watcher.IsRunning())

		watcher.Start()

		watcher.Stop()
		assert.False(t, watcher.IsRunning())
	})

	t.Run("Options", func(t *testing.T) {
		watcher, err := NewFSWatcher(cfgFile,
			WithDebounceInterval(50*time.Millisecond),
			WithFSCallback(func(cfg *Config) {
				assert.NotNil(t, cfg)
			}),
		)
		require.NoError(t, err)

		watcher.Start()

		watcher.Stop()
		assert.False(t, watcher.IsRunning())
	})
}

func TestFSConfigLoader(t *testing.T) {
	cfgFile := t.TempDir() + "/config.yaml"
	content := []byte(`server:
  port: "8080"
  host: "localhost"
database:
  host: "localhost"
  port: "5432"
  name: "testdb"
  user: "testuser"
  ssl_mode: "disable"`)

	err := os.WriteFile(cfgFile, content, 0644)
	require.NoError(t, err)

	t.Run("Load with FSWatcher", func(t *testing.T) {
		loader, err := LoadWithFSWatcher(cfgFile,
			WithAutoReloadInterval(100*time.Millisecond),
			WithOnChange(func(cfg *Config) {
				assert.NotNil(t, cfg)
			}),
		)
		require.NoError(t, err)
		assert.NotNil(t, loader.Get())

		time.Sleep(200 * time.Millisecond)
		loader.Stop()
	})

	t.Run("Stop stops watcher", func(t *testing.T) {
		loader, err := LoadWithFSWatcher(cfgFile,
			WithAutoReloadInterval(100*time.Millisecond),
		)
		require.NoError(t, err)

		assert.True(t, loader.IsReloading())

		loader.Stop()
		assert.False(t, loader.IsReloading())
	})
}
