package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	called := 0
	err := Do(context.Background(), func() error {
		called++
		return nil
	}, WithMaxAttempts(3))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called != 1 {
		t.Errorf("expected call count 1, got %d", called)
	}
}

func TestDo_RetryOnFailure(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, WithMaxAttempts(3))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestDo_MaxAttemptsReached(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return errors.New("permanent error")
	}, WithMaxAttempts(3), WithRetryIf(func(e error) bool { return true }))

	if err == nil {
		t.Fatal("expected error")
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cancel() // Cancel immediately

	err := Do(ctx, func() error {
		return errors.New("error")
	}, WithMaxAttempts(10))

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDo_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Do(ctx, func() error {
		return errors.New("error")
	}, WithBaseDelay(100*time.Millisecond), WithMaxAttempts(10))

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("took too long: %v", elapsed)
	}
}

func TestDo_WithConstantBackoff(t *testing.T) {
	callCount := 0
	start := time.Now()

	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}
		return nil
	}, WithMaxAttempts(3), WithBackoff(Constant), WithBaseDelay(50*time.Millisecond))

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Constant: 50ms * 2 = ~100ms
	if elapsed < 50*time.Millisecond {
		t.Errorf("too fast: %v", elapsed)
	}
}

func TestDo_WithLinearBackoff(t *testing.T) {
	callCount := 0

	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}
		return nil
	}, WithMaxAttempts(3), WithBackoff(Linear), WithBaseDelay(50*time.Millisecond))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestDo_WithExponentialBackoff(t *testing.T) {
	callCount := 0

	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}
		return nil
	}, WithMaxAttempts(3), WithBackoff(Exponential), WithBaseDelay(50*time.Millisecond), WithJitter())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestDo_DoesNotRetryOnNilError(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return nil // success on first call
	}, WithMaxAttempts(3))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected call count 1, got %d", callCount)
	}
}

func TestDoWithResult(t *testing.T) {
	result, err := DoWithResult(context.Background(), func() (int, error) {
		return 42, nil
	}, WithMaxAttempts(1))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestDoWithResult_WithError(t *testing.T) {
	result, err := DoWithResult[int](context.Background(), func() (int, error) {
		return 0, errors.New("error")
	}, WithMaxAttempts(1))

	if err == nil {
		t.Fatal("expected error")
	}

	if result != 0 {
		t.Errorf("expected zero value")
	}
}

func TestDefaultRetryIf(t *testing.T) {
	if DefaultRetryIf(nil) {
		t.Error("should not retry on nil error")
	}

	if DefaultRetryIf(context.Canceled) {
		t.Error("should not retry on context.Canceled")
	}

	if DefaultRetryIf(context.DeadlineExceeded) {
		t.Error("should not retry on context.DeadlineExceeded")
	}

	if !DefaultRetryIf(errors.New("error")) {
		t.Error("should retry on regular error")
	}
}

func TestRetry_Shorthand(t *testing.T) {
	callCount := 0
	err := Retry(context.Background(), 3, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestWithRetryIf_CustomFilter(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		err := errors.New("specific error")
		return err
	}, WithMaxAttempts(3), WithRetryIf(func(e error) bool {
		return e.Error() == "specific error"
	}))

	if err == nil {
		t.Fatal("expected error")
	}

	if callCount != 3 {
		t.Errorf("expected call count 3, got %d", callCount)
	}
}

func TestWithMaxDelay(t *testing.T) {
	callCount := 0

	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 3 {
			return errors.New("error")
		}
		return nil
	}, WithMaxAttempts(3), WithBaseDelay(10*time.Second), WithMaxDelay(50*time.Millisecond), WithBackoff(Constant))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should cap delay at 50ms, not use 10s
}
