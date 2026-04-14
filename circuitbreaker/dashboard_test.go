package circuitbreaker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpmesh "github.com/azghr/mesh/http"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	cb := httpmesh.NewCircuitBreaker(nil)
	Register("test-breaker", cb)

	assert.Equal(t, 1, Count())
}

func TestUnregister(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	cb := httpmesh.NewCircuitBreaker(nil)
	Register("test-breaker", cb)
	assert.Equal(t, 1, Count())

	Unregister("test-breaker")
	assert.Equal(t, 0, Count())
}

func TestHandler(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	cb := httpmesh.NewCircuitBreaker(nil)
	Register("my-service", cb)

	handler := Handler()
	req := httptest.NewRequest(http.MethodGet, "/debug/circuit-breakers", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), response["total"])
}

func TestHandlerWithNames(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	cb := httpmesh.NewCircuitBreaker(nil)
	Register("api-service", cb)

	handler := HandlerWithNames()
	req := httptest.NewRequest(http.MethodGet, "/debug/circuit-breakers", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)

	breakers := response["breakers"].(map[string]interface{})
	assert.Contains(t, breakers, "api-service")
}

func TestResetAll(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	cb := httpmesh.NewCircuitBreaker(nil)
	Register("test", cb)

	// Trigger some state changes
	cb.Execute(func() error { return nil })
	cb.Execute(func() error { return assert.AnError })

	ResetAll()

	// Verify all breakers are reset
	assert.True(t, cb.IsClosed())
}

func TestCount_Empty(t *testing.T) {
	// Clear registry
	globalRegistry.mu.Lock()
	globalRegistry.breakers = make(map[string]*httpmesh.CircuitBreaker)
	globalRegistry.nameToID = make(map[string]string)
	globalRegistry.mu.Unlock()

	assert.Equal(t, 0, Count())
}
