// Package database provides database connection pool and migration utilities.
//
// This package includes connection pooling and migration management for
// PostgreSQL databases. It provides a robust migration system for versioned
// schema changes with rollback support.
//
// # Overview
//
// The database package provides:
//
//   - DB: PostgreSQL connection pool wrapper with health checking
//   - MigrationRunner: Version-controlled schema migration runner
//   - Migration: Individual migration definition with up/down SQL
//
// The migration system ensures atomic, ordered application of schema
// changes and tracks migration history in a metadata table.
//
// # Migration System
//
// Define migrations sorted by version:
//
//	migrations := []database.Migration{
//	    {
//	        Version: 2026041601,
//	        Name:    "create_users_table",
//	        Up: `CREATE TABLE IF NOT EXISTS users (
//	            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
//	            email VARCHAR(255) UNIQUE NOT NULL
//	        )`,
//	        Down: `DROP TABLE IF EXISTS users`,
//	    },
//	}
//
//	runner := database.NewMigrationRunner(db, migrations)
//	if err := runner.Run(ctx); err != nil {
//	    return err
//	}
//
// # Migration Runner Usage
//
// The MigrationRunner provides several operations:
//
//	// Run pending migrations
//	if err := runner.Run(ctx); err != nil {
//	    return err
//	}
//
//	// Rollback to a specific version
//	if err := runner.Rollback(ctx, 2026041501); err != nil {
//	    return err
//	}
//
//	// Check migration status
//	status, _ := runner.Status(ctx)
//
// # Best Practices
//
//   - Always define both Up and Down SQL
//   - Keep migrations small and focused on single changes
//   - Use Check for dependency validation before running
//   - Test rollbacks in development/staging
//   - Version format: YYYYMMDDNN (e.g., 2026041601)
//   - Never modify applied migrations; create new ones
//   - Use transactions for multi-step migrations
//
// # Metadata Table
//
// Migrations are tracked in a _schema_migrations table:
//
//	CREATE TABLE IF NOT EXISTS _schema_migrations (
//	    version BIGINT PRIMARY KEY,
//	    name VARCHAR(255) NOT NULL,
//	    applied_at TIMESTAMP DEFAULT NOW()
//	);
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Migration represents a database migration.
type Migration struct {
	Version int64  // Migration version (e.g., 2026041601)
	Name    string // Migration name
	Up      string // SQL to apply migration
	Down    string // SQL to rollback migration
	Check   string // Optional pre-flight check query
}

// NewMigration creates a new migration.
func NewMigration(version int64, name, up, down string) Migration {
	return Migration{
		Version: version,
		Name:    name,
		Up:      up,
		Down:    down,
	}
}

// MigrationRunner manages database schema migrations.
type MigrationRunner struct {
	db         *sql.DB
	table      string
	migrations []Migration
	mu         sync.RWMutex
}

// NewMigrationRunner creates a new migration runner.
// migrations should be sorted by Version in ascending order.
func NewMigrationRunner(db *sql.DB, migrations []Migration) *MigrationRunner {
	return &MigrationRunner{
		db:         db,
		table:      "schema_migrations",
		migrations: migrations,
	}
}

// Run applies pending migrations.
func (m *MigrationRunner) Run(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("ensuring migrations table: %w", err)
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current version: %w", err)
	}

	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			continue
		}

		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("applying migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// Rollback rolls back the specified number of migrations.
func (m *MigrationRunner) Rollback(ctx context.Context, steps int) error {
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current version: %w", err)
	}

	for i := 0; i < steps; i++ {
		migration := m.findMigration(currentVersion)
		if migration == nil {
			return fmt.Errorf("migration %d not found", currentVersion)
		}

		if err := m.rollbackMigration(ctx, *migration); err != nil {
			return fmt.Errorf("rolling back migration %d: %w", currentVersion, err)
		}

		currentVersion = migration.Version - 1
	}

	return nil
}

