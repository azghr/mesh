package memorycache

import (
	"context"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	cache := New()

	if cache.config.MaxSize != 10000 {
		t.Errorf("expected MaxSize 10000, got %d", cache.config.MaxSize)
	}

	if cache.config.TTL != 5*time.Minute {
		t.Errorf("expected TTL 5min, got %v", cache.config.TTL)
	}
}

func TestNew_WithOptions(t *testing.T) {
	cache := New(
		WithMaxSize(100),
		WithTTL(time.Hour),
	)

	if cache.config.MaxSize != 100 {
		t.Errorf("expected MaxSize 100, got %d", cache.config.MaxSize)
	}

	if cache.config.TTL != time.Hour {
		t.Errorf("expected TTL 1hr, got %v", cache.config.TTL)
	}
}

func TestCache_SetAndGet(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)

	val, err := cache.Get("key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestCache_Get_NotFound(t *testing.T) {
	cache := New()

	_, err := cache.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCache_Get_Expired(t *testing.T) {
	cache := New(WithTTL(time.Millisecond))

	cache.Set("key1", "value1", time.Millisecond)

	time.Sleep(10 * time.Millisecond)

	_, err := cache.Get("key1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for expired key, got %v", err)
	}
}

func TestCache_Set_UpdatesExisting(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Set("key1", "value2", time.Minute)

	val, _ := cache.Get("key1")
	if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}

func TestCache_Delete(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Delete("key1")

	_, err := cache.Get("key1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCache_Clear(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}
}

func TestCache_LRU_Eviction(t *testing.T) {
	cache := New(WithMaxSize(3))

	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	cache.Set("key3", "value3", time.Minute)

	cache.Set("key4", "value4", time.Minute)

	_, err := cache.Get("key1")
	if err != ErrNotFound {
		t.Errorf("expected key1 to be evicted, got %v", err)
	}
}

func TestCache_GetOrSet_CacheHit(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)

	val, err := cache.GetOrSet(context.Background(), "key1", func() (any, error) {
		t.Error("fetchFn should not be called on cache hit")
		return nil, nil
	}, time.Minute)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestCache_GetOrSet_CacheMiss(t *testing.T) {
	cache := New()

	fetched := false
	val, err := cache.GetOrSet(context.Background(), "key1", func() (any, error) {
		fetched = true
		return "fetched", nil
	}, time.Minute)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fetched {
		t.Error("expected fetchFn to be called on cache miss")
	}

	if val != "fetched" {
		t.Errorf("expected fetched, got %v", val)
	}

	cachedVal, _ := cache.Get("key1")
	if cachedVal != "fetched" {
		t.Errorf("expected value to be cached, got %v", cachedVal)
	}
}

func TestCache_GetOrSet_FetchError(t *testing.T) {
	cache := New()

	_, err := cache.GetOrSet(context.Background(), "key1", func() (any, error) {
		return nil, context.DeadlineExceeded
	}, time.Minute)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestCache_Size(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)

	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}
}

func TestCache_Metrics(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Get("key1")
	cache.Get("nonexistent")

	metrics := cache.GetMetrics()

	if metrics.Size != 1 {
		t.Errorf("expected Size 1, got %d", metrics.Size)
	}

	if metrics.Hits != 1 {
		t.Errorf("expected Hits 1, got %d", metrics.Hits)
	}

	if metrics.Misses != 1 {
		t.Errorf("expected Misses 1, got %d", metrics.Misses)
	}
}

func TestCache_ResetMetrics(t *testing.T) {
	cache := New()

	cache.Set("key1", "value1", time.Minute)
	cache.Get("key1")

	cache.ResetMetrics()

	metrics := cache.GetMetrics()

	if metrics.Hits != 0 || metrics.Misses != 0 {
		t.Errorf("expected metrics to be reset, got Hits=%d Misses=%d", metrics.Hits, metrics.Misses)
	}
}

func TestCache_Concurrent(t *testing.T) {
	cache := New(WithMaxSize(1000))

	done := make(chan struct{}, 100)

	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			cache.Set("key", "value", time.Minute)
			cache.Get("key")
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	if cache.Size() > 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}
