package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupBasic(t *testing.T) {
	d := NewDedup()

	callCount := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(10 * time.Millisecond)
		return "value", nil
	}

	val, err := d.Do("key1", fn)
	assert.NoError(t, err)
	assert.Equal(t, "value", val)
	assert.Equal(t, int32(1), callCount)
}

func TestDedupConcurrent(t *testing.T) {
	d := NewDedup()

	callCount := int32(0)
	block := make(chan struct{})

	fn := func() (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		<-block
		return "value", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.Do("key1", fn)
		}()
	}

	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, int32(1), callCount)

	close(block)
	wg.Wait()

	assert.Equal(t, int32(1), callCount, "function should only be called once")
}

func TestDedupStoreCacheHit(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	_ = c.Set(ctx, "key1", "cached", time.Minute)

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	callCount := 0
	fn := func(ctx context.Context) (interface{}, error) {
		callCount++
		return "computed", nil
	}

	val, err := store.Fetch(ctx, "key1", time.Minute, fn)
	assert.NoError(t, err)
	assert.Equal(t, "cached", val)
	assert.Equal(t, 0, callCount)
}

func TestDedupStoreCacheMiss(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	callCount := 0
	fn := func(ctx context.Context) (interface{}, error) {
		callCount++
		return "computed", nil
	}

	val, err := store.Fetch(ctx, "key1", time.Minute, fn)
	assert.NoError(t, err)
	assert.Equal(t, "computed", val)
	assert.Equal(t, 1, callCount)

	val, err = store.Fetch(ctx, "key1", time.Minute, fn)
	assert.NoError(t, err)
	assert.Equal(t, "computed", val)
	assert.Equal(t, 1, callCount)
}

func TestDedupStoreConcurrent(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	actualCalls := int32(0)
	block := make(chan struct{})
	slowFn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&actualCalls, 1)
		<-block
		return "computed", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Fetch(ctx, "key1", time.Minute, slowFn)
		}()
	}

	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, int32(1), actualCalls, "only one slow function call should happen")

	close(block)
	wg.Wait()

	assert.Equal(t, int32(1), actualCalls, "only one slow function call happened total")
}

func TestDedupStoreWithDest(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	type User struct {
		Name string `json:"name"`
	}

	var result User
	fn := func(ctx context.Context) (interface{}, error) {
		return User{Name: "Alice"}, nil
	}

	err = store.FetchInto(ctx, "user:1", &result, time.Minute, fn)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", result.Name)

	var result2 User
	err = store.FetchInto(ctx, "user:1", &result2, time.Minute, fn)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", result2.Name)
}

func TestDedupCtxBasic(t *testing.T) {
	d := NewDedup()

	ctx := context.Background()

	val, err := d.DoCtx(ctx, "key1", func(ctx context.Context) (interface{}, error) {
		return "value", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestDedupStoreFnError(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	expectedErr := errors.New("database error")
	fn := func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	}

	_, err = store.Fetch(ctx, "key1", time.Minute, fn)
	assert.Equal(t, expectedErr, err)

	_, err = store.Fetch(ctx, "key1", time.Minute, fn)
	assert.Equal(t, expectedErr, err)
}

func TestDedupDifferentKeys(t *testing.T) {
	d := NewDedup()

	callCount1 := int32(0)
	callCount2 := int32(0)

	fn1 := func() (interface{}, error) {
		atomic.AddInt32(&callCount1, 1)
		time.Sleep(5 * time.Millisecond)
		return "value1", nil
	}

	fn2 := func() (interface{}, error) {
		atomic.AddInt32(&callCount2, 1)
		time.Sleep(5 * time.Millisecond)
		return "value2", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.Do("key1", fn1)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.Do("key2", fn2)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), callCount1, "key1 should only be called once")
	assert.Equal(t, int32(1), callCount2, "key2 should only be called once")
}

func TestDedupStoreSequential(t *testing.T) {
	mockRedis := newMockRedisClient(t)
	defer mockRedis.Close()

	c, err := New(mockRedis, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	get := func(ctx context.Context, key string) (interface{}, error) {
		var val interface{}
		err := c.Get(ctx, key, &val)
		return val, err
	}
	set := func(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
		return c.Set(ctx, key, val, ttl)
	}

	store := NewDedupStore(get, set)

	results := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		val, err := store.Fetch(ctx, "key1", time.Minute, func(ctx context.Context) (interface{}, error) {
			return "computed", nil
		})
		require.NoError(t, err)
		results = append(results, val.(string))
	}

	assert.Equal(t, 3, len(results))
	assert.Equal(t, "computed", results[0])
	assert.Equal(t, "computed", results[1])
	assert.Equal(t, "computed", results[2])
}
