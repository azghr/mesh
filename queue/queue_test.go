package queue

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	q := New()

	if q == nil {
		t.Error("expected queue, got nil")
	}

	if q.queues == nil {
		t.Error("expected queues map, got nil")
	}
}

func TestNewWithOptions(t *testing.T) {
	q := New(
		WithBufferSize(50),
		WithWorkers(4),
	)

	if q.config.BufferSize != 50 {
		t.Errorf("expected buffer size 50, got %d", q.config.BufferSize)
	}

	if q.config.Workers != 4 {
		t.Errorf("expected workers 4, got %d", q.config.Workers)
	}
}

func TestEnqueue(t *testing.T) {
	q := New()

	ctx := context.Background()
	job := Job{
		Type:    "test",
		Payload: map[string]any{"msg": "hello"},
	}

	err := q.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnqueue_WithID(t *testing.T) {
	q := New()

	ctx := context.Background()
	job := Job{
		ID:      "job-123",
		Type:    "test",
		Payload: "data",
	}

	err := q.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	dequeued, err := q.Dequeue(ctx, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if dequeued.ID != "job-123" {
		t.Errorf("expected job ID job-123, got %s", dequeued.ID)
	}
}

func TestDequeue(t *testing.T) {
	q := New()

	ctx := context.Background()

	// Dequeue from non-existent queue
	_, err := q.Dequeue(ctx, "nonexistent")
	if err != ErrQueueNotFound {
		t.Errorf("expected ErrQueueNotFound, got %v", err)
	}

	// Enqueue then dequeue
	q.Enqueue(ctx, Job{Type: "test", Payload: "data"})

	job, err := q.Dequeue(ctx, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if job.Type != "test" {
		t.Errorf("expected job type test, got %s", job.Type)
	}
}

func TestJobFields(t *testing.T) {
	q := New()

	ctx := context.Background()
	now := time.Now()

	job := Job{
		Type:       "email",
		Payload:    map[string]any{"to": "test@example.com"},
		MaxRetries: 5,
	}

	q.Enqueue(ctx, job)

	dequeued, _ := q.Dequeue(ctx, "email")

	if dequeued.Type != "email" {
		t.Errorf("expected type email, got %s", dequeued.Type)
	}

	if dequeued.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", dequeued.MaxRetries)
	}

	if dequeued.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Check CreatedAt is close to now
	if dequeued.CreatedAt.Sub(now) > time.Second {
		t.Errorf("expected CreatedAt close to now, got %v", dequeued.CreatedAt)
	}
}

func TestWorker(t *testing.T) {
	q := New()

	worker := q.Worker("email")
	if worker == nil {
		t.Error("expected worker, got nil")
	}

	// Same worker for same type
	worker2 := q.Worker("email")
	if worker != worker2 {
		t.Error("expected same worker instance")
	}

	// Different worker for different type
	worker3 := q.Worker("sms")
	if worker == worker3 {
		t.Error("expected different worker for different type")
	}
}

func TestClose(t *testing.T) {
	q := New()

	// Close should not panic
	err := q.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	q := New(WithBufferSize(1))

	ctx := context.Background()

	// Fill the queue
	q.Enqueue(ctx, Job{Type: "test", Payload: "1"})
	q.Enqueue(ctx, Job{Type: "test", Payload: "2"})

	// This should fail with ErrQueueFull
	err := q.Enqueue(ctx, Job{Type: "test", Payload: "3"})
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestMultipleQueues(t *testing.T) {
	q := New()

	ctx := context.Background()

	q.Enqueue(ctx, Job{Type: "email", Payload: "email1"})
	q.Enqueue(ctx, Job{Type: "sms", Payload: "sms1"})

	emailJob, _ := q.Dequeue(ctx, "email")
	smsJob, _ := q.Dequeue(ctx, "sms")

	if emailJob.Type != "email" {
		t.Errorf("expected email job, got %s", emailJob.Type)
	}

	if smsJob.Type != "sms" {
		t.Errorf("expected sms job, got %s", smsJob.Type)
	}
}

func TestContextCancellation(t *testing.T) {
	q := New()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should return context.Canceled
	err := q.Enqueue(ctx, Job{Type: "test", Payload: "data"})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
