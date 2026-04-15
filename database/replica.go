// Package database provides read replica support for database scalability.
//
// This package implements read/write splitting with automatic replica health
// monitoring and failover for improved database scalability.
//
// Example:
//
//	manager := database.NewReplicaManager()
//
//	// Add primary (write) database
//	primary, err := database.NewPool(primaryConfig)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	manager.SetPrimary(primary)
//
//	// Add read replicas
//	replica1, _ := database.NewPool(replicaConfig1)
//	replica2, _ := database.NewPool(replicaConfig2)
//	manager.AddReplica("replica1", replica1)
//	manager.AddReplica("replica2", replica2)
//
//	// Use for queries
//	tx, err := manager.Primary().DB().Begin()
//	rows, err := manager.Replica().DB().Query("SELECT * FROM users")
package database

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicaStatus represents the health status of a replica
type ReplicaStatus struct {
	Name        string
	Healthy     bool
	Lag         time.Duration // Replication lag
	LastCheck   time.Time
	ErrorCount  int64
	TotalChecks int64
}

// ReplicaManager manages read replicas with automatic failover
type ReplicaManager struct {
	primary       *Pool
	replicas      map[string]*ManagedReplica
	mu            sync.RWMutex
	checkInterval time.Duration
	healthyCount  int64
	totalCount    int64
}

// ManagedReplica wraps a Pool with health monitoring
type ManagedReplica struct {
	*Pool
	status    ReplicaStatus
	mu        sync.RWMutex
	lastError error
}

// NewReplicaManager creates a new replica manager
func NewReplicaManager(logger interface{}) *ReplicaManager {
	// Logger is optional - if not provided, logs will be discarded
	return &ReplicaManager{
		replicas:      make(map[string]*ManagedReplica),
		checkInterval: 30 * time.Second,
	}
}

// SetPrimary sets the primary database for write operations
func (rm *ReplicaManager) SetPrimary(pool *Pool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.primary = pool
}

// Primary returns the primary database connection
func (rm *ReplicaManager) Primary() *Pool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.primary
}

// AddReplica adds a read replica to the manager
func (rm *ReplicaManager) AddReplica(name string, pool *Pool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if pool == nil {
		return fmt.Errorf("pool cannot be nil")
	}

	replica := &ManagedReplica{
		Pool: pool,
		status: ReplicaStatus{
			Name:        name,
			Healthy:     true,
			LastCheck:   time.Now(),
			TotalChecks: 0,
		},
	}

	rm.replicas[name] = replica

	// Start health monitoring
	go rm.monitorReplicaHealth(name, replica)

	return nil
}

// RemoveReplica removes a read replica from the manager
func (rm *ReplicaManager) RemoveReplica(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if replica, exists := rm.replicas[name]; exists {
		replica.mu.Lock()
		replica.status.Healthy = false
		replica.mu.Unlock()

		delete(rm.replicas, name)
	}
}

// Replica returns a healthy read replica using round-robin selection
func (rm *ReplicaManager) Replica() *Pool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	atomic.AddInt64(&rm.totalCount, 1)

	// Get all healthy replicas
	healthyReplicas := make([]*ManagedReplica, 0)
	for _, replica := range rm.replicas {
		replica.mu.RLock()
		healthy := replica.status.Healthy
		replica.mu.RUnlock()

		if healthy {
			healthyReplicas = append(healthyReplicas, replica)
		}
	}

	if len(healthyReplicas) == 0 {
		// No healthy replicas, fall back to primary
		atomic.AddInt64(&rm.healthyCount, 0)
		return rm.primary
	}

	atomic.AddInt64(&rm.healthyCount, 1)

	// Select replica using round-robin for load distribution
	idx := rand.Intn(len(healthyReplicas))
	return healthyReplicas[idx].Pool
}

// ReplicaByName returns a specific replica by name
func (rm *ReplicaManager) ReplicaByName(name string) *Pool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if replica, exists := rm.replicas[name]; exists {
		replica.mu.RLock()
		healthy := replica.status.Healthy
		replica.mu.RUnlock()

		if healthy {
			return replica.Pool
		}
	}

	return rm.primary
}

// GetReplicaStatus returns the status of all replicas
func (rm *ReplicaManager) GetReplicaStatus() map[string]ReplicaStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	statuses := make(map[string]ReplicaStatus, len(rm.replicas))
	for name, replica := range rm.replicas {
		replica.mu.RLock()
		statuses[name] = replica.status
		replica.mu.RUnlock()
	}

	return statuses
}

// HealthyReplicaCount returns the number of healthy replicas
func (rm *ReplicaManager) HealthyReplicaCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	count := 0
	for _, replica := range rm.replicas {
		replica.mu.RLock()
		if replica.status.Healthy {
			count++
		}
		replica.mu.RUnlock()
	}

	return count
}

