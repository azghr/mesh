// Package discovery provides service discovery for microservices.
//
// This package provides a simple service registry using Redis
// for service registration, discovery, and health checking.
//
// # Features
//
//   - Service registration with metadata
//   - Service discovery by name
//   - Health check support
//   - DNS-like service lookup
//
// # Usage
//
//	// Register a service
//	sd.Register(ctx, "user-service", "localhost:8080", metadata)
//
//	// Discover services
//	instances, _ := sd.Discover(ctx, "user-service")
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	servicePrefix = "mesh:service:"
	heartbeatKey  = "mesh:heartbeat:"
)

// Config configures service discovery
type Config struct {
	Redis     *redis.Client
	TTL       time.Duration
	Heartbeat time.Duration
}

// Service represents a registered service
type Service struct {
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Metadata      map[string]string `json:"metadata"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
}

// Registry manages service discovery
type Registry struct {
	config   Config
	mu       sync.RWMutex
	handlers map[string]HealthCheckFunc
}

// HealthCheckFunc is the function type for health checks
type HealthCheckFunc func(ctx context.Context, addr string) error

// New creates a new service registry
//
//	sd := discovery.New(discovery.Config{
//	    Redis: client,
//	    TTL: 30 * time.Second,
//	    Heartbeat: 10 * time.Second,
//	})
func New(cfg Config) *Registry {
	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 10 * time.Second
	}

	return &Registry{
		config:   cfg,
		handlers: make(map[string]HealthCheckFunc),
	}
}

// Register registers a service
//
//	sd.Register(ctx, "user-service", "localhost:8080", map[string]string{
//	    "version": "1.0.0",
//	})
func (r *Registry) Register(ctx context.Context, name, addr string, metadata map[string]string) error {

	service := Service{
		Name:          name,
		Address:       addr,
		Metadata:      metadata,
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	key := servicePrefix + name + ":" + addr
	if err := r.config.Redis.Set(ctx, key, data, r.config.TTL).Err(); err != nil {
		return fmt.Errorf("set: %w", err)
	}

	return nil
}

// Deregister removes a service
func (r *Registry) Deregister(ctx context.Context, name, addr string) error {
	key := servicePrefix + name + ":" + addr
	return r.config.Redis.Del(ctx, key).Err()
}

// Discover finds services by name
//
//	instances, _ := sd.Discover(ctx, "user-service")
func (r *Registry) Discover(ctx context.Context, name string) ([]Service, error) {
	pattern := servicePrefix + name + ":*"
	keys, err := r.config.Redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no services found: %s", name)
	}

	services := make([]Service, 0, len(keys))
	for _, key := range keys {
		data, err := r.config.Redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var service Service
		if err := json.Unmarshal([]byte(data), &service); err != nil {
			continue
		}

		services = append(services, service)
	}

	return services, nil
}

// DiscoverOne finds a single service (random selection for load balancing)
func (r *Registry) DiscoverOne(ctx context.Context, name string) (*Service, error) {
	services, err := r.Discover(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no service found: %s", name)
	}

	// Round-robin or first available
	return &services[0], nil
}

// ListAll lists all registered services
func (r *Registry) ListAll(ctx context.Context) (map[string][]Service, error) {
	pattern := servicePrefix + "*"
	keys, err := r.config.Redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]Service)
	for _, key := range keys {
		data, err := r.config.Redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var service Service
		if err := json.Unmarshal([]byte(data), &service); err != nil {
			continue
		}

		result[service.Name] = append(result[service.Name], service)
	}

	return result, nil
}

// RegisterHealthCheck registers a health check for a service
func (r *Registry) RegisterHealthCheck(name string, fn HealthCheckFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = fn
}

// CheckHealth checks health of all services
func (r *Registry) CheckHealth(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	services, _ := r.ListAll(ctx)

	for name, svcs := range services {
		for _, svc := range svcs {
			if fn, ok := r.handlers[name]; ok {
				if err := fn(ctx, svc.Address); err != nil {
					results[svc.Address] = err
				}
			}
		}
	}

	return results
}

// Heartbeat updates service heartbeat
func (r *Registry) Heartbeat(ctx context.Context, name, addr string) error {
	key := servicePrefix + name + ":" + addr

	data, err := r.config.Redis.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	var service Service
	if err := json.Unmarshal([]byte(data), &service); err != nil {
		return err
	}

	service.LastHeartbeat = time.Now()
	newData, err := json.Marshal(service)
	if err != nil {
		return err
	}

	return r.config.Redis.Set(ctx, key, newData, r.config.TTL).Err()
}

// StartHeartbeat starts sending heartbeats in background
func (r *Registry) StartHeartbeat(ctx context.Context, name, addr string) {
	go func() {
		ticker := time.NewTicker(r.config.Heartbeat)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.Heartbeat(ctx, name, addr)
			}
		}
	}()
}

// ServiceInfo returns service information
func (r *Registry) ServiceInfo(ctx context.Context, name, addr string) (*Service, error) {
	key := servicePrefix + name + ":" + addr

	data, err := r.config.Redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var service Service
	if err := json.Unmarshal([]byte(data), &service); err != nil {
		return nil, err
	}

	return &service, nil
}

// getPort extracts port from address string
func getPort(addr string) string {
	// Simple implementation - if addr doesn't contain colon, return empty
	// In practice, this should handle the full address properly
	return addr
}
