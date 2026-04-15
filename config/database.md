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
```