// HealthRatio returns the ratio of healthy replica requests to total requests
func (rm *ReplicaManager) HealthRatio() float64 {
	total := atomic.LoadInt64(&rm.totalCount)
	if total == 0 {
		return 0
	}
	healthy := atomic.LoadInt64(&rm.healthyCount)
	return float64(healthy) / float64(total) * 100
}

// monitorReplicaHealth monitors the health of a specific replica
func (rm *ReplicaManager) monitorReplicaHealth(name string, replica *ManagedReplica) {
	ticker := time.NewTicker(rm.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := replica.Ping(ctx)
		cancel()

		replica.mu.Lock()
		replica.status.TotalChecks++
		replica.status.LastCheck = time.Now()

		if err != nil {
			replica.status.Healthy = false
			replica.status.ErrorCount++
			replica.lastError = err
		} else {
			replica.status.Healthy = true
			replica.status.Lag = rm.measureReplicationLag(ctx, replica)
			replica.lastError = nil
		}
		replica.mu.Unlock()
	}
}

// measureReplicationLag measures the replication lag of a replica
func (rm *ReplicaManager) measureReplicationLag(ctx context.Context, replica *ManagedReplica) time.Duration {
	// Query the replication lag from pg_stat_replication or similar
	// This is a simplified version - in production you'd query actual replication stats
	var lag time.Duration

	// For PostgreSQL, you would query:
	// SELECT CASE WHEN pg_is_in_recovery() THEN
	//   GREATEST(
	//     pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()),
	//     pg_wal_lsn_diff(pg_current_wal_lsn(), pg_last_wal_replay_lsn())
	//   )
	// ELSE 0 END AS lag

	// Simplified: just check if we can query
	err := replica.Ping(ctx)
	if err != nil {
		lag = time.Hour // Mark as high lag if unreachable
	} else {
		lag = 0 // Assume minimal lag if reachable
	}

	return lag
}

// Close closes all connections (primary and replicas)
func (rm *ReplicaManager) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var lastErr error

	// Close primary
	if rm.primary != nil {
		if err := rm.primary.Close(); err != nil {
			lastErr = err
		}
	}

	// Close all replicas
	for _, replica := range rm.replicas {
		if err := replica.Close(); err != nil {
			lastErr = err
		}
	}

	rm.replicas = make(map[string]*ManagedReplica)

	return lastErr
}

// ExecuteInTx executes a function within a transaction on the primary
func (rm *ReplicaManager) ExecuteInTx(ctx context.Context, fn func(*sql.Tx) error) error {
	primary := rm.Primary()
	if primary == nil {
		return fmt.Errorf("no primary database configured")
	}

	primary.mu.RLock()
	defer primary.mu.RUnlock()

	if primary.closed {
		return fmt.Errorf("primary database is closed")
	}

	db := primary.DB()
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			lastErr = err
			continue
		}

		if err := fn(tx); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("function error: %w, rollback error: %v", err, rbErr)
			}
			lastErr = err

			if ctx.Err() != nil {
				return err
			}
			continue
		}

		if err := tx.Commit(); err != nil {
			lastErr = err

			if ctx.Err() != nil {
				return err
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("transaction failed after %d attempts: %w", maxRetries, lastErr)
}

// QueryOnReplica executes a read query on a replica (or primary if no replicas available)
func (rm *ReplicaManager) QueryOnReplica(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	replica := rm.Replica()
	if replica == nil {
		return nil, fmt.Errorf("no database available")
	}

	return replica.DB().QueryContext(ctx, query, args...)
}

// QueryRowOnReplica executes a read query on a replica (or primary if no replicas available)
func (rm *ReplicaManager) QueryRowOnReplica(ctx context.Context, query string, args ...interface{}) *sql.Row {
	replica := rm.Replica()
	if replica == nil {
		return nil
	}

	return replica.DB().QueryRowContext(ctx, query, args...)
}

// ExecOnPrimary executes a write query on the primary
func (rm *ReplicaManager) ExecOnPrimary(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	primary := rm.Primary()
	if primary == nil {
		return nil, fmt.Errorf("no primary database configured")
	}

	return primary.DB().ExecContext(ctx, query, args...)
}

// GetStats returns statistics about the replica manager
func (rm *ReplicaManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_replicas":   len(rm.replicas),
		"healthy_replicas": rm.HealthyReplicaCount(),
		"health_ratio":     rm.HealthRatio(),
		"total_requests":   atomic.LoadInt64(&rm.totalCount),
		"healthy_requests": atomic.LoadInt64(&rm.healthyCount),
	}

	if rm.primary != nil {
		stats["primary_available"] = true
		stats["primary_stats"] = rm.primary.Stats()
	} else {
		stats["primary_available"] = false
	}

	return stats
}

// String returns a string representation of the replica manager status
func (rm *ReplicaManager) String() string {
	stats := rm.GetStats()
	return fmt.Sprintf("ReplicaManager{Total: %d, Healthy: %d, HealthRatio: %.1f%%}",
		stats["total_replicas"],
		stats["healthy_replicas"],
		stats["health_ratio"])
}
