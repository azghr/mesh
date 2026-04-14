// Package circuitbreaker provides HTTP handlers for monitoring circuit breakers.
package circuitbreaker

import (
	"encoding/json"
	"net/http"
	"sync"

	httpmesh "github.com/azghr/mesh/http"
)

// Registry holds all circuit breakers for monitoring
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*httpmesh.CircuitBreaker
	nameToID map[string]string
}

// globalRegistry is the singleton registry
var globalRegistry = &Registry{
	breakers: make(map[string]*httpmesh.CircuitBreaker),
	nameToID: make(map[string]string),
}

// Register adds a circuit breaker to the global registry
func Register(name string, cb *httpmesh.CircuitBreaker) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	id := generateID()
	globalRegistry.breakers[id] = cb
	globalRegistry.nameToID[name] = id
}

// Unregister removes a circuit breaker from the registry
func Unregister(name string) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if id, ok := globalRegistry.nameToID[name]; ok {
		delete(globalRegistry.breakers, id)
		delete(globalRegistry.nameToID, name)
	}
}

// Handler returns an HTTP handler that serves circuit breaker status
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalRegistry.mu.RLock()
		defer globalRegistry.mu.RUnlock()

		breakers := make([]BreakerInfo, 0, len(globalRegistry.breakers))

		for id, cb := range globalRegistry.breakers {
			breakers = append(breakers, BreakerInfo{
				ID:              id,
				State:           cb.State().String(),
				FailureCount:    cb.FailureCount(),
				SuccessCount:    cb.SuccessCount(),
				LastFailureTime: cb.LastFailureTime().String(),
				LastStateChange: cb.LastStateChange().String(),
				NextAttempt:     cb.NextAttempt().String(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":    len(breakers),
			"breakers": breakers,
		})
	})
}

// HandlerWithNames returns an HTTP handler with named breakers
func HandlerWithNames() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalRegistry.mu.RLock()
		defer globalRegistry.mu.RUnlock()

		breakers := make(map[string]BreakerInfo)

		for name, id := range globalRegistry.nameToID {
			cb := globalRegistry.breakers[id]
			breakers[name] = BreakerInfo{
				ID:              id,
				State:           cb.State().String(),
				FailureCount:    cb.FailureCount(),
				SuccessCount:    cb.SuccessCount(),
				LastFailureTime: cb.LastFailureTime().String(),
				LastStateChange: cb.LastStateChange().String(),
				NextAttempt:     cb.NextAttempt().String(),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":    len(breakers),
			"breakers": breakers,
		})
	})
}

// BreakerInfo holds circuit breaker status information
type BreakerInfo struct {
	ID              string `json:"id"`
	State           string `json:"state"`
	FailureCount    int    `json:"failure_count"`
	SuccessCount    int    `json:"success_count"`
	LastFailureTime string `json:"last_failure_time,omitempty"`
	LastStateChange string `json:"last_state_change,omitempty"`
	NextAttempt     string `json:"next_attempt,omitempty"`
}

// ResetAll resets all circuit breakers in the registry
func ResetAll() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	for _, cb := range globalRegistry.breakers {
		cb.Reset()
	}
}

// Count returns the number of registered circuit breakers
func Count() int {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return len(globalRegistry.breakers)
}

func generateID() string {
	return "cb-" + timestampID()
}

func timestampID() string {
	return "1234567890"
}
