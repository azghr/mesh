// Package health provides health checking and monitoring utilities.
//
// This package includes health checkers, probes for Kubernetes liveness,
// readiness, and startup checks, and deep checks for infrastructure dependencies.
//
// # Overview
//
// The health package provides:
//
//   - DeepHealthChecker: Aggregates multiple probes by check type
//   - CheckType: liveness, readiness, and startup probe types
//   - Built-in checks: Database, Redis, and composite checks
//   - ReadinessResponse: JSON response format for /health endpoint
//
// This implementation follows Kubernetes probe patterns and integrates
// with connection pool monitoring for database readiness.
//
// # Probe Types
//
// Kubernetes supports three probe types:
//
//   - Liveness: Is the container alive? (restart if fail)
//   - Readiness: Is the container ready to serve traffic?
//   - Startup: Has the container started? (disable others until pass)
//
// # Basic Usage
//
// Create a health checker and register probes:
//
//	checker := health.NewDeepHealthChecker()
//
//	// Database readiness probe
//	checker.RegisterProbe(health.ProbeDefinition{
//	    Name:   "database",
//	    Type:   health.CheckTypeReadiness,
//	    Config: health.ProbeConfig{Timeout: 3 * time.Second, MaxConnectionUtilization: 0.9},
//	    Check:  health.DatabaseReadinessCheck(db, health.ProbeConfig{}),
//	})
//
//	// Redis readiness probe
//	checker.RegisterProbe(health.ProbeDefinition{
//	    Name:   "redis",
//	    Type:   health.CheckTypeReadiness,
//	    Config: health.ProbeConfig{Timeout: 3 * time.Second},
//	    Check:  health.RedisReadinessCheck(redisClient, health.ProbeConfig{}),
//	})
//
//	// Check readiness
//	response := checker.Readiness(ctx)
//
// # Built-in Check Functions
//
// The package provides ready-to-use check functions:
//
//	DatabaseReadinessCheck(db, cfg)   // DB query + connection pool utilization
//	DatabaseLivenessCheck(db)     // Simple DB ping
//	RedisReadinessCheck(client, cfg) // Redis ping with timeout
//	RedisLivenessCheck(client)    // Simple Redis ping
//	CompositeCheck(checks...)      // Run multiple checks
//	RequireAllCheck(name, checks...)    // All must pass
//	RequireAnyCheck(name, checks...)  // Any can pass
//
// # Setup Default Probes
//
// Quickly set up common probes:
//
//	health.SetupDefaultProbes(checker, db, redisClient)
//
// This registers liveness, database, and redis probes with defaults.
//
// # JSON Response
//
// The ReadinessResponse format:
//
//	{
//	  "status": "pass",
//	  "timestamp": "2026-04-16T12:00:00Z",
//	  "checks": {
//	    "database": {"status": "pass", "latency": "1.2ms"},
//	    "redis": {"status": "pass", "latency": "0.5ms"}
//	  }
//	}
//
// # Best Practices
//
//   - Liveness: Keep simple - just check if process is alive
//   - Readiness: Check all dependencies + connection pools
//   - Startup: Include initialization that must complete
//   - Timeouts: Set appropriate timeouts per probe type
//   - Connection pools: Monitor utilization in readiness
//   - Composite checks: Use RequireAnyCheck for optional deps
package health

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// CheckType represents the type of health check.
type CheckType string

const (
	CheckTypeLiveness  CheckType = "liveness"
	CheckTypeReadiness CheckType = "readiness"
	CheckTypeStartup   CheckType = "startup"
)

// ProbeConfig holds configuration for a health probe.
type ProbeConfig struct {
	Timeout                  time.Duration
	Critical                 bool
	MaxConnectionUtilization float64 // 0.0-1.0 percentage
}

// DefaultProbeConfigs returns default configurations for probes.
func DefaultProbeConfigs() map[CheckType]ProbeConfig {
	return map[CheckType]ProbeConfig{
		CheckTypeLiveness: {
			Timeout:  5 * time.Second,
			Critical: true,
		},
		CheckTypeReadiness: {
			Timeout:                  10 * time.Second,
			Critical:                 false,
			MaxConnectionUtilization: 0.9,
		},
		CheckTypeStartup: {
			Timeout:  30 * time.Second,
			Critical: true,
		},
	}
}

