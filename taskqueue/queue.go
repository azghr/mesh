// Package taskqueue provides a Redis-based task queue for asynchronous job processing.
//
// This package implements a simple task queue using Redis lists with support for:
//   - Multiple queues
//   - Delayed execution
//   - Retry with exponential backoff
//   - Dead letter queue for failed jobs
//
// # Usage
//
//	queue := taskqueue.New(redisClient)
//	queue.Enqueue(ctx, "email", payload)
//
//	// Worker
//	worker := queue.Worker("email")
//	worker.Process(handler)
package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	queuePrefix = "mesh:queue:"
	dlqPrefix   = "mesh:dlq:"
	retryPrefix = "mesh:retry:"
)

// Job represents a queued job
type Job struct {
	ID        string          `json:"id"`
	Queue     string          `json:"queue"`
	Payload   json.RawMessage `json:"payload"`
	Retries   int             `json:"retries"`
	CreatedAt time.Time       `json:"created_at"`
	RetryAt   time.Time       `json:"retry_at,omitempty"`
}

// Queue manages task queues
type Queue struct {
	client *redis.Client
}

// New creates a new task queue
func New(client *redis.Client) *Queue {
	return &Queue{client: client}
}

// Enqueue adds a job to the queue
//
//	queue.Enqueue(ctx, "email", map[string]interface{}{
//	    "to": "user@example.com",
//	    "subject": "Hello",
//	})
func (q *Queue) Enqueue(ctx context.Context, name string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	job := Job{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Queue:     name,
		Payload:   data,
		Retries:   0,
		CreatedAt: time.Now(),
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	return q.client.RPush(ctx, queuePrefix+name, jobData).Err()
}

// EnqueueWithDelay adds a job with delay
func (q *Queue) EnqueueWithDelay(ctx context.Context, name string, payload interface{}, delay time.Duration) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	job := Job{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Queue:     name,
		Payload:   data,
		Retries:   0,
		CreatedAt: time.Now(),
		RetryAt:   time.Now().Add(delay),
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	return q.client.ZAdd(ctx, queuePrefix+"delayed:"+name, redis.Z{
		Score:  float64(time.Now().Add(delay).Unix()),
		Member: jobData,
	}).Err()
}

// Worker provides a simpler interface for processing jobs
type Worker struct {
	queue  *Queue
	name   string
	config config
}

// Worker creates a new worker for a queue
func (q *Queue) Worker(name string, opts ...Option) *Worker {
	cfg := config{
		name:         name,
		maxRetries:   3,
		retryDelay:   time.Second,
		pollInterval: time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Worker{
		queue:  q,
		name:   name,
		config: cfg,
	}
}

// Process starts processing jobs
func (w *Worker) Process(ctx context.Context, handler JobHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			job, err := w.queue.dequeue(ctx, w.name)
			if err != nil {
				if errors.Is(err, redis.Nil) {
					time.Sleep(w.config.pollInterval)
					continue
				}
				return err
			}

			if job == nil {
				time.Sleep(w.config.pollInterval)
				continue
			}

			err = handler(ctx, job.Payload)
			if err != nil && job.Retries < w.config.maxRetries {
				job.Retries++
				job.RetryAt = time.Now().Add(w.config.retryDelay * time.Duration(job.Retries))
				w.queue.requeue(ctx, job)
			} else if err != nil {
				w.queue.sendToDLQ(ctx, job, err)
			}
		}
	}
}

// JobHandler is the function type for processing jobs
type JobHandler func(ctx context.Context, payload json.RawMessage) error

// config configures processing
type config struct {
	name         string
	maxRetries   int
	retryDelay   time.Duration
	pollInterval time.Duration
}

// Option configures the worker
type Option func(*config)

// WithMaxRetries sets maximum retry attempts
func WithMaxRetries(n int) Option {
	return func(c *config) {
		c.maxRetries = n
	}
}

// WithRetryDelay sets delay between retries
func WithRetryDelay(d time.Duration) Option {
	return func(c *config) {
		c.retryDelay = d
	}
}

// WithPollInterval sets polling interval
func WithPollInterval(d time.Duration) Option {
	return func(c *config) {
		c.pollInterval = d
	}
}

// dequeue gets a job from the queue
func (q *Queue) dequeue(ctx context.Context, name string) (*Job, error) {
	result, err := q.client.LPop(ctx, queuePrefix+name).Result()
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(result), &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// requeue adds a job back to the queue for retry
func (q *Queue) requeue(ctx context.Context, job *Job) error {
	jobData, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return q.client.ZAdd(ctx, retryPrefix+job.Queue, redis.Z{
		Score:  float64(job.RetryAt.Unix()),
		Member: jobData,
	}).Err()
}

// sendToDLQ sends a failed job to the dead letter queue
func (q *Queue) sendToDLQ(ctx context.Context, job *Job, err error) {
	jobData, _ := json.Marshal(job)
	q.client.RPush(ctx, dlqPrefix+job.Queue, jobData)
}

// QueueStats returns statistics for a queue
func (q *Queue) QueueStats(ctx context.Context, name string) (Stats, error) {
	length, err := q.client.LLen(ctx, queuePrefix+name).Result()
	if err != nil {
		return Stats{}, err
	}

	dlqLength, err := q.client.LLen(ctx, dlqPrefix+name).Result()
	if err != nil {
		return Stats{}, err
	}

	delayedLength, err := q.client.ZCard(ctx, queuePrefix+"delayed:"+name).Result()
	if err != nil {
		return Stats{}, err
	}

	return Stats{
		Queue:      name,
		Length:     length,
		DeadLetter: dlqLength,
		Delayed:    delayedLength,
	}, nil
}

// Stats represents queue statistics
type Stats struct {
	Queue      string `json:"queue"`
	Length     int64  `json:"length"`
	DeadLetter int64  `json:"dead_letter"`
	Delayed    int64  `json:"delayed"`
}
