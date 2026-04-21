// Package queue provides an in-memory job queue for asynchronous task processing.
//
// This package implements a simple in-memory queue with:
// - Multiple queues
// - Worker pool for processing
// - Retry support
// - Graceful shutdown
//
// Example:
//
//	queue := queue.New(queue.WithWorkers(4))
//
//	queue.Enqueue(ctx, queue.Job{
//	    Type: "send_email",
//	    Payload: map[string]any{"to": user.Email},
//	})
//
//	worker := queue.Worker("send_email")
//	worker.Start(ctx, func(ctx context.Context, job queue.Job) error {
//	    return sendEmail(ctx, job.Payload)
//	})
package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Job struct {
	ID         string
	Type       string
	Payload    any
	Retries    int
	MaxRetries int
	CreatedAt  time.Time
}

type Queue struct {
	queues  map[string]chan Job
	mu      sync.RWMutex
	workers map[string]*Worker
	config  Config
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type Config struct {
	BufferSize int
	Workers    int
}

type Option func(*Config)

func WithBufferSize(size int) Option {
	return func(c *Config) {
		c.BufferSize = size
	}
}

func WithWorkers(n int) Option {
	return func(c *Config) {
		c.Workers = n
	}
}

func New(opts ...Option) *Queue {
	cfg := Config{
		BufferSize: 100,
		Workers:    1,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Queue{
		queues:  make(map[string]chan Job),
		workers: make(map[string]*Worker),
		config:  cfg,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	if job.ID == "" {
		job.ID = generateID()
	}
	job.CreatedAt = time.Now()
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}

	q.mu.RLock()
	ch, ok := q.queues[job.Type]
	q.mu.RUnlock()

	if !ok {
		q.mu.Lock()
		ch, ok = q.queues[job.Type]
		if !ok {
			ch = make(chan Job, q.config.BufferSize)
			q.queues[job.Type] = ch
		}
		q.mu.Unlock()
	}

	select {
	case ch <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrQueueFull
	}
}

func (q *Queue) Dequeue(ctx context.Context, jobType string) (Job, error) {
	q.mu.RLock()
	ch, ok := q.queues[jobType]
	q.mu.RUnlock()

	if !ok {
		return Job{}, ErrQueueNotFound
	}

	select {
	case job := <-ch:
		return job, nil
	case <-ctx.Done():
		return Job{}, ctx.Err()
	}
}

func (q *Queue) Worker(jobType string) *Worker {
	q.mu.Lock()
	defer q.mu.Unlock()

	if w, ok := q.workers[jobType]; ok {
		return w
	}

	w := &Worker{
		queue:   q,
		jobType: jobType,
		ctx:     q.ctx,
	}
	q.workers[jobType] = w
	return w
}

func (q *Queue) Close() error {
	q.cancel()
	q.wg.Wait()
	return nil
}

type Worker struct {
	queue   *Queue
	jobType string
	handler func(context.Context, Job) error
	ctx     context.Context
	started bool
	mu      sync.Mutex
}

func (w *Worker) Start(ctx context.Context, handler func(context.Context, Job) error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return
	}

	w.handler = handler
	w.started = true
	w.queue.wg.Add(1)

	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer w.queue.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.queue.queues[w.jobType]:
			if err := w.handler(ctx, job); err != nil {
				w.retry(ctx, job, err)
			}
		}
	}
}

func (w *Worker) retry(ctx context.Context, job Job, err error) {
	if job.Retries >= job.MaxRetries {
		return
	}

	job.Retries++
	w.queue.Enqueue(ctx, job)
}

var (
	ErrQueueFull        = &queueError{"queue is full"}
	ErrQueueNotFound    = &queueError{"queue not found"}
	ErrWorkerNotStarted = &queueError{"worker not started"}
)

type queueError struct {
	msg string
}

func (e *queueError) Error() string {
	return e.msg
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
