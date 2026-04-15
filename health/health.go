package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RedisCheck creates a health check function for Redis
func RedisCheck(client interface {
	Ping(ctx context.Context) error
}) Check {
	return func(ctx context.Context) error {
		return client.Ping(ctx)
	}
}

// DatabaseCheck creates a health check function for database connections
func DatabaseCheck(pingable interface {
	Ping(ctx context.Context) error
}) Check {
	return func(ctx context.Context) error {
		return pingable.Ping(ctx)
	}
}

// Status represents the health status
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusWarn    Status = "warn"
	StatusUnknown Status = "unknown"
)

// Check is a function that performs a health check
type Check func(ctx context.Context) error

// HealthCheck represents a single health check
type HealthCheck struct {
	name        string
	check       Check
	timeout     time.Duration
	lastStatus  Status
	lastError   error
	lastChecked time.Time
	mu          sync.RWMutex
}

// Result represents the result of a health check
type Result struct {
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration,omitempty"`
}

// Checker manages health checks
type Checker struct {
	checks map[string]*HealthCheck
	mu     sync.RWMutex
}

// NewChecker creates a new health checker
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]*HealthCheck),
	}
}

// Register registers a new health check
func (c *Checker) Register(name string, check Check) {
	c.RegisterWithOptions(name, check, 30*time.Second)
}

// RegisterWithOptions registers a new health check with options
func (c *Checker) RegisterWithOptions(name string, check Check, timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checks[name] = &HealthCheck{
		name:    name,
		check:   check,
		timeout: timeout,
	}
}

// Unregister removes a health check
func (c *Checker) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.checks, name)
}

// Check runs all health checks and returns their results
func (c *Checker) Check(ctx context.Context) map[string]Result {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]Result, len(c.checks))

	for name, hc := range c.checks {
		results[name] = c.runCheck(ctx, hc)
	}

	return results
}

// CheckOne runs a specific health check
func (c *Checker) CheckOne(ctx context.Context, name string) (Result, error) {
	c.mu.RLock()
	hc, exists := c.checks[name]
	c.mu.RUnlock()

	if !exists {
		return Result{}, fmt.Errorf("health check '%s' not found", name)
	}

	return c.runCheck(ctx, hc), nil
}

// runCheck executes a single health check
func (c *Checker) runCheck(ctx context.Context, hc *HealthCheck) Result {
	start := time.Now()

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	// Run the check
	err := hc.check(checkCtx)
	duration := time.Since(start)

	// Update health check state
	hc.mu.Lock()
	hc.lastChecked = start
	hc.lastError = err

	if err != nil {
		hc.lastStatus = StatusFail
	} else {
		hc.lastStatus = StatusPass
	}
	hc.mu.Unlock()

	// Build result
	result := Result{
		Name:      hc.name,
		Timestamp: start,
		Duration:  duration.String(),
	}

	if err != nil {
		result.Status = StatusFail
		result.Error = err.Error()
	} else {
		result.Status = StatusPass
	}

	return result
}

// Status returns the overall health status
func (c *Checker) Status(ctx context.Context) Status {
	results := c.Check(ctx)

	for _, result := range results {
		if result.Status == StatusFail {
			return StatusFail
		}
	}

	return StatusPass
}

// GetLastStatus returns the last known status of a health check
func (c *Checker) GetLastStatus(name string) (Status, error) {
	c.mu.RLock()
	hc, exists := c.checks[name]
	c.mu.RUnlock()

	if !exists {
		return StatusUnknown, fmt.Errorf("health check '%s' not found", name)
	}

	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.lastStatus, nil
}

// GetLastChecked returns the last time a health check was run
func (c *Checker) GetLastChecked(name string) (time.Time, error) {
	c.mu.RLock()
	hc, exists := c.checks[name]
	c.mu.RUnlock()

	if !exists {
		return time.Time{}, fmt.Errorf("health check '%s' not found", name)
	}

	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.lastChecked, nil
}

// List returns all registered health check names
func (c *Checker) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.checks))
	for name := range c.checks {
		names = append(names, name)
	}

	return names
}

// Count returns the number of registered health checks
func (c *Checker) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.checks)
}
