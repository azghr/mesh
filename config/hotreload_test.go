package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWithHotReload_NotFound(t *testing.T) {
	_, err := LoadWithHotReload("nonexistent.yaml")
	assert.Error(t, err)
}

func TestHotReloadWatcher_New(t *testing.T) {
	watcher := &HotReloadWatcher{
		filePath: "test.yaml",
		interval: 5 * time.Second,
		callback: nil,
		stopCh:   make(chan struct{}),
	}

	assert.Equal(t, "test.yaml", watcher.filePath)
	assert.Equal(t, 5*time.Second, watcher.interval)
	assert.False(t, watcher.closed.Load())
}

func TestHotReloadWatcher_IsRunning(t *testing.T) {
	watcher := &HotReloadWatcher{
		stopCh: make(chan struct{}),
	}

	assert.True(t, watcher.IsRunning())
}

func TestHotReloadWatcher_Start(t *testing.T) {
	watcher := &HotReloadWatcher{
		filePath: "test.yaml",
		interval: 100 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	watcher.Start()
	assert.True(t, watcher.IsRunning())

	watcher.Stop()
	assert.False(t, watcher.IsRunning())
}

func TestConfigLoader_Get(t *testing.T) {
	cl := &ConfigLoader{}
	cl.config.Store(&Config{})

	cfg := cl.Get()
	assert.NotNil(t, cfg)
}

func TestConfigLoader_GetLatest(t *testing.T) {
	cl := &ConfigLoader{}
	cl.config.Store(&Config{})

	cfg := cl.GetLatest()
	assert.NotNil(t, cfg)
}

func TestConfigLoader_IsReloading(t *testing.T) {
	cl := &ConfigLoader{
		hotReloadEnabled: true,
	}

	assert.False(t, cl.IsReloading()) // watcher is nil initially
}

func TestNewHotReloadConfig(t *testing.T) {
	cfg := NewHotReloadConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Second, cfg.Interval)
	assert.Nil(t, cfg.OnChange)
}

func TestWithAutoReloadInterval(t *testing.T) {
	opt := WithAutoReloadInterval(10 * time.Second)
	cfg := NewHotReloadConfig()
	opt(&cfg)

	assert.Equal(t, 10*time.Second, cfg.Interval)
}

func TestWithOnChange(t *testing.T) {
	callbackCalled := false
	opt := WithOnChange(func(c *Config) {
		callbackCalled = true
	})
	cfg := NewHotReloadConfig()
	opt(&cfg)

	assert.NotNil(t, cfg.OnChange)
	cfg.OnChange(&Config{})
	assert.True(t, callbackCalled)
}

func TestLoadWithHotReload_Config(t *testing.T) {
	// Create a temp config file
	content := `
server:
  port: "8080"
  host: "localhost"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Load with hot reload
	cl, err := LoadWithHotReload(tmpFile.Name())
	require.NoError(t, err)

	assert.NotNil(t, cl)
	assert.NotNil(t, cl.Get())
	assert.Equal(t, "8080", cl.Get().Server.Port)

	// Cleanup
	cl.Stop()
}

func TestLoadWithHotReload_WithCallback(t *testing.T) {
	// Create a temp config file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("server:\n  port: \"8080\"\n")
	tmpFile.Close()

	var callbackCfg *Config
	cl, err := LoadWithHotReload(tmpFile.Name(), WithOnChange(func(c *Config) {
		callbackCfg = c
	}))
	require.NoError(t, err)

	assert.NotNil(t, cl)
	assert.NotNil(t, callbackCfg)
	assert.Equal(t, "8080", callbackCfg.Server.Port)

	cl.Stop()
}
