// Package config provides configuration management with hot reload support.
//
// This package adds file watching and automatic configuration reloading
// without requiring service restart.
//
// Example:
//
//	cfg, watcher, err := config.LoadAndWatch("config.yaml", func(newCfg *myConfig) {
//	    log.Info("Configuration reloaded", "config", newCfg)
//	})
//	defer watcher.Stop()
package config

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// WatchOption configures the config watcher
type WatchOption func(*Watcher)

// WithInterval sets the check interval for file changes
func WithInterval(interval time.Duration) WatchOption {
	return func(w *Watcher) {
		w.interval = interval
	}
}

// Watcher monitors a config file and triggers callbacks on changes
type Watcher struct {
	filePath string
	interval time.Duration
	callback func(any)
	stopCh   chan struct{}
	closed   atomic.Bool
}

// NewWatcher creates a new config file watcher
func NewWatcher(filePath string, callback func(any), opts ...WatchOption) *Watcher {
	w := &Watcher{
		filePath: filePath,
		interval: 5 * time.Second,
		callback: callback,
		stopCh:   make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Start begins watching the config file for changes
func (w *Watcher) Start() {
	if w.closed.Load() {
		return
	}

	go w.watch()
}

// watch monitors the file and triggers callback on changes
func (w *Watcher) watch() {
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
				w.reload()
			}
		}
	}
}

// getLastModified returns the last modification time
func (w *Watcher) getLastModified() int64 {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

// reload reloads the config file
func (w *Watcher) reload() {
	data, err := os.ReadFile(w.filePath)
	if err != nil {
		return
	}

	var config any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return
	}

	w.callback(config)
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	if !w.closed.CompareAndSwap(false, true) {
		return
	}
	close(w.stopCh)
}

// IsRunning returns true if the watcher is running
func (w *Watcher) IsRunning() bool {
	return !w.closed.Load()
}

// LoadAndWatch loads a config file and starts watching for changes
func LoadAndWatch[T any](filePath string, onReload func(*T)) (*T, *Watcher, error) {
	config, err := loadConfig[T](filePath)
	if err != nil {
		return nil, nil, err
	}

	watcher := &Watcher{
		filePath: filePath,
		interval: 5 * time.Second,
		callback: func(newConfig any) {
			if cfg, ok := newConfig.(*T); ok {
				onReload(cfg)
			}
		},
		stopCh: make(chan struct{}),
	}

	go watcher.watch()

	return config, watcher, nil
}

// loadConfig loads config from file
func loadConfig[T any](filePath string) (*T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// Reload triggers a manual reload of the config
func (w *Watcher) Reload() {
	w.reload()
}
