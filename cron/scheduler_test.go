package cron

import (
	"context"
	"testing"
	"time"
)

func TestAddJob(t *testing.T) {
	scheduler := New()
	err := scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}
}

func TestAddJob_InvalidSchedule(t *testing.T) {
	scheduler := New()
	err := scheduler.AddJob("test", "invalid", func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}

func TestAddJob_Duplicate(t *testing.T) {
	scheduler := New()
	scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	})
	err := scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for duplicate job")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	scheduler := New()
	scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	})
	scheduler.Start()
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()
}

func TestScheduler_Trigger(t *testing.T) {
	scheduler := New()
	triggered := false
	scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		triggered = true
		return nil
	})
	scheduler.Start()
	scheduler.Trigger(context.Background(), "test")
	scheduler.Stop()

	if !triggered {
		t.Error("job was not triggered")
	}
}

func TestScheduler_Trigger_NotFound(t *testing.T) {
	scheduler := New()
	err := scheduler.Trigger(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestScheduler_ListJobs(t *testing.T) {
	scheduler := New()
	scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	})
	jobs := scheduler.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestWithOverlap(t *testing.T) {
	scheduler := New()
	scheduler.AddJob("test", "0 0 * * *", func(ctx context.Context) error {
		return nil
	}, WithOverlap(false))

	jobs := scheduler.ListJobs()
	if len(jobs) != 1 || jobs[0].AllowOverlap {
		t.Error("overlap not disabled")
	}
}

func TestParseField(t *testing.T) {
	tests := []struct {
		expr   string
		valid  []int
		expect []int
	}{
		{"*", []int{0, 59}, []int{-1}},
		{"0,30,59", []int{0, 59}, []int{0, 30, 59}},
		{"1-5", []int{1, 31}, []int{1, 2, 3, 4, 5}},
		{"*/5", []int{0, 59}, []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55}},
	}

	for _, tt := range tests {
		result := parseField(tt.expr, tt.valid)
		if len(result) != len(tt.expect) {
			t.Errorf("parseField(%s) = %v, want %v", tt.expr, result, tt.expect)
		}
	}
}

func BenchmarkParseField(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parseField("*/5", []int{0, 59})
	}
}
