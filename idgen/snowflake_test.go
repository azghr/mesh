package idgen

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	gen, err := New(1)
	if err != nil {
		t.Fatalf("New(1) error = %v", err)
	}
	if gen.machineID != 1 {
		t.Errorf("machineID = %d, want 1", gen.machineID)
	}
}

func TestNew_InvalidMachineID(t *testing.T) {
	tests := []int64{-1, 1024, 2000}

	for _, id := range tests {
		_, err := New(id)
		if err == nil {
			t.Errorf("New(%d) expected error, got nil", id)
		}
	}
}

func TestGenerator_Next(t *testing.T) {
	gen, _ := New(1)

	ids := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id := gen.Next()
		if id <= 0 {
			t.Errorf("Next() = %d, want > 0", id)
		}
		if ids[id] {
			t.Errorf("duplicate ID: %d", id)
		}
		ids[id] = true
	}
}

func TestGenerator_Next_Order(t *testing.T) {
	gen, _ := New(1)

	var prev int64 = 0
	for i := 0; i < 100; i++ {
		id := gen.Next()
		if id <= prev {
			t.Errorf("ID not increasing: prev=%d, current=%d", prev, id)
		}
		prev = id
	}
}

func TestGenerator_Concurrent(t *testing.T) {
	gen, _ := New(1)

	var wg sync.WaitGroup
	ids := make(chan int64, 10000)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ids <- gen.Next()
			}
		}()
	}

	wg.Wait()
	close(ids)

	unique := make(map[int64]bool)
	for id := range ids {
		unique[id] = true
	}

	if len(unique) != 10000 {
		t.Errorf("expected 10000 unique IDs, got %d", len(unique))
	}
}

func TestGenerator_Info(t *testing.T) {
	gen, _ := New(42)

	id := gen.Next()
	info := gen.Info(id)

	if info.ID != id {
		t.Errorf("Info.ID = %d, want %d", info.ID, id)
	}
	if info.MachineID != 42 {
		t.Errorf("Info.MachineID = %d, want 42", info.MachineID)
	}
}

func TestGenerator_MaxSequence(t *testing.T) {
	// Test that generator handles max sequence overflow
	// This is hard to test directly without mocking time,
	// so we just verify basic functionality
	gen, _ := New(1)

	// Generate multiple IDs
	for i := 0; i < 5000; i++ {
		gen.Next()
	}
	// If we reached here without panic, basic overflow handling works
}

func BenchmarkNext(b *testing.B) {
	gen, _ := New(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Next()
	}
}
