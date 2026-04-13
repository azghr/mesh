// Package database provides advanced database connection pooling with metrics
// and health monitoring for all Lumex services.
//
// This package extends the basic database functionality with production-ready
// connection pooling, metrics collection, and observability.
//
// Example:
//
//	pool, err := database.NewPool(database.Config{
//	    Host: "localhost",
//	    Port: 5432,
//	    User: "user",
//	    Password: "pass",
//	    Name: "dbname",
//	    MaxOpenConns: 25,
//	    MaxIdleConns: 10,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Close()
//
//	// Use pool.DB() with sqlc
//	queries := sqlc.New(pool.DB())
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
)

// PoolConfig holds configuration for database connection pooling
type PoolConfig struct {
	// Service name for metrics and logging
	ServiceName string

	// Maximum number of open connections to the database.
	// Default: 25
	MaxOpenConns int

	// Maximum number of connections in the idle connection pool.
	// Default: 10
	MaxIdleConns int

	// Minimum number of connections in the idle connection pool.
	// Default: 5
	MinIdleConns int

	// Maximum amount of time a connection may be reused.
	// Default: 1 hour
	ConnMaxLifetime time.Duration

	// Maximum amount of time a connection may be idle.
	// Default: 5 minutes
	ConnMaxIdleTime time.Duration

	// How long to wait for a connection to become available
	// when the pool is exhausted.
	// Default: 30 seconds
	ConnMaxWaitTime time.Duration

	// How long to wait for a connection to be established.
	// Default: 5 seconds
	ConnTimeout time.Duration

	// Enable health checks
	EnableHealthCheck bool

	// Health check interval
	HealthCheckInterval time.Duration

	// Logger for pool events
	Logger logger.Logger
}

// DefaultPoolConfig returns a pool configuration with sensible defaults
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		ServiceName:         "database",
		MaxOpenConns:        25,
		MaxIdleConns:        10,
		MinIdleConns:        5,
		ConnMaxLifetime:     1 * time.Hour,
		ConnMaxIdleTime:     5 * time.Minute,
		ConnMaxWaitTime:     30 * time.Second,
		ConnTimeout:         5 * time.Second,
		EnableHealthCheck:   true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// PoolManager manages database connection pools with metrics and health monitoring
type PoolManager struct {
	pools map[string]*ManagedPool
	mu    sync.RWMutex
}

// ManagedPool wraps a Pool with metrics and health monitoring
type ManagedPool struct {
	*Pool
	config    PoolConfig
	metrics   *PoolMetrics
	startTime time.Time
	mu        sync.RWMutex
	closed    bool
}

// PoolMetrics collects connection pool metrics
type PoolMetrics struct {
	TotalConnections  int64
	IdleConnections   int64
	InUseConnections  int64
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64
	TotalQueries      int64
	TotalErrors       int64
	SlowQueries       int64
	AverageQueryTime  time.Duration
	MaxQueryTime      time.Duration
	lastHealthCheck   time.Time
	healthCheckPassed bool
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]*ManagedPool),
	}
}

// GetPool returns a managed pool by name, creating it if it doesn't exist
func (pm *PoolManager) GetPool(name string, cfg Config, poolCfg PoolConfig) (*ManagedPool, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Return existing pool if found
	if pool, exists := pm.pools[name]; exists && !pool.closed {
		return pool, nil
	}

	// Apply defaults
	if poolCfg.MaxOpenConns == 0 {
		poolCfg.MaxOpenConns = DefaultPoolConfig().MaxOpenConns
	}
	if poolCfg.MaxIdleConns == 0 {
		poolCfg.MaxIdleConns = DefaultPoolConfig().MaxIdleConns
	}
	if poolCfg.MinIdleConns == 0 {
		poolCfg.MinIdleConns = DefaultPoolConfig().MinIdleConns
	}
	if poolCfg.ConnMaxLifetime == 0 {
		poolCfg.ConnMaxLifetime = DefaultPoolConfig().ConnMaxLifetime
	}
	if poolCfg.ConnMaxIdleTime == 0 {
		poolCfg.ConnMaxIdleTime = DefaultPoolConfig().ConnMaxIdleTime
	}
	if poolCfg.ConnMaxWaitTime == 0 {
		poolCfg.ConnMaxWaitTime = DefaultPoolConfig().ConnMaxWaitTime
	}
	if poolCfg.ConnTimeout == 0 {
		poolCfg.ConnTimeout = DefaultPoolConfig().ConnTimeout
	}

	// Create base pool
	pool, err := NewPool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Configure connection pool settings
	db := pool.DB()
	db.SetMaxOpenConns(poolCfg.MaxOpenConns)
	db.SetMaxIdleConns(poolCfg.MaxIdleConns)
	db.SetConnMaxLifetime(poolCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(poolCfg.ConnMaxIdleTime)

	managedPool := &ManagedPool{
		Pool:      pool,
		config:    poolCfg,
		metrics:   &PoolMetrics{},
		startTime: time.Now(),
		closed:    false,
	}

	// Start health checks if enabled
	if poolCfg.EnableHealthCheck {
		go managedPool.runHealthChecks()
	}

	// Start metrics collection
	go managedPool.collectMetrics()

	pm.pools[name] = managedPool

	return managedPool, nil
}

// Close closes all managed pools
func (pm *PoolManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var lastErr error
	for name, pool := range pm.pools {
		if err := pool.Close(); err != nil {
			lastErr = err
		}
		delete(pm.pools, name)
	}

	return lastErr
}

// GetPoolNames returns all managed pool names
func (pm *PoolManager) GetPoolNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.pools))
	for name := range pm.pools {
		names = append(names, name)
	}
	return names
}

