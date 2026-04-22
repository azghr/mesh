// Package shutdown provides enhanced graceful shutdown management.
//
// This package adds health checks, connection draining, and ordered shutdown phases
// for production-ready service lifecycle management.
package shutdown

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type (
	// EnhancedManager provides enhanced shutdown management with phases
	EnhancedManager struct {
		mu           sync.RWMutex
		tasks        map[string]*Task
		phases       []*Phase
		onShutdown   []func()
		shutdownErr  error
		logger       *log.Logger
		connTracker  *ConnTracker
		healthChecks *HealthChecker
	}

	// Phase represents a shutdown phase
	Phase struct {
		Name   string
		Tasks  []string
		Order  int
		Strict bool // If true, all tasks must succeed
	}

	// ConnTracker tracks active connections for draining
	ConnTracker struct {
		active   atomic.Int64
		draining atomic.Bool
		mu       sync.Mutex
	}

	// HealthChecker provides health check endpoints
	HealthChecker struct {
		mu             sync.RWMutex
		ready          atomic.Bool
		healthy        atomic.Bool
		checkers       []HealthCheck
		beforeShutdown []func() error
		afterShutdown  []func() error
	}

	// HealthCheck is a function that checks component health
	HealthCheck func(ctx context.Context) error
)

// NewEnhancedManager creates a new enhanced shutdown manager
func NewEnhancedManager() *EnhancedManager {
	return &EnhancedManager{
		tasks:       make(map[string]*Task),
		phases:      make([]*Phase, 0),
		logger:      log.Default(),
		connTracker: &ConnTracker{},
		healthChecks: &HealthChecker{
			healthy: atomic.Bool{},
			ready:   atomic.Bool{},
		},
	}
}

// ConnTracker methods

// RegisterConnection registers an active connection
func (ct *ConnTracker) RegisterConnection() {
	if !ct.draining.Load() {
		ct.active.Add(1)
	}
}

// UnregisterConnection unregisters a connection
func (ct *ConnTracker) UnregisterConnection() {
	ct.active.Add(-1)
}

// ActiveConnections returns the current number of active connections
func (ct *ConnTracker) ActiveConnections() int {
	return int(ct.active.Load())
}

// StartDraining signals that draining has begun
func (ct *ConnTracker) StartDraining() {
	ct.draining.Store(true)
}

// IsDraining returns true if draining is in progress
func (ct *ConnTracker) IsDraining() bool {
	return ct.draining.Load()
}

