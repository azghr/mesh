package bloom

import (
	"context"
	"testing"

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

func TestNew(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	if bf == nil {
		t.Fatal("New returned nil")
	}
	if bf.name != "test" {
		t.Errorf("name = %s, want test", bf.name)
	}
}

func TestAdd(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	err := bf.Add(context.Background(), "item1")
	if err != nil {
		t.Fatalf("Add error = %v", err)
	}
}

func TestExists(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	bf.Add(context.Background(), "item1")

	exists, err := bf.Exists(context.Background(), "item1")
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}
	if !exists {
		t.Error("item1 should exist")
	}
}

func TestExists_NotFound(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	bf.Add(context.Background(), "item1")

	exists, err := bf.Exists(context.Background(), "item2")
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}
	if exists {
		t.Error("item2 should not exist")
	}
}

func TestAddMany(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	items := []string{"item1", "item2", "item3"}

	err := bf.AddMany(context.Background(), items)
	if err != nil {
		t.Fatalf("AddMany error = %v", err)
	}
}

func TestExistsMany(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	bf.AddMany(context.Background(), []string{"item1", "item2"})

	result, err := bf.ExistsMany(context.Background(), []string{"item1", "item2", "item3"})
	if err != nil {
		t.Fatalf("ExistsMany error = %v", err)
	}

	if !result["item1"] {
		t.Error("item1 should exist")
	}
	if !result["item2"] {
		t.Error("item2 should exist")
	}
	if result["item3"] {
		t.Error("item3 should not exist")
	}
}

func TestReset(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	bf.Add(context.Background(), "item1")
	bf.Reset(context.Background())

	exists, _ := bf.Exists(context.Background(), "item1")
	if exists {
		t.Error("item should not exist after reset")
	}
}

func TestStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	bf := New(client, "test", 1000, 0.01)
	bf.AddMany(context.Background(), []string{"item1", "item2"})

	stats, err := bf.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats error = %v", err)
	}

	if stats.Name != "test" {
		t.Errorf("name = %s, want test", stats.Name)
	}
	if stats.ItemCount == 0 {
		t.Error("item count should be > 0")
	}
}

func TestOptimalNumBits(t *testing.T) {
	tests := []struct {
		n        int
		p        float64
		expected uint64
	}{
		{100, 0.01, 958},
		{1000, 0.01, 9585},
		{10000, 0.01, 95851},
		{100, 0.1, 479},
	}

	for _, tt := range tests {
		result := optimalNumBits(tt.n, tt.p)
		if result < tt.expected {
			t.Errorf("optimalNumBits(%d, %f) = %d, want >= %d", tt.n, tt.p, result, tt.expected)
		}
	}
}

func TestOptimalNumHashes(t *testing.T) {
	tests := []struct {
		n        float64
		m        float64
		expected uint64
	}{
		{100, 958, 7},
		{1000, 9585, 7},
		{10000, 95851, 7},
	}

	for _, tt := range tests {
		result := optimalNumHashes(tt.n, tt.m)
		if result < tt.expected {
			t.Errorf("optimalNumHashes(%f, %f) = %d, want >= %d", tt.n, tt.m, result, tt.expected)
		}
	}
}
