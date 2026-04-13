package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResilientClient_Get_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewResilientClient(DefaultResilientClientConfig("test-service"))
	resp, err := client.Get(server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestResilientClient_Get_RetryOn500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	config := DefaultResilientClientConfig("test-service")
	config.RetryConfig.MaxRetries = 5
	config.RetryConfig.InitialDelay = 10 * time.Millisecond

	client := NewResilientClient(config)
	resp, err := client.Get(server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, attempts)
	resp.Body.Close()
}

func TestResilientClient_Get_NoRetryOn400(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client := NewResilientClient(DefaultResilientClientConfig("test-service"))
	resp, err := client.Get(server.URL)

	require.Error(t, err)
	assert.NotNil(t, resp) // Response is returned even on error
	assert.Contains(t, err.Error(), "HTTP 400")
	assert.Equal(t, 1, attempts, "should not retry on 400 errors")
	if resp != nil {
		resp.Body.Close()
	}
}

func TestResilientClient_Get_CircuitBreaker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := DefaultResilientClientConfig("test-service")
	config.CircuitBreakerConfig.MaxFailures = 3
	config.RetryConfig.MaxRetries = 1
	config.RetryConfig.InitialDelay = 10 * time.Millisecond

	client := NewResilientClient(config)

	// Trigger failures to open circuit
	for i := 0; i < 4; i++ {
		_, _ = client.Get(server.URL)
	}

	// Circuit should now be open
	assert.True(t, client.CircuitBreaker().IsOpen())

	// Next call should fail immediately
	_, err := client.Get(server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPEN")
}

func TestResilientClient_GetJSON(t *testing.T) {
	type Response struct {
		Message string `json:"message"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()

	client := NewResilientClient(DefaultResilientClientConfig("test-service"))

	var result Response
	err := client.GetJSON(server.URL, &result)

	require.NoError(t, err)
	assert.Equal(t, "hello", result.Message)
}

func TestResilientClient_PostJSON(t *testing.T) {
	type Request struct {
		Name string `json:"name"`
	}
	type Response struct {
		ID string `json:"id"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Response{ID: "123"})
	}))
	defer server.Close()

	client := NewResilientClient(DefaultResilientClientConfig("test-service"))

	var result Response
	err := client.PostJSON(server.URL, Request{Name: "test"}, &result)

	require.NoError(t, err)
	assert.Equal(t, "123", result.ID)
}

func TestResilientClient_PostJSON_RetryOn500(t *testing.T) {
	type Request struct {
		Name string `json:"name"`
	}
	type Response struct {
		Status string `json:"status"`
	}

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Response{Status: "created"})
	}))
	defer server.Close()

	config := DefaultResilientClientConfig("test-service")
	config.RetryConfig.MaxRetries = 5
	config.RetryConfig.InitialDelay = 10 * time.Millisecond

	client := NewResilientClient(config)

	var result Response
	err := client.PostJSON(server.URL, Request{Name: "test"}, &result)

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, "created", result.Status)
}

func TestResilientClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultResilientClientConfig("test-service")
	config.HTTPTimeout = 50 * time.Millisecond

	client := NewResilientClient(config)

	_, err := client.Get(server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed")
}

func TestResilientClient_ServiceName(t *testing.T) {
	config := DefaultResilientClientConfig("my-service")
	client := NewResilientClient(config)

	assert.Equal(t, "my-service", client.ServiceName())
}