// WaitForDrain waits for all connections to drain
func (ct *ConnTracker) WaitForDrain(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for ct.active.Load() > 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for connections to drain: %d remaining", ct.active.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// HealthChecker methods

// SetHealthy sets the healthy status
func (hc *HealthChecker) SetHealthy(healthy bool) {
	hc.healthy.Store(healthy)
}

// SetReady sets the ready status
func (hc *HealthChecker) SetReady(ready bool) {
	hc.ready.Store(ready)
}

// IsHealthy returns the healthy status
func (hc *HealthChecker) IsHealthy() bool {
	return hc.healthy.Load()
}

// IsReady returns the ready status
func (hc *HealthChecker) IsReady() bool {
	return hc.ready.Load()
}

// AddHealthCheck adds a health check function
func (hc *HealthChecker) AddHealthCheck(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checkers = append(hc.checkers, check)
}

// AddBeforeShutdown adds a function to run before shutdown
func (hc *HealthChecker) AddBeforeShutdown(fn func() error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.beforeShutdown = append(hc.beforeShutdown, fn)
}

// AddAfterShutdown adds a function to run after shutdown
func (hc *HealthChecker) AddAfterShutdown(fn func() error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.afterShutdown = append(hc.afterShutdown, fn)
}

// RunHealthChecks runs all health checks
func (hc *HealthChecker) RunHealthChecks(ctx context.Context) error {
	hc.mu.RLock()
	checkers := hc.checkers
	hc.mu.RUnlock()

	for _, check := range checkers {
		if err := check(ctx); err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
	}
	return nil
}

// RunBeforeShutdown runs all before-shutdown functions
func (hc *HealthChecker) RunBeforeShutdown() error {
	hc.mu.RLock()
	fns := hc.beforeShutdown
	hc.mu.RUnlock()

	for _, fn := range fns {
		if err := fn(); err != nil {
			return fmt.Errorf("before shutdown failed: %w", err)
		}
	}
	return nil
}

// RunAfterShutdown runs all after-shutdown functions
func (hc *HealthChecker) RunAfterShutdown() error {
	hc.mu.RLock()
	fns := hc.afterShutdown
	hc.mu.RUnlock()

	for _, fn := range fns {
		if err := fn(); err != nil {
			return fmt.Errorf("after shutdown failed: %w", err)
		}
	}
	return nil
}

// EnhancedManager methods

// copyTasks returns a copy of tasks map
func (m *EnhancedManager) copyTasks() map[string]*Task {
	tasks := make(map[string]*Task)
	for k, v := range m.tasks {
		tasks[k] = v
	}
	return tasks
}

// executeTask executes a single task with timeout
func (m *EnhancedManager) executeTask(ctx context.Context, task *Task) error {
	if m.logger != nil {
		m.logger.Printf("[shutdown] Stopping %s...", task.name)
	}

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

// AddPhase adds a shutdown phase
func (m *EnhancedManager) AddPhase(name string, tasks []string, order int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phases = append(m.phases, &Phase{
		Name:  name,
		Tasks: tasks,
		Order: order,
	})
}

// RegisterTask registers a shutdown task
func (m *EnhancedManager) RegisterTask(name string, fn func(ctx context.Context) error, opts ...TaskOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &Task{
		name:    name,
		fn:      fn,
		timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(task)
	}

	m.tasks[name] = task
}

// WaitForSignal waits for termination signals and executes shutdown
func (m *EnhancedManager) WaitForSignal(ctx context.Context) error {
	// Import and use the signal handler
	return m.waitForSignal(ctx)
}

// waitForSignal waits for termination signals
func (m *EnhancedManager) waitForSignal(ctx context.Context) error {
	sigCh := make(chan error, 1)

	go func() {
		err := m.Shutdown(ctx)
		sigCh <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-sigCh:
		return err
	}
}

// AddHealthCheck adds a health check
func (m *EnhancedManager) AddHealthCheck(check HealthCheck) {
	m.healthChecks.AddHealthCheck(check)
}

// AddBeforeShutdown adds a before-shutdown function
func (m *EnhancedManager) AddBeforeShutdown(fn func() error) {
	m.healthChecks.AddBeforeShutdown(fn)
}

// AddAfterShutdown adds an after-shutdown function
func (m *EnhancedManager) AddAfterShutdown(fn func() error) {
	m.healthChecks.AddAfterShutdown(fn)
}

// Shutdown performs phased shutdown
func (m *EnhancedManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	tasks := m.copyTasks()
	phases := m.phases
	m.mu.Unlock()

	// Start draining
	m.connTracker.StartDraining()

	// Run before-shutdown hooks
	if err := m.healthChecks.RunBeforeShutdown(); err != nil {
		m.logger.Printf("[shutdown] before shutdown error: %v", err)
	}

	// Set not ready
	m.healthChecks.SetReady(false)

	// Wait for connections to drain
	if err := m.connTracker.WaitForDrain(30 * time.Second); err != nil {
		m.logger.Printf("[shutdown] drain warning: %v", err)
	}

	// Execute phases in order
	var errors []error

	// Sort phases
	for i := range phases {
		for j := i + 1; len(phases) > 1; j++ {
			if phases[j].Order < phases[i].Order {
				phases[i], phases[j] = phases[j], phases[i]
			}
		}
	}

	for _, phase := range phases {
		for _, taskName := range phase.Tasks {
			task, ok := tasks[taskName]
			if !ok {
				continue
			}

			if err := m.executeTask(ctx, task); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", task.name, err))
				if phase.Strict {
					m.shutdownErr = fmt.Errorf("phase %s failed: %v", phase.Name, errors)
					return m.shutdownErr
				}
			}
		}
	}

	// Run after-shutdown hooks
	if err := m.healthChecks.RunAfterShutdown(); err != nil {
		m.logger.Printf("[shutdown] after shutdown error: %v", err)
	}

	if len(errors) > 0 {
		m.shutdownErr = fmt.Errorf("shutdown errors: %v", errors)
		return m.shutdownErr
	}

	return nil
}

// HTTPHandler returns an HTTP handler for health checks
func (m *EnhancedManager) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","connections":%d}`, m.connTracker.ActiveConnections())
		case "/ready":
			if m.healthChecks.IsReady() && m.connTracker.ActiveConnections() == 0 {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"status":"ready","connections":%d}`, m.connTracker.ActiveConnections())
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, `{"status":"not_ready","draining":%v}`, m.connTracker.IsDraining())
			}
		case "/drain":
			m.connTracker.StartDraining()
			m.healthChecks.SetReady(false)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"draining_started"}`)
		default:
			http.NotFound(w, r)
		}
	}
}

// GetConnTracker returns the connection tracker
func (m *EnhancedManager) GetConnTracker() *ConnTracker {
	return m.connTracker
}

// GetHealthChecker returns the health checker
func (m *EnhancedManager) GetHealthChecker() *HealthChecker {
	return m.healthChecks
}

// DrainConfig holds configuration for graceful HTTP draining
type DrainConfig struct {
	// Timeout for draining connections
	Timeout time.Duration
	// GracePeriod is time after no new connections before force close
	GracePeriod time.Duration
	// OnDrainStarted is called when draining begins
	OnDrainStarted func()
	// OnAllDrained is called when all connections close
	OnAllDrained func()
}

// GracefulDrain performs graceful drain of an HTTP server
func GracefulDrain(srv *http.Server, cfg DrainConfig) error {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Start draining - stop accepting new connections
	srv.SetKeepAlivesEnabled(false)

	if cfg.OnDrainStarted != nil {
		cfg.OnDrainStarted()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Channel for server shutdown
	done := make(chan error, 1)

	go func() {
		done <- srv.Shutdown(ctx)
	}()

	// Wait for either shutdown complete or timeout
	select {
	case <-ctx.Done():
		if cfg.GracePeriod > 0 {
			time.Sleep(cfg.GracePeriod)
		}
		return ErrShutdownTimeout
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	if cfg.OnAllDrained != nil {
		cfg.OnAllDrained()
	}

	return nil
}

// ServeWithGracefulShutdown starts an HTTP server with graceful shutdown
func ServeWithGracefulShutdown(addr string, handler http.Handler, mgr *EnhancedManager) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Wrap handler to track connections
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mgr.connTracker.RegisterConnection()
		defer mgr.connTracker.UnregisterConnection()
		handler.ServeHTTP(w, r)
	})

	srv.Handler = wrappedHandler

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	// Wait for signals
	mgr.WaitForSignal(context.Background())

	return nil
}
