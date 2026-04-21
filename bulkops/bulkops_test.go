package bulkops

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestInsert_Empty(t *testing.T) {
	var called bool
	err := Insert(context.Background(), []int{}, 100, func(batch []int) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called {
		t.Error("function should not be called for empty items")
	}
}

func TestInsert_SingleBatch(t *testing.T) {
	items := []int{1, 2, 3}
	called := 0

	err := Insert(context.Background(), items, 100, func(batch []int) error {
		called++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestInsert_MultipleBatches(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	called := 0
	batchSizes := []int{}

	err := Insert(context.Background(), items, 2, func(batch []int) error {
		called++
		batchSizes = append(batchSizes, len(batch))
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
	if len(batchSizes) != 3 {
		t.Errorf("expected 3 batch sizes, got %d", len(batchSizes))
	}
}

func TestInsert_Error(t *testing.T) {
	items := []int{1, 2, 3}
	testErr := errors.New("insert error")

	err := Insert(context.Background(), items, 100, func(batch []int) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected testErr, got %v", err)
	}
}

func TestInsert_ZeroBatchSize(t *testing.T) {
	items := []int{1, 2, 3}
	called := 0

	err := Insert(context.Background(), items, 0, func(batch []int) error {
		called++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call with default batch size, got %d", called)
	}
}

func TestInsert_NegativeBatchSize(t *testing.T) {
	items := []int{1, 2, 3}
	called := 0

	err := Insert(context.Background(), items, -1, func(batch []int) error {
		called++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call with default batch size, got %d", called)
	}
}

func TestUpdate(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	called := 0

	err := Update(context.Background(), items, 2, func(batch []string) error {
		called++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 2 {
		t.Errorf("expected 2 calls, got %d", called)
	}
}

func TestDelete(t *testing.T) {
	ids := []string{"1", "2", "3", "4", "5"}
	called := 0

	err := Delete(context.Background(), ids, 2, func(batch []string) error {
		called++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := []int{1, 2, 3}
	err := Insert(ctx, items, 1, func(batch []int) error {
		return nil
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestInBatches(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	batches := InBatches(items, 2)

	if len(batches) != 3 {
		t.Errorf("expected 3 batches, got %d", len(batches))
	}

	if len(batches[0]) != 2 {
		t.Errorf("expected batch 0 size 2, got %d", len(batches[0]))
	}

	if len(batches[2]) != 1 {
		t.Errorf("expected batch 2 size 1, got %d", len(batches[2]))
	}
}

func TestInBatches_Empty(t *testing.T) {
	items := []int{}
	batches := InBatches(items, 2)

	if batches != nil {
		t.Error("expected nil for empty items")
	}
}

func TestInBatches_ZeroBatchSize(t *testing.T) {
	items := []int{1, 2, 3}
	batches := InBatches(items, 0)

	if batches != nil {
		t.Error("expected nil for zero batch size")
	}
}

func TestResult(t *testing.T) {
	r := NewResult(100)
	r.AddProcessed(50)
	r.AddFailed(5)

	if r.Total != 100 {
		t.Errorf("expected Total 100, got %d", r.Total)
	}
	if r.Processed != 50 {
		t.Errorf("expected Processed 50, got %d", r.Processed)
	}
	if r.Failed != 5 {
		t.Errorf("expected Failed 5, got %d", r.Failed)
	}
}

func TestConcurrent(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	err := Insert(context.Background(), items, 10, func(batch []int) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 10 {
		t.Errorf("expected 10 calls, got %d", callCount)
	}
}
