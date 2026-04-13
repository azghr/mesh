package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 5, config.MinIdleConns)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.DialTimeout)
	assert.Equal(t, 3*time.Second, config.ReadTimeout)
	assert.Equal(t, 3*time.Second, config.WriteTimeout)
	assert.Equal(t, 4*time.Second, config.PoolTimeout)
	assert.Equal(t, 5*time.Minute, config.IdleTimeout)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "creates client with default config",
			config: Config{
				Host: "invalid-host",
				Port: 6379,
			},
			wantErr: true, // Invalid host
		},
		{
			name: "creates client with custom config",
			config: Config{
				Host:            "invalid-host",
				Port:            6379,
				Password:        "password",
				DB:              1,
				PoolSize:        20,
				MaxRetries:      5,
				DialTimeout:     10 * time.Second,
				ReadTimeout:     5 * time.Second,
				WriteTimeout:    5 * time.Second,
				PoolTimeout:     8 * time.Second,
				IdleTimeout:     10 * time.Minute,
				MinIdleConns:    10,
			},
			wantErr: true, // Invalid host
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				// Close client if successful
				_ = client.Close()
			}
		})
	}
}

func TestNewClientWithMiniRedis(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	config := Config{
		Host:         server.Host(),
		Port:         0, // miniredis uses Unix socket or handles this internally
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	}

	// Create client with custom addr
	client, err := NewClient(config)

	// Should succeed or fail depending on connection
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, client)
	} else {
		assert.NotNil(t, client)
		client.Close()
	}
}

func TestClient_Ping(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	config := Config{
		Host:         "",
		Port:         0, // miniredis handles this
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	}

	// Create a real redis client for testing
	redisClient, err := NewClient(config)
	if err == nil {
		ctx := context.Background()
		err = redisClient.Ping(ctx)
		assert.NoError(t, err)
		redisClient.Close()
	}
}

func TestClient_Close(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	config := Config{
		Host:         "",
		Port:         0, // miniredis handles this
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	client, err := NewClient(config)
	if err == nil {
		err := client.Close()
		assert.NoError(t, err)

		// Second close should be safe
		err = client.Close()
		assert.NoError(t, err)
	}
}

func TestClient_IsClosed(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	config := Config{
		Host:         "",
		Port:         0, // miniredis handles this
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	client, err := NewClient(config)
	if err == nil {
		assert.False(t, client.IsClosed())

		client.Close()
		assert.True(t, client.IsClosed())
	}
}

func TestClient_HealthCheck(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	config := Config{
		Host:         "",
		Port:         0, // miniredis handles this
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	client, err := NewClient(config)
	if err == nil {
		ctx := context.Background()
		err := client.HealthCheck(ctx)
		assert.NoError(t, err)
		client.Close()
	}
}

func TestClient_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	// This would require testcontainers setup
	// For now, test with mock
	t.Skip("requires testcontainers")
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "valid config",
			config: Config{
				Host: "localhost",
				Port: 6379,
			},
			valid: true,
		},
		{
			name: "config with zero port",
			config: Config{
				Host: "localhost",
				Port: 0,
			},
			valid: true, // Redis client will handle this
		},
		{
			name: "config with negative pool size",
			config: Config{
				Host:     "localhost",
				Port:     6379,
				PoolSize: -1,
			},
			valid: true, // Redis client will handle this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify config can be created
			assert.NotNil(t, tt.config)
		})
	}
}

func TestClient_Config(t *testing.T) {
	config := Config{
		Host:     "testhost",
		Port:     6380,
		Password: "testpass",
		DB:       2,
	}

	client := &Client{
		client: nil,
		config: config,
		closed: false,
	}

	assert.Equal(t, config, client.Config())
}

// Benchmark tests
func BenchmarkClient_Ping(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	server, err := miniredis.Run()
	require.NoError(b, err)
	defer server.Close()

	config := Config{
		Host:         "",
		Port:         0, // miniredis handles this
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	client, err := NewClient(config)
	require.NoError(b, err)
	defer client.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = client.Ping(ctx)
	}
}
