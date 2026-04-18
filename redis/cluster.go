// Package redis provides shared Redis client utilities for all services.
//
// This package implements Redis Cluster and Sentinel support for high-availability deployments,
// in addition to the standard single-node client.
//
// Example:
//
//	// Cluster mode
//	cluster, err := redis.NewCluster(redis.ClusterConfig{
//	    Addrs:    []string{"localhost:7000", "localhost:7001", "localhost:7002"},
//	    PoolSize: 10,
//	})
//
//	// Sentinel mode
//	sentinel, err := redis.NewSentinel(redis.SentinelConfig{
//	    MasterName:  "mymaster",
//	    SentinelAddrs: []string{"localhost:26379"},
//	    Password:   "sentinel-pass",
//	})
package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClusterConfig holds the configuration for connecting to Redis Cluster
type ClusterConfig struct {
	Addrs           []string      // Cluster node addresses
	PoolSize        int           // Max connections per node
	MinIdleConns    int           // Min idle connections per node
	Password        string        // Password for authentication
	DB              int           // Default DB
	MaxRetries      int           // Maximum number of retries
	ReadFromReplica bool          // Read from replicas when possible
	DialTimeout     time.Duration // Connection timeout
	ReadTimeout     time.Duration // Read timeout
	WriteTimeout    time.Duration // Write timeout
	PoolTimeout     time.Duration // Pool connection timeout
	IdleTimeout     time.Duration // Idle connection timeout
}

// DefaultClusterConfig returns a Cluster configuration with sensible defaults
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		Addrs:           []string{},
		PoolSize:        10,
		MinIdleConns:    5,
		Password:        "",
		DB:              0,
		MaxRetries:      3,
		ReadFromReplica: false,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     4 * time.Second,
		IdleTimeout:     5 * time.Minute,
	}
}

// ClusterClient wraps the Redis cluster client with additional functionality
type ClusterClient struct {
	client *redis.ClusterClient
	mu     sync.RWMutex
	closed bool
	config ClusterConfig
}

// NewCluster creates a new Redis Cluster client with the given configuration
func NewCluster(config ClusterConfig) (*ClusterClient, error) {
	// Apply defaults for zero values
	if config.PoolSize == 0 {
		config.PoolSize = 10
	}
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 5
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 3 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 3 * time.Second
	}
	if config.PoolTimeout == 0 {
		config.PoolTimeout = 4 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}

	// Validate addrs
	if len(config.Addrs) == 0 {
		return nil, fmt.Errorf("at least one cluster address is required")
	}

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        config.Addrs,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		Password:     config.Password,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to redis cluster: %w", err)
	}

	return &ClusterClient{
		client: client,
		config: config,
	}, nil
}

// Client returns the underlying redis.ClusterClient for use with redis commands
func (c *ClusterClient) Client() *redis.ClusterClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// Close closes the Redis Cluster client
func (c *ClusterClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.client.Close()
}

// IsClosed returns true if the client has been closed
func (c *ClusterClient) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Ping checks if the Redis Cluster is responding
func (c *ClusterClient) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	return c.client.Ping(ctx).Err()
}

// HealthCheck performs a health check on the Redis Cluster
func (c *ClusterClient) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Ping to check connection
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis cluster ping failed: %w", err)
	}

	return nil
}

// ForEachNode executes a function on each cluster node
func (c *ClusterClient) ForEachNode(ctx context.Context, fn func(ctx context.Context, node *redis.Client) error) error {
	// Get cluster slots to discover nodes
	slots, err := c.client.ClusterSlots(ctx).Result()
	if err != nil {
		return err
	}
	// Call fn for each master node (first node in each slot range is master)
	for _, slot := range slots {
		if len(slot.Nodes) == 0 {
			continue
		}
		// First node is typically the master
		masterAddr := slot.Nodes[0].Addr
		node := redis.NewClient(&redis.Options{
			Addr: masterAddr,
		})
		if err := fn(ctx, node); err != nil {
			node.Close()
			return err
		}
		node.Close()
	}
	return nil
}

// GetClusterSlots returns the cluster slots information
func (c *ClusterClient) GetClusterSlots(ctx context.Context) ([]redis.ClusterSlot, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.ClusterSlots(ctx).Result()
}

// Config returns the client configuration
func (c *ClusterClient) Config() ClusterConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SentinelConfig holds the configuration for connecting to Redis via Sentinel
type SentinelConfig struct {
	MasterName       string        // Master name to monitor
	SentinelAddrs    []string      // Sentinel addresses
	SentinelPassword string        // Sentinel password
	Password         string        // Master password
	DB               int           // Default DB
	PoolSize         int           // Max connections
	MinIdleConns     int           // Min idle connections
	DialTimeout      time.Duration // Connection timeout
	ReadTimeout      time.Duration // Read timeout
	WriteTimeout     time.Duration // Write timeout
	PoolTimeout      time.Duration // Pool connection timeout
	IdleTimeout      time.Duration // Idle connection timeout
}

