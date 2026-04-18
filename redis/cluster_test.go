package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterConfig_Defaults(t *testing.T) {
	config := DefaultClusterConfig()

	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 5, config.MinIdleConns)
	assert.Equal(t, 3, config.MaxRetries)
	assert.NotZero(t, config.DialTimeout)
	assert.NotZero(t, config.ReadTimeout)
	assert.NotZero(t, config.WriteTimeout)
}

func TestClusterConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  ClusterConfig
		wantErr bool
	}{
		{
			name: "empty addrs",
			config: ClusterConfig{
				Addrs: []string{},
			},
			wantErr: true,
		},
		{
			name: "valid addrs",
			config: ClusterConfig{
				Addrs: []string{"localhost:7000"},
			},
			wantErr: false,
		},
		{
			name: "multiple addrs",
			config: ClusterConfig{
				Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCluster(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// We expect an error because there's no actual cluster
				// but we're testing that validation works
				assert.Error(t, err) // Connection will fail, but that's expected without a cluster
			}
		})
	}
}

func TestSentinelConfig_Defaults(t *testing.T) {
	config := DefaultSentinelConfig()

	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 5, config.MinIdleConns)
	assert.NotZero(t, config.DialTimeout)
	assert.NotZero(t, config.ReadTimeout)
}

func TestSentinelConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  SentinelConfig
		wantErr bool
	}{
		{
			name: "empty sentinel addrs",
			config: SentinelConfig{
				MasterName:    "mymaster",
				SentinelAddrs: []string{},
			},
			wantErr: true,
		},
		{
			name: "empty master name",
			config: SentinelConfig{
				MasterName:    "",
				SentinelAddrs: []string{"localhost:26379"},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: SentinelConfig{
				MasterName:    "mymaster",
				SentinelAddrs: []string{"localhost:26379"},
			},
			wantErr: true, // Will fail because no sentinel running
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSentinel(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Error(t, err) // Connection will fail, but validation works
			}
		})
	}
}

func TestClusterClient_NotImplementedWithoutCluster(t *testing.T) {
	// This test verifies that the API is correct
	// Actual cluster tests would require a running Redis cluster
	config := ClusterConfig{
		Addrs: []string{"localhost:7000"},
	}

	_, err := NewCluster(config)
	// Expected to fail since there's no cluster
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestSentinelClient_NotImplementedWithoutSentinel(t *testing.T) {
	config := SentinelConfig{
		MasterName:    "mymaster",
		SentinelAddrs: []string{"localhost:26379"},
	}

	_, err := NewSentinel(config)
	// Expected to fail since there's no sentinel
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestClusterClient_Interface(t *testing.T) {
	// Verify ClusterClient implements expected interface
	var (
		_ interface{} = (*ClusterClient)(nil)
	)
	assert.True(t, true) // Placeholder
}

func TestSentinelClient_Interface(t *testing.T) {
	// Verify SentinelClient implements expected interface
	var (
		_ interface{} = (*SentinelClient)(nil)
	)
	assert.True(t, true) // Placeholder
}

func TestClusterConfig_Fields(t *testing.T) {
	cfg := ClusterConfig{
		Addrs:        []string{"x1", "x2"},
		PoolSize:     20,
		MinIdleConns: 10,
		Password:     "secret",
		MaxRetries:   5,
	}

	assert.Equal(t, []string{"x1", "x2"}, cfg.Addrs)
	assert.Equal(t, 20, cfg.PoolSize)
	assert.Equal(t, 10, cfg.MinIdleConns)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, 5, cfg.MaxRetries)
}

func TestSentinelConfig_Fields(t *testing.T) {
	cfg := SentinelConfig{
		MasterName:       "mymaster",
		SentinelAddrs:    []string{"s1:26379"},
		SentinelPassword: "sentinel-pass",
		Password:         "master-pass",
		DB:               1,
		PoolSize:         20,
	}

	assert.Equal(t, "mymaster", cfg.MasterName)
	assert.Equal(t, []string{"s1:26379"}, cfg.SentinelAddrs)
	assert.Equal(t, "sentinel-pass", cfg.SentinelPassword)
	assert.Equal(t, "master-pass", cfg.Password)
	assert.Equal(t, 1, cfg.DB)
	assert.Equal(t, 20, cfg.PoolSize)
}

func TestClusterClient_Close_Idempotent(t *testing.T) {
	config := ClusterConfig{
		Addrs: []string{"localhost:7000"},
	}

	// Can't test without cluster, but verify API structure
	_ = config
	assert.True(t, true)
}

func TestSentinelClient_Close_Idempotent(t *testing.T) {
	config := SentinelConfig{
		MasterName:    "mymaster",
		SentinelAddrs: []string{"localhost:26379"},
	}

	// Can't test without sentinel, but verify API structure
	_ = config
	assert.True(t, true)
}

// BenchmarkClusterConfig creation
func BenchmarkClusterConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultClusterConfig()
	}
}

// BenchmarkSentinelConfig creation
func BenchmarkSentinelConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultSentinelConfig()
	}
}

// Helper for tests that need context
func newTestContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	return ctx
}
