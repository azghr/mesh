// Package shutdown provides graceful shutdown management for services.
//
// This package implements service lifecycle management with proper dependency
// ordering, timeout handling, and error aggregation.
//
// Example:
//
//	mgr := shutdown.NewManager()
//	mgr.Register("database", func(ctx context.Context) error {
//	    return db.Close()
//	}, shutdown.WithDependsOn("redis"))
//	mgr.Register("redis", func(ctx context.Context) error {
//	    return redis.Close()
//	})
//
//	// On shutdown signal
//	mgr.Shutdown(context.Background())
//
// Graceful HTTP server shutdown:
//
//	err := shutdown.GracefulHTTP(server, shutdown.Config{
//	    Timeout: 30*time.Second,
//	    ShutdownHooks: []shutdown.Hook{
//	        func(ctx context.Context) error { return db.Close() },
//	    },
//	})
package shutdown

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ErrShutdownTimeout indicates shutdown did not complete within the timeout
var ErrShutdownTimeout = fmt.Errorf("shutdown timed out")

// ErrShutdownCancelled indicates shutdown was cancelled via context
var ErrShutdownCancelled = fmt.Errorf("shutdown was cancelled")

// Hook is a function called during graceful shutdown.
type Hook func(ctx context.Context) error

// Config holds configuration for GracefulHTTP.
type Config struct {
	// Timeout for draining active connections (default: 30s)
	Timeout time.Duration
	// ShutdownHooks are called after server stops accepting connections
	ShutdownHooks []Hook
	// PreShutdownHooks are called before stopping the server
	PreShutdownHooks []Hook
}

// Task represents a shutdown task
type Task struct {
	name      string
	fn        func(ctx context.Context) error
	dependsOn []string
	timeout   time.Duration
}

// Manager coordinates graceful shutdown of services in dependency order
type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	onShutdown  []func()
	shutdownErr error
	logger      *log.Logger
}

// Option configures the shutdown manager
type Option func(*Manager)

// WithLogger sets a custom logger
func WithLogger(logger *log.Logger) Option {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithTimeout sets the default timeout for all shutdown tasks
func WithTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, task := range m.tasks {
			if task.timeout == 0 {
				task.timeout = timeout
			}
		}
	}
}

// NewManager creates a new shutdown manager
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		tasks:  make(map[string]*Task),
		logger: log.Default(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// TaskOption configures a shutdown task
type TaskOption func(*Task)

// WithDependsOn specifies task dependencies (tasks that must complete first)
func WithDependsOn(deps ...string) TaskOption {
	return func(t *Task) {
		t.dependsOn = deps
	}
}

// WithTaskTimeout sets timeout for this specific task
func WithTaskTimeout(timeout time.Duration) TaskOption {
	return func(t *Task) {
		t.timeout = timeout
	}
}

// Register registers a shutdown task
func (m *Manager) Register(name string, fn func(ctx context.Context) error, opts ...TaskOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &Task{
		name:      name,
		fn:        fn,
		dependsOn: nil,
		timeout:   30 * time.Second, // default timeout
	}

	for _, opt := range opts {
		opt(task)
	}

	m.tasks[name] = task
}

// RegisterSimple registers a simple shutdown task without context
func (m *Manager) RegisterSimple(name string, fn func() error) {
	m.Register(name, func(ctx context.Context) error {
		return fn()
	})
}

// OnShutdown registers a callback to be called when shutdown begins
func (m *Manager) OnShutdown(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onShutdown = append(m.onShutdown, fn)
}

// Shutdown executes all registered shutdown tasks in dependency order
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	tasks := m.copyTasks()
	onShutdown := m.onShutdown
	m.mu.Unlock()

	// Call shutdown callbacks
	for _, fn := range onShutdown {
		fn()
	}

	if m.logger != nil {
		m.logger.Println("[shutdown] Starting graceful shutdown...")
	}

	// Sort tasks by dependency
	ordered := m.resolveDependencies(tasks)

	// Execute shutdown tasks sequentially (in dependency order)
	errCh := make(chan error, len(ordered))

	for _, task := range ordered {
		if err := m.executeTask(ctx, task); err != nil {
			errCh <- fmt.Errorf("%s: %w", task.name, err)
		}
	}

	close(errCh)

	// Collect errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		m.shutdownErr = fmt.Errorf("shutdown errors: %v", errors)
		if m.logger != nil {
			m.logger.Printf("[shutdown] Errors: %v", errors)
		}
		return m.shutdownErr
	}

	if m.logger != nil {
		m.logger.Println("[shutdown] Completed successfully")
	}

	return nil
}

