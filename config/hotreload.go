// Package config provides configuration management with hot reload support.
//
// This package adds automatic configuration reloading without requiring service restart.
package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// HotReloadConfig holds configuration for hot reload functionality
type HotReloadConfig struct {
	Enabled  bool          // Enable automatic reload
	Interval time.Duration // How often to check for changes
	OnChange func(*Config) // Callback when config changes
}

// HotReloadOption configures the hot reload functionality
type HotReloadOption func(*HotReloadConfig)

// WithAutoReloadInterval sets the interval for checking config changes
func WithAutoReloadInterval(interval time.Duration) HotReloadOption {
	return func(c *HotReloadConfig) {
		c.Interval = interval
	}
}

// WithOnChange sets the callback for config changes
func WithOnChange(callback func(*Config)) HotReloadOption {
	return func(c *HotReloadConfig) {
		c.OnChange = callback
	}
}

// NewHotReloadConfig creates a default hot reload config
func NewHotReloadConfig() HotReloadConfig {
	return HotReloadConfig{
		Enabled:  true,
		Interval: 30 * time.Second,
		OnChange: nil,
	}
}

// ConfigLoader holds the current config and handles hot reload
type ConfigLoader struct {
	config            atomic.Pointer[Config]
	filePath          string
	hotReloadEnabled  bool
	hotReloadInterval time.Duration
	hotReloadCallback func(*Config)
	hotReloadWatcher  *HotReloadWatcher
	mu                sync.RWMutex
}

// HotReloadWatcher watches for file changes
type HotReloadWatcher struct {
	filePath string
	interval time.Duration
	callback func(*Config)
	stopCh   chan struct{}
	closed   atomic.Bool
	wg       sync.WaitGroup
}

// LoadWithHotReload loads configuration from a YAML file with automatic reload
//
// Example:
//
//	cfg, err := config.LoadWithHotReload("config.yaml",
//	    config.WithAutoReloadInterval(30*time.Second),
//	    config.WithOnChange(func(newCfg *config.Config) {
//	        log.Info("config reloaded", "environment", newCfg.Server.Environment)
//	    }),
//	)
func LoadWithHotReload(filePath string, opts ...HotReloadOption) (*ConfigLoader, error) {
	hotCfg := NewHotReloadConfig()
	for _, opt := range opts {
		opt(&hotCfg)
	}

	cl := &ConfigLoader{
		filePath:          filePath,
		hotReloadEnabled:  hotCfg.Enabled,
		hotReloadInterval: hotCfg.Interval,
		hotReloadCallback: hotCfg.OnChange,
	}

	// Load initial config
	cfg, err := loadConfigFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	cl.config.Store(cfg)

	// Start hot reload if enabled
	if cl.hotReloadEnabled {
		cl.startHotReload()
	}

	return cl, nil
}

// startHotReload starts the background watcher
func (cl *ConfigLoader) startHotReload() {
	watcher := &HotReloadWatcher{
		filePath: cl.filePath,
		interval: cl.hotReloadInterval,
		callback: cl.reloadConfig,
		stopCh:   make(chan struct{}),
	}
	cl.hotReloadWatcher = watcher

	// Call onChange immediately with initial config if callback is set
	if cl.hotReloadCallback != nil {
		cl.hotReloadCallback(cl.config.Load())
	}

	watcher.Start()
}

// reloadConfig reloads the config from file
func (cl *ConfigLoader) reloadConfig(newCfg *Config) {
	oldCfg := cl.config.Load()
	cl.config.Store(newCfg)

	if cl.hotReloadCallback != nil && newCfg != nil {
		cl.hotReloadCallback(newCfg)
	}

	if oldCfg != nil && newCfg != nil {
		fmt.Printf("Config reloaded: %s\n", cl.filePath)
	}
}

// Get returns the current config (thread-safe)
func (cl *ConfigLoader) Get() *Config {
	return cl.config.Load()
}

// GetLatest returns the latest config (alias for Get)
func (cl *ConfigLoader) GetLatest() *Config {
	return cl.Get()
}

// GetWithLock returns a copy of the current config
func (cl *ConfigLoader) GetWithLock() Config {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	if cfg := cl.config.Load(); cfg != nil {
		return *cfg
	}
	return Config{}
}

// Stop stops the hot reload watcher
func (cl *ConfigLoader) Stop() {
	if cl.hotReloadWatcher != nil {
		cl.hotReloadWatcher.Stop()
	}
}

// Start starts the hot reload watcher (if stopped)
func (cl *ConfigLoader) Start() {
	if cl.hotReloadWatcher != nil && cl.hotReloadWatcher.closed.Load() {
		cl.hotReloadWatcher = &HotReloadWatcher{
			filePath: cl.filePath,
			interval: cl.hotReloadInterval,
			callback: cl.reloadConfig,
			stopCh:   make(chan struct{}),
		}
		cl.hotReloadWatcher.Start()
	}
}

// IsReloading returns true if hot reload is enabled
func (cl *ConfigLoader) IsReloading() bool {
	return cl.hotReloadEnabled && cl.hotReloadWatcher != nil && !cl.hotReloadWatcher.closed.Load()
}

// Start begins watching for config changes
func (w *HotReloadWatcher) Start() {
	if w.closed.Load() {
		return
	}

	w.wg.Add(1)
	go w.watch()
}

// watch monitors the config file for changes
func (w *HotReloadWatcher) watch() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	lastMod := w.getLastModified()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			currentMod := w.getLastModified()
			if currentMod > lastMod {
				lastMod = currentMod
				w.doReload()
			}
		}
	}
}

// getLastModified returns the last modification time of the config file
func (w *HotReloadWatcher) getLastModified() int64 {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

// doReload reloads the config file and calls the callback
func (w *HotReloadWatcher) doReload() {
	cfg, err := loadConfigFile(w.filePath)
	if err != nil {
		fmt.Printf("Error reloading config: %v\n", err)
		return
	}

	if w.callback != nil {
		w.callback(cfg)
	}
}

// Stop stops the watcher
func (w *HotReloadWatcher) Stop() {
	if !w.closed.CompareAndSwap(false, true) {
		return
	}
	close(w.stopCh)
	w.wg.Wait()
}

// IsRunning returns true if the watcher is running
func (w *HotReloadWatcher) IsRunning() bool {
	return !w.closed.Load()
}

// loadConfigFile loads configuration from a YAML file
func loadConfigFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.Database.Port = v
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.PortInt = port
		}
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = port
		}
	}
}
