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
		pool.Submit(context.Background(), func(ctx context.Context) {
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

	pool.SubmitNonBlocking(func(ctx context.Context) { time.Sleep(100 * time.Millisecond) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pool.Submit(ctx, func(ctx context.Context) {})
	assert.Equal(t, context.Canceled, err)

	pool.Shutdown()
}

func TestPool_SubmitNonBlocking(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(1))
	pool.Start()

	err := pool.SubmitNonBlocking(func(ctx context.Context) {})
	require.NoError(t, err)

	err = pool.SubmitNonBlocking(func(ctx context.Context) {})
	assert.Equal(t, ErrQueueFull, err)

	pool.Shutdown()
}

func TestPool_Shutdown(t *testing.T) {
	pool := New(WithSize(2), WithQueueSize(10))
	pool.Start()

	pool.SubmitNonBlocking(func(ctx context.Context) { time.Sleep(10 * time.Millisecond) })
	pool.SubmitNonBlocking(func(ctx context.Context) { time.Sleep(10 * time.Millisecond) })

	pool.Shutdown()
	assert.False(t, pool.Running())
}

func TestPool_SubmitAfterShutdown(t *testing.T) {
	pool := New()
	pool.Shutdown()

	err := pool.Submit(context.Background(), func(ctx context.Context) {})
	assert.Equal(t, ErrPoolShutdown, err)
}

func TestPool_ShutdownWithContext(t *testing.T) {
	pool := New(WithSize(2), WithQueueSize(100))
	pool.Start()

	for i := 0; i < 50; i++ {
		pool.SubmitNonBlocking(func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond)
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pool.ShutdownWithContext(ctx)
	assert.True(t, err == nil || err == context.DeadlineExceeded)
}

func TestPool_QueueLength(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(5))
	pool.Start()

	pool.SubmitNonBlocking(func(ctx context.Context) { time.Sleep(100 * time.Millisecond) })
	pool.SubmitNonBlocking(func(ctx context.Context) {})

	time.Sleep(10 * time.Millisecond)
	assert.True(t, pool.QueueLength() >= 1)

	pool.Shutdown()
}

func TestPool_Concurrent(t *testing.T) {
	pool := New(WithSize(4), WithQueueSize(1000))
	pool.Start()

	var wg sync.WaitGroup
	var counter atomic.Int64

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pool.Submit(context.Background(), func(ctx context.Context) {
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

	select {
	case pool.taskChan <- func(ctx context.Context) {}:
	default:
	}

	pool.Shutdown()
}

func TestPool_WithTaskTimeout(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(10), WithTaskTimeout(50*time.Millisecond))
	pool.Start()

	var timedOut atomic.Bool

	_ = pool.Submit(context.Background(), func(ctx context.Context) {
		select {
		case <-ctx.Done():
			timedOut.Store(true)
		case <-time.After(100 * time.Millisecond):
		}
	})

	time.Sleep(100 * time.Millisecond)
	pool.Shutdown()

	assert.True(t, timedOut.Load())
}

func TestPool_SubmitWithTimeout(t *testing.T) {
	pool := New(WithSize(1), WithQueueSize(10))
	pool.Start()

	err := pool.SubmitWithTimeout(context.Background(), func(ctx context.Context) {
		time.Sleep(200 * time.Millisecond)
	}, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	pool.Shutdown()

	assert.NoError(t, err)
}
