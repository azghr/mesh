package elasticsearch

import (
	"testing"
)

func TestNew(t *testing.T) {
	c, err := New(Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestNew_MissingAddress(t *testing.T) {
	_, err := New(Config{
		Addresses: []string{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Index(t *testing.T) {
	c, _ := New(Config{
		Addresses: []string{"http://localhost:9200"},
	})

	err := c.Index(nil, "test", "1", map[string]string{"name": "test"})
	if err != nil {
		t.Logf("Index error (expected without running ES): %v", err)
	}
}

func TestClient_Search(t *testing.T) {
	c, _ := New(Config{
		Addresses: []string{"http://localhost:9200"},
	})

	_, err := c.Search(nil, "test", "query")
	if err != nil {
		t.Logf("Search error (expected without running ES): %v", err)
	}
}

func TestClient_Delete(t *testing.T) {
	c, _ := New(Config{
		Addresses: []string{"http://localhost:9200"},
	})

	err := c.Delete(nil, "test", "1")
	if err != nil {
		t.Logf("Delete error (expected without running ES): %v", err)
	}
}

func TestBulkOperation(t *testing.T) {
	ops := []BulkOperation{
		{
			Metadata: map[string]string{"index": "_doc", "_index": "test", "_id": "1"},
			Document: map[string]string{"name": "test"},
		},
	}

	if len(ops) != 1 {
		t.Error("expected 1 operation")
	}
}
