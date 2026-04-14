// Package database provides PostgreSQL connection utilities.
//
// This package wraps database/sql with connection pool management
// and helper functions for queries and transactions.
//
// Quick example:
//
//	cfg := database.Config{
//	    Host: "localhost", Port: 5432, User: "user", Password: "pass",
//	    Name: "app", SSLMode: "disable",
//	}
//	pool, err := database.NewPool(cfg)
//	if err != nil {
//	    return err
//	}
//	defer pool.Close()
//
//	// Use pool.DB() with sqlc, pgx, or any database library
//	users := pool.DB().QueryContext(ctx, "SELECT * FROM users")
//
// # Features
//
// - Connection pool with configurable limits
// - Transaction helper functions
// - Query helpers for common patterns
// - ScanRow/ScanRows for generic result mapping
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
// Deprecated: This function always returns nil. Use dependency injection with NewPool instead.
// This exists for backward compatibility only.
func GetInstance() *sql.DB {
	return nil
}

// Close closes the database connection if it is open.
// Deprecated: This function always returns nil. Use Pool.Close() instead.
// This exists for backward compatibility only.
func Close() error {
	return nil
}

// ConfigAdapter provides a way to convert service-specific configs to the shared database config.
// Services can implement this interface to convert their config types.
type ConfigAdapter interface {
	ToDatabaseConfig() Config
}

// TxFunc is a function that operates within a transaction
type TxFunc func(tx *sql.Tx) error

// WithTransaction executes a function within a transaction.
// If the function returns an error, the transaction is rolled back.
// If successful, the transaction is committed.
func WithTransaction(ctx context.Context, db *sql.DB, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithTransactionOpts executes a function within a transaction with options.
func WithTransactionOpts(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// QueryRow executes a query that returns a single row.
func QueryRow(ctx context.Context, db *sql.DB, query string, args ...any) *sql.Row {
	return db.QueryRowContext(ctx, query, args...)
}

// Query executes a query that returns multiple rows.
func Query(ctx context.Context, db *sql.DB, query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows.
func Exec(ctx context.Context, db *sql.DB, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}

// TxQueryRow executes a query within a transaction.
func TxQueryRow(ctx context.Context, tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, query, args...)
}

// TxQuery executes a query within a transaction.
func TxQuery(ctx context.Context, tx *sql.Tx, query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(ctx, query, args...)
}

// TxExec executes a query within a transaction.
func TxExec(ctx context.Context, tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, query, args...)
}

// QueryResult is a helper for queries that return data
type QueryResult struct {
	Rows *sql.Rows
	Err  error
}

// TxQueryWithResult executes a query and returns the result
func TxQueryWithResult(ctx context.Context, tx *sql.Tx, query string, args ...any) *QueryResult {
	rows, err := tx.QueryContext(ctx, query, args...)
	return &QueryResult{Rows: rows, Err: err}
}

// ScanRow scans a single row into the destination variables.
func ScanRow[T any](row *sql.Row, dest *T) error {
	return row.Scan(dest)
}

// ScanRows scans multiple rows into a slice.
func ScanRows[T any](rows *sql.Rows, mapper func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()

	var results []T
	for rows.Next() {
		item, err := mapper(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// MustGet retrieves a value or panics if not found or error.
func MustGet[T any](row *sql.Row, dest *T) T {
	err := row.Scan(dest)
	if err != nil {
		panic(err)
	}
	return *dest
}

// OptionalGet retrieves a value or returns zero value if not found.
func OptionalGet[T any](row *sql.Row, dest *T) (T, bool, error) {
	err := row.Scan(dest)
	if err == sql.ErrNoRows {
		var zero T
		return zero, false, nil
	}
	if err != nil {
		var zero T
		return zero, false, err
	}
	return *dest, true, nil
}
