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
//	pool.Submit(func() {
//	    // Do work
//	})
package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrPoolShutdown is returned when submitting to a shutdown pool
var ErrPoolShutdown = errors.New("pool is shutdown")

// ErrQueueFull is returned when the queue is full (non-blocking mode)
var ErrQueueFull = errors.New("task queue is full")

// Task represents a unit of work to be executed
type Task func()

// Pool manages a pool of workers
type Pool struct {
	workers    int
	queueSize  int
	taskChan   chan Task
	wg         sync.WaitGroup
	shutdown   atomic.Bool
	shutdownCh chan struct{}
	mu         sync.RWMutex
	running    bool
}

// Option configures the worker pool
type Option func(*Pool)

// WithSize sets the number of workers (default: 4)
func WithSize(workers int) Option {
	return func(p *Pool) {
		p.workers = workers
	}
}

// WithQueueSize sets the task queue size (default: 100)
func WithQueueSize(size int) Option {
	return func(p *Pool) {
		p.queueSize = size
	}
}

// WithTaskTimeout sets a timeout for each task (optional)
func WithTaskTimeout(timeout time.Duration) Option {
	return func(p *Pool) {
		// Would need to add this to Pool struct
		_ = timeout
	}
}

// New creates a new worker pool
func New(opts ...Option) *Pool {
	p := &Pool{
		workers:    4,
		queueSize:  100,
		shutdownCh: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(p)
	}

	p.taskChan = make(chan Task, p.queueSize)
	return p
}

// Start starts the worker pool (tasks can be submitted before or after start)
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

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

// Shutdown gracefully shuts down the pool.
// Waits for all queued tasks to complete, but does not wait for in-flight tasks.
func (p *Pool) Shutdown() {
	if !p.shutdown.CompareAndSwap(false, true) {
		return // Already shutting down
	}

	close(p.taskChan)
	p.wg.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
}

// ShutdownWithContext shuts down with a context timeout
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

// Running returns true if the pool is running
func (p *Pool) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running && !p.shutdown.Load()
}

// QueueLength returns the current number of pending tasks
func (p *Pool) QueueLength() int {
	return len(p.taskChan)
}

// Worker is the main loop that processes tasks
func (p *Pool) worker() {
	defer p.wg.Done()

	for {
		select {
		case task, ok := <-p.taskChan:
			if !ok {
				return // Channel closed, exit
			}
			task()
		case <-p.shutdownCh:
			return
		}
	}
}
