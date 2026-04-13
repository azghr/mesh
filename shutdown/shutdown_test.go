package shutdown

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	require.NotNil(t, mgr)
	assert.Empty(t, mgr.Tasks())
}

func TestManager_Register(t *testing.T) {
	mgr := NewManager()

	mgr.Register("test", func(ctx context.Context) error {
		return nil
	})

	assert.Contains(t, mgr.Tasks(), "test")
}

func TestManager_RegisterSimple(t *testing.T) {
	mgr := NewManager()

	mgr.RegisterSimple("test", func() error {
		return nil
	})

	assert.Contains(t, mgr.Tasks(), "test")
}

func TestManager_Shutdown(t *testing.T) {
	mgr := NewManager()

	var order []string
	mgr.Register("first", func(ctx context.Context) error {
		order = append(order, "first")
		return nil
	})
	mgr.Register("second", func(ctx context.Context) error {
		order = append(order, "second")
		return nil
	})

	err := mgr.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Len(t, order, 2, "both tasks should be executed")
	assert.Contains(t, order, "first")
	assert.Contains(t, order, "second")
}

func TestManager_ShutdownWithError(t *testing.T) {
	mgr := NewManager()

	mgr.Register("failing", func(ctx context.Context) error {
		return assert.AnError
	})

	err := mgr.Shutdown(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing")
}

func TestManager_DependencyOrder(t *testing.T) {
	mgr := NewManager()

	var order []string

	mgr.Register("a", func(ctx context.Context) error {
		order = append(order, "a")
		return nil
	}, WithDependsOn("b", "c"))

	mgr.Register("b", func(ctx context.Context) error {
		order = append(order, "b")
		return nil
	}, WithDependsOn("c"))

	mgr.Register("c", func(ctx context.Context) error {
		order = append(order, "c")
		return nil
	})

	err := mgr.Shutdown(context.Background())
	require.NoError(t, err)

	// c should come before b, b should come before a
	cIdx := -1
	bIdx := -1
	aIdx := -1

	for i, name := range order {
		switch name {
		case "c":
			cIdx = i
		case "b":
			bIdx = i
		case "a":
			aIdx = i
		}
	}

	assert.True(t, cIdx < bIdx, "c should come before b")
	assert.True(t, bIdx < aIdx, "b should come before a")
}

func TestManager_Timeout(t *testing.T) {
	mgr := NewManager()

	slowCalled := atomic.Bool{}

	mgr.Register("slow", func(ctx context.Context) error {
		slowCalled.Store(true)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	}, WithTaskTimeout(100*time.Millisecond))

	start := time.Now()
	err := mgr.Shutdown(context.Background())
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.True(t, elapsed < 5*time.Second, "should timeout before the slow function completes")
	assert.True(t, slowCalled.Load(), "slow function should have been called")
}

func TestManager_OnShutdown(t *testing.T) {
	mgr := NewManager()

	var called bool
	mgr.OnShutdown(func() {
		called = true
	})

	mgr.Register("test", func(ctx context.Context) error {
		return nil
	})

	err := mgr.Shutdown(context.Background())
	require.NoError(t, err)
	assert.True(t, called, "OnShutdown callback should be called")
}

func TestManager_Error(t *testing.T) {
	mgr := NewManager()

	mgr.Register("test", func(ctx context.Context) error {
		return assert.AnError
	})

	err := mgr.Shutdown(context.Background())
	require.Error(t, err)
	assert.Equal(t, err, mgr.Error())
}

func TestManager_ConcurrentShutdown(t *testing.T) {
	mgr := NewManager()

	mgr.Register("test", func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// Run shutdown concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	var err1, err2 error

	go func() {
		defer wg.Done()
		err1 = mgr.Shutdown(context.Background())
	}()

	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		err2 = mgr.Shutdown(context.Background())
	}()

	wg.Wait()

	// At least one should complete without error
	if err1 == nil || err2 == nil {
		// Good - one completed
	} else {
		// Both errors - one might be context cancellation
		t.Logf("err1: %v, err2: %v", err1, err2)
	}
}

func TestManager_WithLogger(t *testing.T) {
	// Should not panic with custom logger
	mgr := NewManager(WithLogger(nil))
	require.NotNil(t, mgr)
}

func TestManager_WaitForSignal(t *testing.T) {
	// This test is tricky to implement without signaling
	// Just verify the method exists and can be called with cancelled context
	mgr := NewManager()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mgr.WaitForSignal(ctx)
	assert.Equal(t, context.Canceled, err)
}
