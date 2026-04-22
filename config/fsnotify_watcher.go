// Package config provides configuration management with file system event-based hot reload.
//
// This package uses fsnotify for real-time file system notifications,
// providing faster config reloads than polling-based approaches.
//
// Example:
//
//	cfg, err := config.LoadWithFSWatcher("config.yaml",
//	    config.WithOnChange(func(newCfg *config.Config) {
//	        log.Info("config reloaded", "environment", newCfg.Server.Environment)
//	    }),
//	)
//
// Key features:
// - Real-time file system notifications via fsnotify
// - Debounced reloads to handle rapid file changes
// - Thread-safe config access
package config

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// FSWatcher watches a config file for changes using fsnotify.
type FSWatcher struct {
	filePath     string
	debounce     time.Duration
	callback     func(*Config)
	stopCh       chan struct{}
	running      atomic.Bool
	wg           sync.WaitGroup
	watcher      *fsnotify.Watcher
	lastLoadTime time.Time
	mu           sync.Mutex
}

// FSWatcherOption configures FSWatcher options.
type FSWatcherOption func(*FSWatcher)

// WithDebounceInterval sets the debounce interval for file changes.
func WithDebounceInterval(d time.Duration) FSWatcherOption {
	return func(w *FSWatcher) {
		w.debounce = d
	}
}

// WithFSCallback sets the callback for config changes.
func WithFSCallback(callback func(*Config)) FSWatcherOption {
	return func(w *FSWatcher) {
		w.callback = callback
	}
}

// NewFSWatcher creates a new FSWatcher for the given config file.
func NewFSWatcher(filePath string, opts ...FSWatcherOption) (*FSWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	fsw := &FSWatcher{
		filePath:     filePath,
		debounce:     100 * time.Millisecond,
		stopCh:       make(chan struct{}),
		watcher:      w,
		lastLoadTime: time.Now(),
	}

	for _, opt := range opts {
		opt(fsw)
	}

	if err := w.Add(filePath); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	return fsw, nil
}

// Start begins watching the config file for changes.
func (w *FSWatcher) Start() {
	if w.running.Load() {
		return
	}
	w.running.Store(true)

	w.wg.Add(1)
	go w.watch()
}

// watch monitors for file system events.
func (w *FSWatcher) watch() {
	defer w.wg.Done()
	w.running.Store(false)

	debounceTimer := time.NewTimer(w.debounce)
	debounceTimer.Stop()

	pendingReload := false

	for {
		select {
		case <-w.stopCh:
			return
		case event := <-w.watcher.Events:
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				pendingReload = true
				debounceTimer.Reset(w.debounce)
			}
		case <-debounceTimer.C:
			if pendingReload {
				pendingReload = false
				w.doReload()
			}
		case err := <-w.watcher.Errors:
			if err != nil {
				fmt.Printf("FSWatcher error: %v\n", err)
			}
		}
	}
}

// doReload reloads the config file and calls the callback.
func (w *FSWatcher) doReload() {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := os.ReadFile(w.filePath)
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		return
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("Error parsing config: %v\n", err)
		return
	}

	applyEnvOverrides(&cfg)
	w.lastLoadTime = time.Now()

	if w.callback != nil {
		w.callback(&cfg)
	}
}

// Stop stops the watcher.
func (w *FSWatcher) Stop() {
	if !w.running.CompareAndSwap(true, false) {
		return
	}

	close(w.stopCh)
	w.wg.Wait()
	w.watcher.Close()
}

// IsRunning returns true if the watcher is running.
func (w *FSWatcher) IsRunning() bool {
	return w.running.Load()
}

// Reload triggers a manual reload.
func (w *FSWatcher) Reload() {
	w.doReload()
}

// FSConfigLoader loads and watches config with file system notifications.
type FSConfigLoader struct {
	config   atomic.Pointer[Config]
	filePath string
	watcher  *FSWatcher
	callback func(*Config)
}

// LoadWithFSWatcher loads config with file system event-based hot reload.
func LoadWithFSWatcher(filePath string, opts ...HotReloadOption) (*FSConfigLoader, error) {
	hotCfg := NewHotReloadConfig()
	for _, opt := range opts {
		opt(&hotCfg)
	}

	cl := &FSConfigLoader{
		filePath: filePath,
		callback: hotCfg.OnChange,
	}

	cfg, err := loadConfigFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	cl.config.Store(cfg)

	if hotCfg.Enabled {
		watcher, err := NewFSWatcher(filePath,
			WithDebounceInterval(hotCfg.Interval),
			WithFSCallback(func(newCfg *Config) {
				oldCfg := cl.config.Load()
				cl.config.Store(newCfg)

				if cl.callback != nil && newCfg != nil {
					cl.callback(newCfg)
				}

				if oldCfg != nil && newCfg != nil {
					fmt.Printf("Config reloaded: %s\n", filePath)
				}
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create watcher: %w", err)
		}

		cl.watcher = watcher
		watcher.Start()
	}

	return cl, nil
}

// Get returns the current config.
func (cl *FSConfigLoader) Get() *Config {
	return cl.config.Load()
}

// GetLatest returns the latest config.
func (cl *FSConfigLoader) GetLatest() *Config {
	return cl.Get()
}

// Stop stops the file watcher.
func (cl *FSConfigLoader) Stop() {
	if cl.watcher != nil {
		cl.watcher.Stop()
	}
}

// IsReloading returns true if file watching is enabled.
func (cl *FSConfigLoader) IsReloading() bool {
	return cl.watcher != nil && cl.watcher.IsRunning()
}