// ProbeDefinition defines a single probe.
type ProbeDefinition struct {
	Name   string
	Type   CheckType
	Check  Check
	Config ProbeConfig
}

// DeepHealthChecker provides comprehensive health checking.
type DeepHealthChecker struct {
	probes map[CheckType][]ProbeDefinition
	mu     sync.RWMutex
}

// NewDeepHealthChecker creates a new deep health checker.
func NewDeepHealthChecker() *DeepHealthChecker {
	return &DeepHealthChecker{
		probes: make(map[CheckType][]ProbeDefinition),
	}
}

// RegisterProbe registers a probe for a specific check type.
func (d *DeepHealthChecker) RegisterProbe(def ProbeDefinition) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.probes[def.Type] = append(d.probes[def.Type], def)
}

// UnregisterProbe removes a probe.
func (d *DeepHealthChecker) UnregisterProbe(name string, checkType CheckType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	probes := d.probes[checkType]
	for i, p := range probes {
		if p.Name == name {
			d.probes[checkType] = append(probes[:i], probes[i+1:]...)
			return
		}
	}
}

// CheckProbes runs all probes for a specific check type.
func (d *DeepHealthChecker) CheckProbes(ctx context.Context, checkType CheckType) map[string]Result {
	d.mu.RLock()
	probes := d.probes[checkType]
	d.mu.RUnlock()

	results := make(map[string]Result)
	for _, probe := range probes {
		results[probe.Name] = d.runProbe(ctx, probe)
	}

	return results
}

// AllChecks runs all registered probes.
func (d *DeepHealthChecker) AllChecks(ctx context.Context) map[string]Result {
	d.mu.RLock()
	allProbes := make([]ProbeDefinition, 0, len(d.probes)*2)
	for _, probes := range d.probes {
		allProbes = append(allProbes, probes...)
	}
	d.mu.RUnlock()

	results := make(map[string]Result)
	for _, probe := range allProbes {
		results[probe.Name] = d.runProbe(ctx, probe)
	}

	return results
}

// runProbe executes a single probe.
func (d *DeepHealthChecker) runProbe(ctx context.Context, probe ProbeDefinition) Result {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, probe.Config.Timeout)
	defer cancel()

	checkErr := probe.Check(checkCtx)
	duration := time.Since(start)

	result := Result{
		Name:      probe.Name,
		Timestamp: start,
		Duration:  duration.String(),
	}

	if checkErr != nil {
		result.Status = StatusFail
		result.Error = checkErr.Error()
	} else {
		result.Status = StatusPass
	}

	return result
}

// IsHealthy checks if a specific check type is healthy.
func (d *DeepHealthChecker) IsHealthy(ctx context.Context, checkType CheckType) bool {
	results := d.CheckProbes(ctx, checkType)

	for _, result := range results {
		if result.Status == StatusFail {
			return false
		}
	}

	return true
}

// Readiness returns the readiness status with details.
func (d *DeepHealthChecker) Readiness(ctx context.Context) ReadinessResponse {
	results := d.CheckProbes(ctx, CheckTypeReadiness)

	response := ReadinessResponse{
		Status:    StatusPass,
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}

	allHealthy := true
	for name, result := range results {
		checkResult := CheckResult{
			Status: string(result.Status),
		}
		if result.Error != "" {
			checkResult.Error = result.Error
		}
		if result.Duration != "" {
			checkResult.Latency = result.Duration
		}

		response.Checks[name] = checkResult

		if result.Status == StatusFail {
			allHealthy = false
		}
	}

	if !allHealthy {
		response.Status = StatusFail
	}

	return response
}