// ensureMigrationsTable creates the migrations tracking table.
func (m *MigrationRunner) ensureMigrationsTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, m.table)

	_, err := m.db.ExecContext(ctx, query)
	return err
}

// getCurrentVersion returns the current schema version.
func (m *MigrationRunner) getCurrentVersion(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(version), 0) FROM %s
	`, m.table)

	var version int64
	err := m.db.QueryRowContext(ctx, query).Scan(&version)
	return version, err
}

// applyMigration applies a single migration.
func (m *MigrationRunner) applyMigration(ctx context.Context, migration Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if migration.Check != "" {
		var exists int
		if err := tx.QueryRowContext(ctx, migration.Check).Scan(&exists); err != nil {
			return fmt.Errorf("migration check failed: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, migration.Up); err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (version, name, applied_at)
		VALUES ($1, $2, NOW())
	`, m.table)
	if _, err := tx.ExecContext(ctx, query, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}

	return nil
}

// rollbackMigration rolls back a single migration.
func (m *MigrationRunner) rollbackMigration(ctx context.Context, migration Migration) error {
	if migration.Down == "" {
		return nil // No rollback defined
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.Down); err != nil {
		return fmt.Errorf("executing rollback: %w", err)
	}

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE version = $1
	`, m.table)
	if _, err := tx.ExecContext(ctx, query, migration.Version); err != nil {
		return fmt.Errorf("deleting migration record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing rollback: %w", err)
	}

	return nil
}

// findMigration finds a migration by version.
func (m *MigrationRunner) findMigration(version int64) *Migration {
	for i := len(m.migrations) - 1; i >= 0; i-- {
		if m.migrations[i].Version == version {
			return &m.migrations[i]
		}
	}
	return nil
}

// Status returns the migration status.
func (m *MigrationRunner) Status(ctx context.Context) ([]MigrationStatus, error) {
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	var status []MigrationStatus
	for _, migration := range m.migrations {
		s := MigrationStatus{
			Version: migration.Version,
			Name:    migration.Name,
		}

		if migration.Version <= currentVersion {
			s.State = "applied"
			s.ApplyOrder = len(status) + 1
		} else {
			s.State = "pending"
		}

		status = append(status, s)
	}

	return status, nil
}

// MigrationStatus represents the status of a migration.
type MigrationStatus struct {
	Version    int64  `json:"version"`
	Name       string `json:"name"`
	State      string `json:"state"` // "applied" or "pending"
	ApplyOrder int    `json:"apply_order,omitempty"`
}

// Migrations returns the list of configured migrations.
func (m *MigrationRunner) Migrations() []Migration {
	return m.migrations
}

// SetTable sets the migrations table name.
func (m *MigrationRunner) SetTable(table string) {
	m.table = table
}

// GetPendingMigrations returns migrations that haven't been applied yet.
func GetPendingMigrations(db *sql.DB, allMigrations []Migration) ([]Migration, error) {
	runner := NewMigrationRunner(db, allMigrations)
	currentVersion, err := runner.getCurrentVersion(context.Background())
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range allMigrations {
		if m.Version > currentVersion {
			pending = append(pending, m)
		}
	}

	return pending, nil
}

// ValidateMigrations validates migration versions are sequential.
func ValidateMigrations(migrations []Migration) error {
	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			return fmt.Errorf("migration %d version must be greater than %d",
				migrations[i].Version, migrations[i-1].Version)
		}
	}
	return nil
}

// GenerateMigrationVersion generates a version based on date and sequence.
// Format: YYYYMMDDNN (e.g., 2026041601)
func GenerateMigrationVersion(date time.Time, sequence int) int64 {
	datePart := date.Format("20060102")
	var base int64
	fmt.Sscanf(datePart, "%d", &base)
	return base*100 + int64(sequence)
}

// ParseMigrationName parses a migration name into version and name.
func ParseMigrationName(name string) (int64, string, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, name, nil
	}

	var version int64
	_, err := fmt.Sscanf(parts[0], "%d", &version)
	if err != nil {
		return 0, name, nil
	}

	return version, parts[1], nil
}
