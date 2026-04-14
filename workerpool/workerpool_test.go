package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	pool := New()
	require.NotNil(t, pool)
	assert.False(t, pool.Running())
}

func TestNewWithOptions(t *testing.T) {
	pool := New(WithSize(8), WithQueueSize(200))
	require.NotNil(t, pool)
}

func TestPool_Start(t *testing.T) {
	pool := New(WithSize(2))
	pool.Start()
	require.True(t, pool.Running())
	pool.Shutdown()
}

func TestPool_Submit(t *testing.T) {
	pool := New(WithSize(2), WithQueueSize(10))
	pool.Start()

	var counter atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.Submit(context.Background(), func() {
			defer wg.Done()
			counter.Add(1)
		})
	}

	wg.Wait()
	pool.Shutdown()

	assert.Equal(t, int32(10), counter.Load())
}

func TestPool_SubmitWithCancel(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(1))
	pool.Start()

	// Fill the queue
	pool.SubmitNonBlocking(func() { time.Sleep(100 * time.Millisecond) })

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pool.Submit(ctx, func() {})
	assert.Equal(t, context.Canceled, err)

	pool.Shutdown()
}

func TestPool_SubmitNonBlocking(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(1))
	pool.Start()

	// Should succeed
	err := pool.SubmitNonBlocking(func() {})
	require.NoError(t, err)

	// Should fail - queue full
	err = pool.SubmitNonBlocking(func() {})
	assert.Equal(t, ErrQueueFull, err)

	pool.Shutdown()
}

func TestPool_Shutdown(t *testing.T) {
	pool := New(WithSize(2), WithQueueSize(10))
	pool.Start()

	// Submit some tasks
	pool.SubmitNonBlocking(func() { time.Sleep(10 * time.Millisecond) })
	pool.SubmitNonBlocking(func() { time.Sleep(10 * time.Millisecond) })

	// Shutdown should complete without waiting for in-flight tasks
	pool.Shutdown()
	assert.False(t, pool.Running())
}

func TestPool_SubmitAfterShutdown(t *testing.T) {
	pool := New()
	pool.Shutdown()

	err := pool.Submit(context.Background(), func() {})
	assert.Equal(t, ErrPoolShutdown, err)
}

func TestPool_ShutdownWithContext(t *testing.T) {
	pool := New(WithSize(2), WithQueueSize(100))

	// Submit some tasks that take time
	pool.Start()
	for i := 0; i < 50; i++ {
		pool.SubmitNonBlocking(func() {
			time.Sleep(10 * time.Millisecond)
		})
	}

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pool.ShutdownWithContext(ctx)
	// May timeout if tasks take longer
	assert.True(t, err == nil || err == context.DeadlineExceeded)
}

func TestPool_QueueLength(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(5))
	pool.Start()

	// Add pending tasks
	pool.SubmitNonBlocking(func() { time.Sleep(100 * time.Millisecond) })
	pool.SubmitNonBlocking(func() {})

	time.Sleep(10 * time.Millisecond) // Let one task start
	assert.True(t, pool.QueueLength() >= 1)

	pool.Shutdown()
}

func TestPool_Concurrent(t *testing.T) {
	pool := New(WithSize(4), WithQueueSize(1000))
	pool.Start()

	var wg sync.WaitGroup
	var counter atomic.Int64

	// Multiple goroutines submitting tasks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pool.Submit(context.Background(), func() {
					counter.Add(1)
				})
			}
		}()
	}

	wg.Wait()
	pool.Shutdown()

	assert.Equal(t, int64(1000), counter.Load())
}

func TestPool_ZeroWorkers(t *testing.T) {
	pool := New(WithSize(0))
	pool.Start()

	// Should not panic - just no workers
	select {
	case pool.taskChan <- func() {}:
	default:
	}

	pool.Shutdown()
}