// executeTask executes a single shutdown task with timeout
func (m *Manager) executeTask(ctx context.Context, task *Task) error {
	if m.logger != nil {
		m.logger.Printf("[shutdown] Stopping %s...", task.name)
	}

	// Create task-specific context with timeout
	taskCtx := ctx
	if task.timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.timeout)
		defer cancel()
	}

	err := task.fn(taskCtx)
	if err != nil {
		if m.logger != nil {
			m.logger.Printf("[shutdown] %s failed: %v", task.name, err)
		}
		return err
	}

	if m.logger != nil {
		m.logger.Printf("[shutdown] %s stopped", task.name)
	}

	return nil
}

// resolveDependencies orders tasks by dependency
func (m *Manager) resolveDependencies(tasks map[string]*Task) []*Task {
	// Build dependency graph
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for name, task := range tasks {
		inDegree[name] = len(task.dependsOn)
		graph[name] = task.dependsOn
	}

	// Topological sort (Kahn's algorithm)
	var result []*Task
	queue := make([]string, 0)

	// Find nodes with no dependencies
	for name := range tasks {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		// Pop from front
		current := queue[0]
		queue = queue[1:]
		result = append(result, tasks[current])

		// Reduce in-degree for dependents
		for name, deps := range graph {
			for _, dep := range deps {
				if dep == current {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = append(queue, name)
					}
				}
			}
		}
	}

	// If we have remaining tasks, there's a cycle - fallback to arbitrary order
	if len(result) < len(tasks) {
		for _, task := range tasks {
			exists := false
			for _, t := range result {
				if t.name == task.name {
					exists = true
					break
				}
			}
			if !exists {
				result = append(result, task)
			}
		}
	}

	return result
}

// copyTasks returns a copy of the tasks map
func (m *Manager) copyTasks() map[string]*Task {
	tasks := make(map[string]*Task)
	for k, v := range m.tasks {
		tasks[k] = v
	}
	return tasks
}

// Error returns the shutdown error if any
func (m *Manager) Error() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shutdownErr
}

// WaitForSignal waits for termination signals and executes shutdown
func (m *Manager) WaitForSignal(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case sig := <-sigCh:
		if m.logger != nil {
			m.logger.Printf("[shutdown] Received signal: %v", sig)
		}
		return m.Shutdown(context.Background())
	}
}

// Tasks returns the list of registered task names
func (m *Manager) Tasks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.tasks))
	for name := range m.tasks {
		names = append(names, name)
	}
	return names
}

// GracefulHTTP performs graceful shutdown of an HTTP server.
//
// It stops accepting new connections, waits for active requests to complete,
// then runs shutdown hooks.
//
// Example:
//
//	err := shutdown.GracefulHTTP(server, shutdown.Config{
//	    Timeout: 30*time.Second,
//	    ShutdownHooks: []shutdown.Hook{
//	        func(ctx context.Context) error { return db.Close() },
//	    },
//	})
func GracefulHTTP(srv *http.Server, cfg Config) error {
	// Set default timeout
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Run pre-shutdown hooks first
	for _, hook := range cfg.PreShutdownHooks {
		if err := hook(context.Background()); err != nil {
			log.Printf("[shutdown] pre-hook error: %v", err)
		}
	}

	// Stop accepting new connections
	srv.SetKeepAlivesEnabled(false)

	// Close the listener to unblock Serve()
	ln := srv.Addr
	if ln == "" {
		ln = ":http"
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Channel for server error
	done := make(chan error, 1)

	go func() {
		done <- srv.Shutdown(ctx)
	}()

	// Wait for either shutdown complete or timeout
	select {
	case <-ctx.Done():
		cancel()
		return ErrShutdownTimeout
	case err := <-done:
		if err != nil {
			log.Printf("[shutdown] HTTP server: %v", err)
		}
	}

	// Run shutdown hooks
	for _, hook := range cfg.ShutdownHooks {
		if err := hook(ctx); err != nil {
			log.Printf("[shutdown] hook error: %v", err)
			return err
		}
	}

	return nil
}
