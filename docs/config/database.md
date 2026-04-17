# database

PostgreSQL connection pool and query utilities.

## What It Does

Wraps the standard `database/sql` package with connection pool management, transaction helpers, and generic query helpers that work with any code generation tool (sqlc, go-pg, etc.).

## Usage

### Basic Connection

```go
pool, err := database.NewPool(database.Config{
    Host: "localhost",
    Port: 5432,
    User: "user",
    Password: "pass",
    Name: "mydb",
    MaxOpenConns: 25,
    MaxIdleConns: 5,
    ConnMaxLifetime: time.Hour,
})
if err != nil {
    return err
}
defer pool.Close()

// Access the underlying *sql.DB
db := pool.DB()
```

### Transactions

```go
err := database.WithTransaction(ctx, pool.DB(), func(tx *sql.Tx) error {
    // Do multiple queries in a transaction
    _, err := tx.ExecContext(ctx, "INSERT INTO ...", ...)
    if err != nil {
        return err // Transaction will rollback
    }
    return tx.ExecContext(ctx, "UPDATE ...", ...)
})
// If err != nil, transaction was rolled back
// If nil, transaction was committed
```

### Query Helpers

```go
// Scan a single row
var user User
row := database.QueryRow(ctx, db, "SELECT * FROM users WHERE id = $1", id)
err := row.Scan(&user.ID, &user.Name, &user.Email)

// Generic row scanning (with sqlc, etc.)
var user User
err := database.ScanRow(row, &user)

// Scan multiple rows
rows, err := database.Query(ctx, db, "SELECT * FROM users")
defer rows.Close()
users, err := database.ScanRows(rows, func(r *sql.Rows) (User, error) {
    var u User
    return u, r.Scan(&u.ID, &u.Name)
})

// Optional - returns zero value if not found
user, found, err := database.OptionalGet[row](row, &user)
```

## Pool Management

```go
// Check connection
err := pool.Ping(ctx)

// Get pool stats
stats := pool.Stats()
log.Printf("open: %d, inuse: %d, idle: %d", 
    stats.OpenConnections, 
    stats.InUse, 
    stats.Idle)

// Graceful close
err := pool.Close()
```

## Configuration

```go
type Config struct {
    Host            string
    Port            int
    User            string
    Password        string
    Name            string
    SSLMode         string        // "disable", "require", etc.
    MaxOpenConns    int           // Max open connections (default: 25)
    MaxIdleConns    int           // Max idle connections (default: 5)
    ConnMaxLifetime time.Duration // How long connections live (default: 1h)
    ConnMaxIdleTime time.Duration // How long idle connections live
}

## Database Migrations

Manages database schema versioning with rollback support.

### MigrationRunner

```go
import "github.com/azghr/mesh/database"

// Define migrations (must be sorted by Version ascending)
migrations := []database.Migration{
    {
        Version: 2026041601,
        Name:    "create_users_table",
        Up: `CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            email VARCHAR(255) UNIQUE NOT NULL,
            name VARCHAR(255) NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        )`,
        Down: `DROP TABLE IF EXISTS users`,
    },
    {
        Version: 2026041602,
        Name:    "create_sessions_table",
        Up: `CREATE TABLE IF NOT EXISTS sessions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        )`,
        Down: `DROP TABLE IF EXISTS sessions`,
        Check: `SELECT 1 FROM users LIMIT 1`, // Pre-flight check
    },
}

runner := database.NewMigrationRunner(db, migrations)

// Apply pending migrations
if err := runner.Run(ctx); err != nil {
    return err
}

// Rollback last migration
if err := runner.Rollback(ctx, 1); err != nil {
    return err
}
```

### Configuration

```go
type Migration struct {
    Version int64  // Version number (e.g., 2026041601)
    Name    string // Migration name
    Up     string // SQL to apply
    Down   string // SQL to rollback
    Check  string // Optional pre-flight check
}
```

### Status

```go
status, err := runner.Status(ctx)
// Returns: [{Version: 2026041601, Name: "create_users_table", State: "applied"}, {Version: 2026041602, Name: "create_sessions_table", State: "pending"}]
```

### Helpers

```go
// Get pending migrations
pending, err := database.GetPendingMigrations(db, migrations)

// Validate migration order
err := database.ValidateMigrations(migrations)

// Generate version from date
version := database.GenerateMigrationVersion(time.Now(), 1) // e.g., 2026041601
```

### Best Practices

1. Keep migrations small and reversible
2. Use descriptive names: `2026041601_create_users_table`
3. Always test rollbacks in development
4. Use `Check` for dependencies (e.g., check parent table exists)
5. Run migrations on startup with proper locking

## Prepared Statement Cache

Reduces database overhead by caching compiled SQL statements.

### Basic Usage

```go
cache := database.NewStmtCache(db, database.StmtCacheConfig{
    MaxStatements: 100,
    TTL:         time.Hour,
})

// Use for queries
rows, err := cache.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
result, err := cache.Exec(ctx, "INSERT INTO ...", args...)
var user User
row := cache.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id)
row.Scan(&user)
```

### Configuration

```go
type StmtCacheConfig struct {
    MaxStatements int           // Max cached statements (default: 100)
    TTL           time.Duration // Statement time-to-live (default: 1 hour)
}
```

### Statistics

```go
hits, misses, evictions := cache.Stats()
// Monitor hit rate: hits / (hits + misses)
// Good hit rate is > 80%
```

### Best Practices

- Use for read-heavy workloads
- Monitor hit rates - adjust MaxStatements if low
- Clear cache periodically for schema changes
- Use separate caches for different query patterns
```