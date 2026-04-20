package taskqueue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client
}

func TestEnqueue(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	queue := New(client)
	err := queue.Enqueue(context.Background(), "test", map[string]string{"msg": "hello"})
	if err != nil {
		t.Fatalf("Enqueue error = %v", err)
	}
}

func TestEnqueueWithDelay(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	queue := New(client)
	err := queue.EnqueueWithDelay(context.Background(), "test", map[string]string{"msg": "hello"}, time.Second)
	if err != nil {
		t.Fatalf("EnqueueWithDelay error = %v", err)
	}
}

func TestDequeue(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	queue := New(client)
	queue.Enqueue(context.Background(), "test", map[string]string{"msg": "hello"})

	job, err := queue.dequeue(context.Background(), "test")
	if err != nil {
		t.Fatalf("dequeue error = %v", err)
	}

	var payload map[string]string
	json.Unmarshal(job.Payload, &payload)
	if payload["msg"] != "hello" {
		t.Errorf("payload = %v, want hello", payload)
	}
}

func TestQueueStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	queue := New(client)
	queue.Enqueue(context.Background(), "test", map[string]string{"msg": "hello"})

	stats, err := queue.QueueStats(context.Background(), "test")
	if err != nil {
		t.Fatalf("QueueStats error = %v", err)
	}

	if stats.Length != 1 {
		t.Errorf("Length = %d, want 1", stats.Length)
	}
}

func TestWorkerOptions(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	queue := New(client)
	worker := queue.Worker("test",
		WithMaxRetries(5),
		WithRetryDelay(2*time.Second),
		WithPollInterval(500*time.Millisecond),
	)

	if worker.config.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", worker.config.maxRetries)
	}
	if worker.config.retryDelay != 2*time.Second {
		t.Errorf("retryDelay = %v, want 2s", worker.config.retryDelay)
	}
	if worker.config.pollInterval != 500*time.Millisecond {
		t.Errorf("pollInterval = %v, want 500ms", worker.config.pollInterval)
	}
}