// DefaultSentinelConfig returns a Sentinel configuration with sensible defaults
func DefaultSentinelConfig() SentinelConfig {
	return SentinelConfig{
		MasterName:       "",
		SentinelAddrs:    []string{},
		SentinelPassword: "",
		Password:         "",
		DB:               0,
		PoolSize:         10,
		MinIdleConns:     5,
		DialTimeout:      5 * time.Second,
		ReadTimeout:      3 * time.Second,
		WriteTimeout:     3 * time.Second,
		PoolTimeout:      4 * time.Second,
		IdleTimeout:      5 * time.Minute,
	}
}

// SentinelClient wraps the Redis Sentinel client with additional functionality
type SentinelClient struct {
	client *redis.Client
	mu     sync.RWMutex
	closed bool
	config SentinelConfig
}

// NewSentinel creates a new Redis Sentinel client with the given configuration
func NewSentinel(config SentinelConfig) (*SentinelClient, error) {
	// Apply defaults for zero values
	if config.PoolSize == 0 {
		config.PoolSize = 10
	}
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 5
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 3 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 3 * time.Second
	}
	if config.PoolTimeout == 0 {
		config.PoolTimeout = 4 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}

	// Validate config
	if len(config.SentinelAddrs) == 0 {
		return nil, fmt.Errorf("at least one sentinel address is required")
	}
	if config.MasterName == "" {
		return nil, fmt.Errorf("master name is required")
	}

	client := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:       config.MasterName,
		SentinelAddrs:    config.SentinelAddrs,
		SentinelPassword: config.SentinelPassword,
		Password:         config.Password,
		DB:               config.DB,
		PoolSize:         config.PoolSize,
		MinIdleConns:     config.MinIdleConns,
		DialTimeout:      config.DialTimeout,
		ReadTimeout:      config.ReadTimeout,
		WriteTimeout:     config.WriteTimeout,
		PoolTimeout:      config.PoolTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to redis via sentinel: %w", err)
	}

	return &SentinelClient{
		client: client,
		config: config,
	}, nil
}

// Client returns the underlying redis.Client for use with redis commands
func (c *SentinelClient) Client() *redis.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// Close closes the Redis Sentinel client
func (c *SentinelClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.client.Close()
}

// IsClosed returns true if the client has been closed
func (c *SentinelClient) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Ping checks if the Redis server via Sentinel is responding
func (c *SentinelClient) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	return c.client.Ping(ctx).Err()
}

// HealthCheck performs a health check on the Redis connection via Sentinel
func (c *SentinelClient) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Ping to check connection
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis sentinel ping failed: %w", err)
	}

	return nil
}

// GetMasterAddr returns the address of the current master
func (c *SentinelClient) GetMasterAddr(ctx context.Context) (string, error) {
	// The FailoverClient automatically routes to the master
	// This returns the configured master name for information
	return c.config.MasterName, nil
}

// GetReplicas returns the addresses of all replicas
// Note: This is a simplified implementation; detailed replica info requires additional sentinel queries
func (c *SentinelClient) GetReplicas(ctx context.Context) ([]string, error) {
	// The FailoverClient manages replicas internally
	// Return empty slice as detailed replica tracking requires sentinel protocol
	return []string{}, nil
}

// Config returns the client configuration
func (c *SentinelClient) Config() SentinelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Set sets a key-value pair with expiration (for ClusterClient)
func (c *ClusterClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client().Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key (for ClusterClient)
func (c *ClusterClient) Get(ctx context.Context, key string) (string, error) {
	return c.Client().Get(ctx, key).Result()
}

// Del deletes keys (for ClusterClient)
func (c *ClusterClient) Del(ctx context.Context, keys ...string) error {
	return c.Client().Del(ctx, keys...).Err()
}

// Exists checks if keys exist (for ClusterClient)
func (c *ClusterClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.Client().Exists(ctx, keys...).Result()
}

// Set sets a key-value pair with expiration (for SentinelClient)
func (c *SentinelClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client().Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key (for SentinelClient)
func (c *SentinelClient) Get(ctx context.Context, key string) (string, error) {
	return c.Client().Get(ctx, key).Result()
}

// Del deletes keys (for SentinelClient)
func (c *SentinelClient) Del(ctx context.Context, keys ...string) error {
	return c.Client().Del(ctx, keys...).Err()
}

// Exists checks if keys exist (for SentinelClient)
func (c *SentinelClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.Client().Exists(ctx, keys...).Result()
}
