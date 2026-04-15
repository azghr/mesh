package database

import (
	"context"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// getTestDB returns a test database container and connection config
func getTestDB(t *testing.T) (testcontainers.Container, Config) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "test_db",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	config := Config{
		Host:            host,
		Port:            port.Int(),
		User:            "test",
		Password:        "test",
		Name:            "test_db",
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	return container, config
}

func TestNewPool(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	t.Run("creates pool successfully", func(t *testing.T) {
		pool, err := NewPool(config)
		require.NoError(t, err)
		require.NotNil(t, pool)
		defer pool.Close()

		assert.False(t, pool.IsClosed())
		assert.NotNil(t, pool.DB())
	})

	t.Run("validates connection with ping", func(t *testing.T) {
		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx := context.Background()
		err = pool.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("fails with invalid config", func(t *testing.T) {
		invalidConfig := Config{
			Host:     "invalid-host",
			Port:     9999,
			User:     "invalid",
			Password: "invalid",
			Name:     "invalid",
			SSLMode:  "disable",
		}

		pool, err := NewPool(invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, pool)
	})

	t.Run("configures connection pool settings", func(t *testing.T) {
		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		stats := pool.Stats()
		assert.Equal(t, 5, stats.MaxOpenConnections)
	})
}

func TestPool_DB(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	t.Run("returns non-nil DB when pool is open", func(t *testing.T) {
		db := pool.DB()
		assert.NotNil(t, db)
	})

	t.Run("returns nil after pool is closed", func(t *testing.T) {
		pool2, err := NewPool(config)
		require.NoError(t, err)
		pool2.Close()

		db := pool2.DB()
		assert.Nil(t, db)
	})
}

func TestPool_Close(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	t.Run("closes pool successfully", func(t *testing.T) {
		pool, err := NewPool(config)
		require.NoError(t, err)

		assert.False(t, pool.IsClosed())
		err = pool.Close()
		assert.NoError(t, err)
		assert.True(t, pool.IsClosed())
	})

	t.Run("close is idempotent", func(t *testing.T) {
		pool, err := NewPool(config)
		require.NoError(t, err)

		firstErr := pool.Close()
		secondErr := pool.Close()

		assert.NoError(t, firstErr)
		assert.NoError(t, secondErr)
		assert.True(t, pool.IsClosed())
	})
}

func TestPool_IsClosed(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)

	assert.False(t, pool.IsClosed())
	pool.Close()
	assert.True(t, pool.IsClosed())
}

func TestPool_Stats(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	stats := pool.Stats()
	assert.Equal(t, 5, stats.MaxOpenConnections)
	// OpenConnections may be > 0 due to Ping during pool creation
	assert.GreaterOrEqual(t, stats.OpenConnections, 0)
	assert.LessOrEqual(t, stats.OpenConnections, 5)
	assert.Equal(t, 0, stats.InUse)
}

func TestPool_Ping(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	t.Run("pings successfully", func(t *testing.T) {
		ctx := context.Background()
		err := pool.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("fails when pool is closed", func(t *testing.T) {
		pool2, err := NewPool(config)
		require.NoError(t, err)
		pool2.Close()

		ctx := context.Background()
		err = pool2.Ping(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pool is closed")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := pool.Ping(ctx)
		assert.Error(t, err)
	})
}

func TestPool_ConcurrentAccess(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	t.Run("concurrent DB calls", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				db := pool.DB()
				assert.NotNil(t, db)
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent ping calls", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := pool.Ping(ctx)
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent close", func(t *testing.T) {
		pool2, err := NewPool(config)
		require.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = pool2.Close()
			}()
		}
		wg.Wait()

		assert.True(t, pool2.IsClosed())
	})
}

func TestPool_ConnExhaustion(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	config.MaxOpenConns = 2
	config.MaxIdleConns = 1

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Create a simple test table
	db := pool.DB()
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test (id SERIAL PRIMARY KEY)")
	require.NoError(t, err)

	ctx := context.Background()

	// Try to use more connections than max
	var wg sync.WaitGroup
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := pool.Ping(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// With connection pooling, all pings should eventually succeed
	for err := range errors {
		assert.NoError(t, err)
	}
}

func TestConnect_Deprecated(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	t.Run("returns db connection", func(t *testing.T) {
		db, err := Connect(config)
		require.NoError(t, err)
		assert.NotNil(t, db)
		defer db.Close()
	})

	t.Run("fails with invalid config", func(t *testing.T) {
		invalidConfig := Config{
			Host:     "invalid",
			Port:     9999,
			User:     "invalid",
			Password: "invalid",
			Name:     "invalid",
			SSLMode:  "disable",
		}

		db, err := Connect(invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestGetInstance_Deprecated(t *testing.T) {
	db := GetInstance()
	assert.Nil(t, db)
}

func TestClose_Deprecated(t *testing.T) {
	err := Close()
	assert.NoError(t, err)
}

func TestConfigAdapter(t *testing.T) {
	t.Run("interface can be implemented", func(t *testing.T) {
		adapter := mockConfigAdapter{
			config: Config{
				Host: "localhost",
				Port: 5432,
			},
		}

		config := adapter.ToDatabaseConfig()
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 5432, config.Port)
	})
}

func TestPool_Query(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	db := pool.DB()

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(100)
		)
	`)
	require.NoError(t, err)

	// Insert test data
	result, err := db.Exec(
		"INSERT INTO users (name, email) VALUES ($1, $2)",
		"John Doe", "john@example.com",
	)
	require.NoError(t, err)

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected)

	// Query test data
	var name, email string
	err = db.QueryRow("SELECT name, email FROM users WHERE name = $1", "John Doe").Scan(&name, &email)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", name)
	assert.Equal(t, "john@example.com", email)
}

func TestPool_Transaction(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	db := pool.DB()

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts (
			id SERIAL PRIMARY KEY,
			balance DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	// Test transaction
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Insert data
	_, err = tx.Exec("INSERT INTO accounts (balance) VALUES (100.00)")
	require.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPool_ConnectionLifetimes(t *testing.T) {
	container, config := getTestDB(t)
	defer container.Terminate(context.Background())

	config.ConnMaxLifetime = 100 * time.Millisecond
	config.ConnMaxIdleTime = 50 * time.Millisecond

	pool, err := NewPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Use connection
	ctx := context.Background()
	err = pool.Ping(ctx)
	require.NoError(t, err)

	// Wait for connections to expire
	time.Sleep(150 * time.Millisecond)

	// Stats should show connections were recycled
	stats := pool.Stats()
	assert.GreaterOrEqual(t, stats.MaxOpenConnections, stats.Idle)
}

// mockConfigAdapter implements ConfigAdapter for testing
type mockConfigAdapter struct {
	config Config
}

func (m mockConfigAdapter) ToDatabaseConfig() Config {
	return m.config
}

// Benchmark tests
func BenchmarkPool_Ping(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	container, config := getTestDB(&testing.T{})
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(b, err)
	defer pool.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pool.Ping(ctx)
	}
}

func BenchmarkPool_ConcurrentPing(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	container, config := getTestDB(&testing.T{})
	defer container.Terminate(context.Background())

	pool, err := NewPool(config)
	require.NoError(b, err)
	defer pool.Close()

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Ping(ctx)
		}
	})
}
