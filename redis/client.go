// Package redis provides shared Redis client utilities for all Lumex services.
//
// This package implements a Redis client with connection pooling,
// health checks, and proper lifecycle management.
//
// Example:
//
//	client, err := redis.NewClient(redis.Config{
//	    Host:     "localhost",
//	    Port:     6379,
//	    Password: "",
//	    DB:       0,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Use the client
//	err = client.Set(ctx, "key", "value", time.Hour)
package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds the configuration for connecting to Redis
type Config struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	IdleTimeout  time.Duration
}

// DefaultConfig returns a Redis configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}
}

// Client wraps the Redis client with additional functionality
type Client struct {
	client *redis.Client
	mu     sync.RWMutex
	closed bool
	config Config
}

// NewClient creates a new Redis client with the given configuration
func NewClient(config Config) (*Client, error) {
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

	client := redis.NewClient(&redis.Options{
		Addr:            fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:        config.Password,
		DB:              config.DB,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		MaxRetries:      config.MaxRetries,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolTimeout:     config.PoolTimeout,
		ConnMaxIdleTime: config.IdleTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

// Client returns the underlying redis.Client for use with redis commands
func (c *Client) Client() *redis.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// Close closes the Redis client
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.client.Close()
}

// IsClosed returns true if the client has been closed
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Ping checks if the Redis server is responding
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	return c.Client().Ping(ctx).Err()
}

// HealthCheck performs a health check on the Redis connection
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Ping to check connection
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}

// Stats returns connection pool statistics
func (c *Client) Stats() *redis.PoolStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Client().PoolStats()
}

// Config returns the client configuration
func (c *Client) Config() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Set sets a key-value pair with expiration
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client().Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.Client().Get(ctx, key).Result()
}

// Del deletes keys
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.Client().Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.Client().Exists(ctx, keys...).Result()
}

// Expire sets a key's expiration time
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.Client().Expire(ctx, key, expiration).Err()
}

// TTL returns the time-to-live for a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.Client().TTL(ctx, key).Result()
}

// FlushDB deletes all keys in the current database
func (c *Client) FlushDB(ctx context.Context) error {
	return c.Client().FlushDB(ctx).Err()
}

// Info returns information about the Redis server
func (c *Client) Info(ctx context.Context, section ...string) (string, error) {
	return c.Client().Info(ctx, section...).Result()
}

// DBSize returns the number of keys in the current database
func (c *Client) DBSize(ctx context.Context) (int64, error) {
	return c.Client().DBSize(ctx).Result()
}

// Keys returns all keys matching a pattern
func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.Client().Keys(ctx, pattern).Result()
}

// Scan iterates over keys matching a pattern
func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	return c.Client().Scan(ctx, cursor, match, count).Result()
}
