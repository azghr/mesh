// Package workerpool provides a simple goroutine pool with backpressure.
//
// This package implements a worker pool pattern for CPU/IO-bound tasks.
// Tasks are submitted to a channel and distributed to available workers.
//
// Example:
//
//	pool := workerpool.New(workerpool.WithSize(4), workerpool.WithQueueSize(100))
//	defer pool.Shutdown()
//
//	// Submit work
//	pool.Submit(ctx, func(ctx context.Context) {
//	    // Do work
//	})
//
// Task timeouts are supported via SubmitWithTimeout or WithTaskTimeout:
//
//	// Pool-level timeout (all tasks)
//	pool := workerpool.New(workerpool.WithTaskTimeout(30 * time.Second))
//
//	// Per-task timeout
//	err := pool.SubmitWithTimeout(ctx, task, 10*time.Second)
package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPoolShutdown = errors.New("pool is shutdown")
	ErrQueueFull    = errors.New("task queue is full")
	ErrTaskTimeout  = errors.New("task timed out")
)

// Task represents a unit of work to be executed.
// The context supports cancellation and timeout detection.
type Task func(ctx context.Context)

// Pool manages a pool of workers.
type Pool struct {
	workers     int
	queueSize   int
	taskTimeout time.Duration
	taskChan    chan Task
	wg          sync.WaitGroup
	shutdown    atomic.Bool
	mu          sync.RWMutex
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// Option configures the worker pool.
type Option func(*Pool)

// WithSize sets the number of workers (default: 4).
func WithSize(workers int) Option {
	return func(p *Pool) {
		p.workers = workers
	}
}

// WithQueueSize sets the task queue size (default: 100).
func WithQueueSize(size int) Option {
	return func(p *Pool) {
		p.queueSize = size
	}
}

// WithTaskTimeout sets a default timeout for each task.
// When set, tasks that exceed this duration will be cancelled.
func WithTaskTimeout(timeout time.Duration) Option {
	return func(p *Pool) {
		p.taskTimeout = timeout
	}
}

// New creates a new worker pool.
func New(opts ...Option) *Pool {
	p := &Pool{
		workers:   4,
		queueSize: 100,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.taskChan = make(chan Task, p.queueSize)
	return p
}

// Start starts the worker pool (tasks can be submitted before or after start).
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.running = true

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Submit adds a task to the pool.
// Blocks if the queue is full until a slot is available or context is cancelled.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if p.shutdown.Load() {
		return ErrPoolShutdown
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.taskChan <- task:
		return nil
	}
}

// SubmitNonBlocking adds a task to the pool without blocking.
// Returns ErrQueueFull if the queue is full.
func (p *Pool) SubmitNonBlocking(task Task) error {
	if p.shutdown.Load() {
		return ErrPoolShutdown
	}

	select {
	case p.taskChan <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// SubmitWithTimeout submits a task with a specific timeout.
// The timeout applies to task execution, not queueing.
func (p *Pool) SubmitWithTimeout(ctx context.Context, task Task, timeout time.Duration) error {
	if p.shutdown.Load() {
		return ErrPoolShutdown
	}

	wrapper := func(taskCtx context.Context) {
		done := make(chan struct{})
		go func() {
			task(taskCtx)
			close(done)
		}()

		select {
		case <-taskCtx.Done():
		case <-done:
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.taskChan <- wrapper:
		go func() {
			taskCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			<-taskCtx.Done()
		}()
		return nil
	}
}

// SetTaskTimeout updates the default task timeout at runtime.
func (p *Pool) SetTaskTimeout(timeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskTimeout = timeout
}

// Shutdown gracefully shuts down the pool.
// Waits for all queued tasks to complete, but does not wait for in-flight tasks.
func (p *Pool) Shutdown() {
	if !p.shutdown.CompareAndSwap(false, true) {
		return
	}

	if p.cancel != nil {
		p.cancel()
	}
	close(p.taskChan)
	p.wg.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
}

// ShutdownWithContext shuts down with a context timeout.
func (p *Pool) ShutdownWithContext(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		p.Shutdown()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Running returns true if the pool is running.
func (p *Pool) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running && !p.shutdown.Load()
}

// QueueLength returns the current number of pending tasks.
func (p *Pool) QueueLength() int {
	return len(p.taskChan)
}

// worker is the main loop that processes tasks.
func (p *Pool) worker() {
	defer p.wg.Done()

	for {
		select {
		case task, ok := <-p.taskChan:
			if !ok {
				return
			}
			if p.taskTimeout > 0 {
				taskCtx, cancel := context.WithTimeout(p.ctx, p.taskTimeout)
				task(taskCtx)
				cancel()
			} else {
				task(p.ctx)
			}
		case <-p.ctx.Done():
			return
		}
	}
}
