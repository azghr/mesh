package discovery

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

	sd := New(Config{
		Redis: client,
		TTL:   30,
	})
	if sd == nil {
		t.Fatal("New returned nil")
	}
}

func TestRegister(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	err := sd.Register(context.Background(), "test-service", "localhost:8080", nil)
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}
}

func TestRegisterAndDiscover(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	err := sd.Register(context.Background(), "test-service", "localhost:8080", map[string]string{
		"version": "1.0.0",
	})
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}

	services, err := sd.Discover(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}

	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestDiscover_NotFound(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	_, err := sd.Discover(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeregister(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	sd.Register(context.Background(), "test-service", "localhost:8080", nil)

	err := sd.Deregister(context.Background(), "test-service", "localhost:8080")
	if err != nil {
		t.Fatalf("Deregister error = %v", err)
	}

	_, err = sd.Discover(context.Background(), "test-service")
	if err == nil {
		t.Fatal("expected error after deregister")
	}
}

func TestListAll(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	sd.Register(context.Background(), "service-a", "localhost:8080", nil)
	sd.Register(context.Background(), "service-b", "localhost:8081", nil)

	all, err := sd.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll error = %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 services, got %d", len(all))
	}
}

func TestRegisterHealthCheck(t *testing.T) {
	sd := New(Config{})
	sd.RegisterHealthCheck("test-service", func(ctx context.Context, addr string) error {
		return nil
	})
}

func TestServiceInfo(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	sd := New(Config{
		Redis: client,
	})

	sd.Register(context.Background(), "test-service", "localhost:8080", map[string]string{
		"version": "1.0.0",
	})

	info, err := sd.ServiceInfo(context.Background(), "test-service", "localhost:8080")
	if err != nil {
		t.Fatalf("ServiceInfo error = %v", err)
	}

	if info.Metadata["version"] != "1.0.0" {
		t.Errorf("version = %s, want 1.0.0", info.Metadata["version"])
	}
}
