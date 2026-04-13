package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResilientClient combines circuit breaker and retry logic for resilient HTTP calls
type ResilientClient struct {
	httpClient      *http.Client
	circuitBreaker  *CircuitBreaker
	retryConfig     *RetryConfig
	serviceName     string
}

// ResilientClientConfig holds configuration for the resilient client
type ResilientClientConfig struct {
	// HTTPTimeout is the timeout for HTTP requests
	HTTPTimeout time.Duration
	// CircuitBreakerConfig is the circuit breaker configuration
	CircuitBreakerConfig *CircuitBreakerConfig
	// RetryConfig is the retry configuration
	RetryConfig *RetryConfig
	// ServiceName is the name of the service being called
	ServiceName string
}

// DefaultResilientClientConfig returns default configuration
func DefaultResilientClientConfig(serviceName string) *ResilientClientConfig {
	return &ResilientClientConfig{
		HTTPTimeout:          30 * time.Second,
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
		RetryConfig:          DefaultRetryConfig(),
		ServiceName:          serviceName,
	}
}

// NewResilientClient creates a new resilient HTTP client
func NewResilientClient(config *ResilientClientConfig) *ResilientClient {
	if config == nil {
		config = DefaultResilientClientConfig("unknown")
	}

	return &ResilientClient{
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
		circuitBreaker: NewCircuitBreaker(config.CircuitBreakerConfig),
		retryConfig:    config.RetryConfig,
		serviceName:    config.ServiceName,
	}
}

// Do executes an HTTP request with circuit breaker and retry logic
func (c *ResilientClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response

	err := c.circuitBreaker.Execute(func() error {
		var err error
		resp, err = c.retryRequest(req)
		return err
	})

	return resp, err
}

// retryRequest executes a request with retry logic
func (c *ResilientClient) retryRequest(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	var resp *http.Response
	err := RetryWithContext(ctx, func() error {
		// Clone the request for each retry
		reqClone := req.Clone(ctx)

		var err error
		resp, err = c.httpClient.Do(reqClone)
		if err != nil {
			// Network errors are retryable
			return NewRetryableError(fmt.Errorf("HTTP request failed: %w", err))
		}

		// Check status code for retryable errors
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return NewRetryableError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
		}

		// Client errors (4xx) are not retryable
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return nil
	}, c.retryConfig)

	return resp, err
}

// Get executes a GET request
func (c *ResilientClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post executes a POST request with JSON body
func (c *ResilientClient) Post(url string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// GetJSON executes a GET request and decodes the JSON response
func (c *ResilientClient) GetJSON(url string, result interface{}) error {
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

// PostJSON executes a POST request with JSON body and decodes the response
func (c *ResilientClient) PostJSON(url string, body interface{}, result interface{}) error {
	resp, err := c.Post(url, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

// CircuitBreaker returns the circuit breaker for monitoring
func (c *ResilientClient) CircuitBreaker() *CircuitBreaker {
	return c.circuitBreaker
}

// ServiceName returns the service name
func (c *ResilientClient) ServiceName() string {
	return c.serviceName
}