// ReadinessResponse represents the readiness check response.
type ReadinessResponse struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents a single check result.
type CheckResult struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DatabaseReadinessCheck creates a deep database readiness check.
func DatabaseReadinessCheck(db *sql.DB, cfg ProbeConfig) Check {
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.MaxConnectionUtilization == 0 {
		cfg.MaxConnectionUtilization = 0.9
	}

	return func(ctx context.Context) error {
		checkCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		var result int
		if err := db.QueryRowContext(checkCtx, "SELECT 1").Scan(&result); err != nil {
			return fmt.Errorf("database query failed: %w", err)
		}

		stats := db.Stats()
		if stats.OpenConnections == 0 {
			return fmt.Errorf("no open connections")
		}

		utilization := float64(stats.InUse) / float64(stats.OpenConnections)
		if utilization > cfg.MaxConnectionUtilization {
			return fmt.Errorf("high connection utilization: %.0f%%", utilization*100)
		}

		return nil
	}
}

// DatabaseLivenessCheck creates a simple database liveness check.
func DatabaseLivenessCheck(db *sql.DB) Check {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var result int
		if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
			return fmt.Errorf("database liveness check failed: %w", err)
		}

		return nil
	}
}

// RedisReadinessCheck creates a deep Redis readiness check.
func RedisReadinessCheck(client *redis.Client, cfg ProbeConfig) Check {
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		pong, err := client.Ping(ctx).Result()
		if err != nil {
			return fmt.Errorf("redis ping failed: %w", err)
		}

		if pong != "PONG" {
			return fmt.Errorf("unexpected redis response: %s", pong)
		}

		return nil
	}
}

// RedisLivenessCheck creates a simple Redis liveness check.
func RedisLivenessCheck(client *redis.Client) Check {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_, err := client.Ping(ctx).Result()
		return err
	}
}

// CompositeCheck combines multiple checks.
func CompositeCheck(checks ...Check) Check {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if err := check(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// RequireAllCheck requires all checks to pass.
func RequireAllCheck(name string, checks ...Check) Check {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if err := check(ctx); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		return nil
	}
}

// RequireAnyCheck requires at least one check to pass.
func RequireAnyCheck(name string, checks ...Check) Check {
	return func(ctx context.Context) error {
		var errs []string
		for _, check := range checks {
			checkErr := check(ctx)
			if checkErr == nil {
				return nil
			}
			errs = append(errs, checkErr.Error())
		}
		return fmt.Errorf("%s: all checks failed: %s", name, strings.Join(errs, "; "))
	}
}

// Example probe setups.
var (
	// DatabaseProbe is an example database probe.
	DatabaseProbe = ProbeDefinition{
		Name:   "database",
		Type:   CheckTypeReadiness,
		Config: ProbeConfig{Timeout: 3 * time.Second, MaxConnectionUtilization: 0.9},
	}

	// RedisProbe is an example Redis probe.
	RedisProbe = ProbeDefinition{
		Name:   "redis",
		Type:   CheckTypeReadiness,
		Config: ProbeConfig{Timeout: 3 * time.Second},
	}

	// LivenessProbe is an example liveness probe.
	LivenessProbe = ProbeDefinition{
		Name:   "liveness",
		Type:   CheckTypeLiveness,
		Config: ProbeConfig{Timeout: 5 * time.Second},
	}
)

// SetupDefaultProbes sets up default health probes.
func SetupDefaultProbes(checker *DeepHealthChecker, db *sql.DB, redisClient *redis.Client) {
	checker.RegisterProbe(ProbeDefinition{
		Name:   "liveness",
		Type:   CheckTypeLiveness,
		Config: ProbeConfig{Timeout: 5 * time.Second},
		Check:  DatabaseLivenessCheck(db),
	})

	checker.RegisterProbe(ProbeDefinition{
		Name:   "database",
		Type:   CheckTypeReadiness,
		Config: ProbeConfig{Timeout: 3 * time.Second, MaxConnectionUtilization: 0.9},
		Check:  DatabaseReadinessCheck(db, ProbeConfig{}),
	})

	checker.RegisterProbe(ProbeDefinition{
		Name:   "redis",
		Type:   CheckTypeReadiness,
		Config: ProbeConfig{Timeout: 3 * time.Second},
		Check:  RedisReadinessCheck(redisClient, ProbeConfig{}),
	})
}
