// Package database provides shared database connection utilities for all Lumex services.
//
// This package implements a connection pool pattern for database connections with proper
// connection pooling, configuration, and lifecycle management using dependency injection.
//
// Example:
//
//	config := Config{
//	    Host:            "localhost",
//	    Port:            5432,
//	    User:            "user",
//	    Password:        "pass",
//	    Name:            "dbname",
//	    SSLMode:         "disable",
//	    MaxOpenConns:    25,
//	    MaxIdleConns:    5,
//	    ConnMaxLifetime: time.Hour,
//	    ConnMaxIdleTime: 30 * time.Minute,
//	}
//
//	pool, err := database.NewPool(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Close()
//
//	// Use pool.DB() with sqlc or other libraries
//	queries := sqlc.New(pool.DB())
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Config holds the configuration for connecting to a PostgreSQL database.
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Pool manages a database connection pool with proper lifecycle management.
// Pool is safe for concurrent use and provides clean shutdown semantics.
type Pool struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
}

// NewPool creates a new database connection pool with the given configuration.
// The pool is automatically configured with the provided connection settings,
// and the connection is verified by pinging the database.
//
// Returns an error if the connection cannot be established or the ping fails.
func NewPool(cfg Config) (*Pool, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool settings from config
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	// Verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{db: db}, nil
}

// DB returns the underlying *sql.DB for use with sqlc and other libraries.
// This method is safe to call concurrently. Returns nil if the pool is closed.
func (p *Pool) DB() *sql.DB {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil
	}
	return p.db
}

// Close closes the database connection pool.
// It is safe to call Close multiple times; subsequent calls are no-ops.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	return p.db.Close()
}

// IsClosed returns true if the pool has been closed.
func (p *Pool) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// Stats returns connection pool statistics for monitoring.
func (p *Pool) Stats() sql.DBStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.db.Stats()
}

// Ping verifies a connection to the database is still alive.
func (p *Pool) Ping(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	return p.db.PingContext(ctx)
}

// Connect creates and initializes a singleton database connection using the provided configuration.
// Deprecated: Use NewPool instead. This function is maintained for backward compatibility.
func Connect(cfg Config) (*sql.DB, error) {
	pool, err := NewPool(cfg)
	if err != nil {
		return nil, err
	}
	return pool.DB(), nil
}

// GetInstance returns the current database instance.
// Deprecated: This is a no-op. Use NewPool to create a pool.
func GetInstance() *sql.DB {
	return nil
}

// Close closes the database connection if it is open.
// Deprecated: This is a no-op. Use Pool.Close() instead.
func Close() error {
	return nil
}

// ConfigAdapter provides a way to convert service-specific configs to the shared database config.
// Services can implement this interface to convert their config types.
type ConfigAdapter interface {
	ToDatabaseConfig() Config
}