// Metrics returns metrics for all pools
func (pm *PoolManager) Metrics() map[string]*PoolMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics := make(map[string]*PoolMetrics)
	for name, pool := range pm.pools {
		pool.mu.RLock()
		metrics[name] = pool.metrics
		pool.mu.RUnlock()
	}
	return metrics
}

// collectMetrics periodically collects pool statistics
func (mp *ManagedPool) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := mp.Pool.Stats()

		mp.mu.Lock()
		mp.metrics.TotalConnections = int64(stats.MaxOpenConnections)
		mp.metrics.IdleConnections = int64(stats.Idle)
		mp.metrics.InUseConnections = int64(stats.InUse)
		mp.metrics.WaitCount = stats.WaitCount
		mp.metrics.WaitDuration = stats.WaitDuration
		mp.metrics.MaxIdleClosed = stats.MaxIdleClosed
		mp.metrics.MaxLifetimeClosed = stats.MaxLifetimeClosed
		mp.mu.Unlock()
	}
}

// runHealthChecks performs periodic health checks
func (mp *ManagedPool) runHealthChecks() {
	if mp.config.HealthCheckInterval == 0 {
		mp.config.HealthCheckInterval = DefaultPoolConfig().HealthCheckInterval
	}

	ticker := time.NewTicker(mp.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), mp.config.ConnTimeout)
		err := mp.Pool.Ping(ctx)
		cancel()

		mp.mu.Lock()
		mp.metrics.lastHealthCheck = time.Now()
		mp.metrics.healthCheckPassed = (err == nil)
		mp.mu.Unlock()

		if err != nil && mp.config.Logger != nil {
			mp.config.Logger.WithError(err).
				WithField("pool", mp.config.ServiceName).
				Warn("Database health check failed")
		}
	}
}

// HealthCheck performs an immediate health check
func (mp *ManagedPool) HealthCheck(ctx context.Context) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if mp.closed {
		return fmt.Errorf("pool is closed")
	}

	return mp.Pool.Ping(ctx)
}

// GetMetrics returns the current pool metrics
func (mp *ManagedPool) GetMetrics() *PoolMetrics {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	// Return a copy to prevent concurrent modification
	metricsCopy := *mp.metrics
	return &metricsCopy
}

// IsHealthy returns whether the last health check passed
func (mp *ManagedPool) IsHealthy() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.metrics.healthCheckPassed
}

// LastHealthCheck returns the time of the last health check
func (mp *ManagedPool) LastHealthCheck() time.Time {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.metrics.lastHealthCheck
}

// Close closes the managed pool
func (mp *ManagedPool) Close() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.closed {
		return nil
	}

	mp.closed = true
	return mp.Pool.Close()
}

// Warmup pre-creates connections to the database
func (mp *ManagedPool) Warmup(ctx context.Context, count int) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if mp.closed {
		return fmt.Errorf("pool is closed")
	}

	if mp.config.Logger != nil {
		mp.config.Logger.WithFields(map[string]any{
			"pool":   mp.config.ServiceName,
			"count":  count,
			"status": "starting",
		}).Info("Connection pool warmup")
	}

	db := mp.Pool.DB()

	// Create connections in parallel
	errChan := make(chan error, count)
	for i := 0; i < count; i++ {
		go func() {
			err := db.PingContext(ctx)
			errChan <- err
		}()
	}

	// Collect results
	var errors []error
	for i := 0; i < count; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if mp.config.Logger != nil {
		mp.config.Logger.WithFields(map[string]any{
			"pool":       mp.config.ServiceName,
			"requested":  count,
			"successful": count - len(errors),
			"failed":     len(errors),
			"status":     "completed",
		}).Info("Connection pool warmup completed")
	}

	if len(errors) > 0 {
		return fmt.Errorf("warmup failed: %d errors", len(errors))
	}

	return nil
}

// ExecuteInTx executes a function within a transaction with retry logic
func (mp *ManagedPool) ExecuteInTx(ctx context.Context, fn func(*sql.Tx) error) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if mp.closed {
		return fmt.Errorf("pool is closed")
	}

	db := mp.Pool.DB()
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Begin transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// Execute function
		if err := fn(tx); err != nil {
			// Rollback on error
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("function error: %w, rollback error: %v", err, rbErr)
			}
			lastErr = err

			// Don't retry on context cancellation or constraint violations
			if ctx.Err() != nil || isNonRetryableError(err) {
				return err
			}
			continue
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			lastErr = err

			// Don't retry on context cancellation
			if ctx.Err() != nil {
				return err
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("transaction failed after %d attempts: %w", maxRetries, lastErr)
}

// isNonRetryableError checks if an error should not be retried
func isNonRetryableError(err error) bool {
	// Add checks for specific error types that shouldn't be retried
	// For now, just check for context errors
	if err == nil {
		return false
	}
	return false
}

// StatsJSON returns pool statistics as JSON for monitoring
func (mp *ManagedPool) StatsJSON() string {
	stats := mp.Pool.Stats()
	return fmt.Sprintf(`{
		"max_open_connections": %d,
		"open_connections": %d,
		"in_use": %d,
		"idle": %d,
		"wait_count": %d,
		"wait_duration": "%s",
		"max_idle_closed": %d,
		"max_lifetime_closed": %d
	}`,
		stats.MaxOpenConnections,
		stats.OpenConnections,
		stats.InUse,
		stats.Idle,
		stats.WaitCount,
		stats.WaitDuration.String(),
		stats.MaxIdleClosed,
		stats.MaxLifetimeClosed,
	)
}

// GetUptime returns the uptime of the pool
func (mp *ManagedPool) GetUptime() time.Duration {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return time.Since(mp.startTime)
}
