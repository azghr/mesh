package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackRegistry_Register(t *testing.T) {
	reg := NewFallbackRegistry()

	fb := FallbackResponse{
		Content:    map[string]string{"name": "test"},
		StatusCode: 200,
		TTL:        time.Minute,
	}
	reg.Register("users", fb)

	got, ok := reg.Get("users")
	require.True(t, ok, "should find fallback")
	assert.Equal(t, "test", got.Content.(map[string]string)["name"])
	assert.Equal(t, 200, got.StatusCode)
}

func TestFallbackRegistry_Get_NotFound(t *testing.T) {
	reg := NewFallbackRegistry()

	_, ok := reg.Get("missing")
	assert.False(t, ok, "should not find fallback")
}

func TestFallbackRegistry_Unregister(t *testing.T) {
	reg := NewFallbackRegistry()
	reg.Register("users", FallbackResponse{Content: "test"})

	reg.Unregister("users")

	_, ok := reg.Get("users")
	assert.False(t, ok, "should not find fallback after unregister")
}

func TestFallbackRegistry_List(t *testing.T) {
	reg := NewFallbackRegistry()
	reg.Register("users", FallbackResponse{Content: "users"})
	reg.Register("orders", FallbackResponse{Content: "orders"})

	list := reg.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "users")
	assert.Contains(t, list, "orders")
}

func TestSimpleCache_SetAndGet(t *testing.T) {
	cache := NewSimpleCache()

	cache.Set("key1", "value1", time.Minute)

	got, ok := cache.Get("key1")
	require.True(t, ok)
	assert.Equal(t, "value1", got)
}

func TestSimpleCache_Get_Expired(t *testing.T) {
	cache := NewSimpleCache()

	cache.Set("key1", "value1", -time.Second)

	_, ok := cache.Get("key1")
	assert.False(t, ok, "should not return expired item")
}

func TestSimpleCache_Get_Missing(t *testing.T) {
	cache := NewSimpleCache()

	_, ok := cache.Get("missing")
	assert.False(t, ok)
}

func TestSimpleCache_Delete(t *testing.T) {
	cache := NewSimpleCache()
	cache.Set("key1", "value1", time.Minute)

	cache.Delete("key1")

	_, ok := cache.Get("key1")
	assert.False(t, ok)
}

func TestFallbackClient_NewFallbackClient(t *testing.T) {
	client := NewFallbackClient(nil)

	assert.NotNil(t, client)
	assert.NotNil(t, client.circuitBreaker)
	assert.NotNil(t, client.fallbacks)
	assert.NotNil(t, client.cache)
}

func TestFallbackClient_Execute_Success(t *testing.T) {
	client := NewFallbackClient(nil)
	client.SetFallback("users", FallbackResponse{
		Content:    "fallback",
		StatusCode: 200,
	})

	result, err := client.Execute(context.Background(), "users", func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestFallbackClient_Execute_UseFallback(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	client := NewFallbackClient(cb)

	client.SetFallback("users", FallbackResponse{
		Content:    "fallback",
		StatusCode: 200,
		TTL:        time.Minute,
	})

	// Force circuit open
	_ = cb.Execute(func() error { return errors.New("fail") })

	// Wait for fallback
	result, err := client.Execute(context.Background(), "users", func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err, "should return fallback, no error")
	assert.Equal(t, "fallback", result)
}

func TestFallbackClient_Execute_NoFallback(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	client := NewFallbackClient(cb)

	// No fallback registered

	// Force circuit open
	_ = cb.Execute(func() error { return errors.New("fail") })

	// Execute without fallback
	result, err := client.Execute(context.Background(), "users", func() (interface{}, error) {
		return "success", nil
	})

	assert.Error(t, err, "should return error when circuit open and no fallback")
	assert.Nil(t, result)
}

func TestFallbackClient_ExecuteWithFallback(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	client := NewFallbackClient(cb)

	client.SetFallback("users", FallbackResponse{
		Content:    "fallback",
		StatusCode: 200,
	})

	// Force circuit open
	_ = cb.Execute(func() error { return errors.New("fail") })

	result, err := client.ExecuteWithFallback(context.Background(), "users", func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "fallback", result)
}
