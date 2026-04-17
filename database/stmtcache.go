// Package database provides prepared statement caching for database queries.
//
// This package provides prepared statement caching to improve query performance
// by avoiding repeated query parsing and planning overhead.
//
// # Overview
//
// Prepared statement caching reduces database overhead by caching compiled SQL statements.
// This is especially beneficial for frequently executed queries.
//
// # Basic Usage
//
// Create a statement cache:
//
//	cache := database.NewStmtCache(db, database.StmtCacheConfig{
//	    MaxStatements: 100,
//	    TTL:           time.Hour,
//	})
//
// Use the cache for queries:
//
//	rows, err := cache.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
//
// # Benefits
//
//   - Reduces database CPU usage for query parsing
//   - Improves response times for repeated queries
//   - Automatic cleanup of stale statements
//   - Hit/miss statistics for monitoring
//
// # Best Practices
//
//   - Use for read-heavy workloads
//   - Set appropriate MaxStatements based on memory
//   - Monitor hit rates - high miss rate may need larger cache
//   - Use different caches for different query patterns
package database

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// StmtCache provides prepared statement caching.
type StmtCache struct {
	db    *sql.DB
	cache map[string]*sql.Stmt
	mu    sync.RWMutex
	stats StmtCacheStats
	ttl   time.Duration
}

// StmtCacheStats tracks prepared statement cache statistics.
type StmtCacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

// StmtCacheConfig configures the prepared statement cache.
type StmtCacheConfig struct {
	MaxStatements int           // Maximum cached statements (default: 100)
	TTL           time.Duration // Statement time-to-live (default: 1 hour)
}

// DefaultStmtCacheConfig returns default configuration.
func DefaultStmtCacheConfig() StmtCacheConfig {
	return StmtCacheConfig{
		MaxStatements: 100,
		TTL:           time.Hour,
	}
}

// NewStmtCache creates a new prepared statement cache.
func NewStmtCache(db *sql.DB, cfg StmtCacheConfig) *StmtCache {
	if cfg.MaxStatements == 0 {
		cfg.MaxStatements = DefaultStmtCacheConfig().MaxStatements
	}
	if cfg.TTL == 0 {
		cfg.TTL = DefaultStmtCacheConfig().TTL
	}

	cache := &StmtCache{
		db:    db,
		cache: make(map[string]*sql.Stmt),
		ttl:   cfg.TTL,
	}

	go cache.cleanupLoop()

	return cache
}

// Prepare returns a cached prepared statement, preparing if not cached.
func (c *StmtCache) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	c.mu.RLock()
	stmt, ok := c.cache[query]
	c.mu.RUnlock()

	if ok {
		c.stats.Hits++
		return stmt, nil
	}

	c.stats.Misses++

	stmt, err := c.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	// Check if another goroutine already added it
	if existing, ok := c.cache[query]; ok {
		c.mu.Unlock()
		c.stats.Hits++
		return existing, nil
	}

	// Evict if at capacity
	if len(c.cache) >= 100 {
		c.evictOne()
	}

	c.cache[query] = stmt
	c.mu.Unlock()

	return stmt, nil
}

// Exec executes a prepared statement with arguments.
func (c *StmtCache) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	stmt, err := c.Prepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.ExecContext(ctx, args...)
}

// Query executes a prepared statement with arguments.
func (c *StmtCache) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := c.Prepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryContext(ctx, args...)
}

// QueryRow executes a prepared statement with arguments, returning one row.
func (c *StmtCache) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	stmt, err := c.Prepare(ctx, query)
	if err != nil {
		return &sql.Row{}
	}
	return stmt.QueryRowContext(ctx, args...)
}

// evictOne removes one statement from the cache.
func (c *StmtCache) evictOne() {
	c.stats.Evictions++
	// Remove oldest entry (simplified - in production, use LRU)
	for query, stmt := range c.cache {
		stmt.Close()
		delete(c.cache, query)
		return
	}
}

// cleanupLoop periodically cleans up stale statements.
func (c *StmtCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		for query, stmt := range c.cache {
			stmt.Close()
			delete(c.cache, query)
		}
		c.mu.Unlock()
	}
}

// Stats returns cache statistics.
func (c *StmtCache) Stats() (hits, misses, evictions int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats.Hits, c.stats.Misses, c.stats.Evictions
}

// Clear removes all cached statements.
func (c *StmtCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, stmt := range c.cache {
		stmt.Close()
	}
	c.cache = make(map[string]*sql.Stmt)
}

// Close closes all cached statements.
func (c *StmtCache) Close() {
	c.Clear()
}
