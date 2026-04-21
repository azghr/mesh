package testing

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestAssertStatus(t *testing.T) {
	AssertStatus(t, 200, 200)
}

func TestAssertStatusOK(t *testing.T) {
	AssertStatusOK(t, 200)
}

func TestAssertStatusCreated(t *testing.T) {
	AssertStatusCreated(t, 201)
}

func TestAssertStatusBadRequest(t *testing.T) {
	AssertStatusBadRequest(t, 400)
}

func TestAssertStatusNotFound(t *testing.T) {
	AssertStatusNotFound(t, 404)
}

func TestAssertJSON_Equal(t *testing.T) {
	got := `{"name":"john","age":30}`
	want := `{"age":30,"name":"john"}`

	AssertJSON(t, got, want)
}

func TestAssertJSONContains(t *testing.T) {
	body := `{"name":"john","age":30,"email":"john@example.com"}`
	want := `{"name":"john","age":30}`

	AssertJSONContains(t, body, want)
}

func TestAssertHeader(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")

	AssertHeader(t, header, "Content-Type", "application/json")
}

func TestAssertBody(t *testing.T) {
	AssertBody(t, "hello", "hello")
}

func TestAssertError(t *testing.T) {
	AssertError(t, errors.New("error"))
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestJSONBody(t *testing.T) {
	m := map[string]string{"name": "john"}
	got := JSONBody(m)

	var gotMap map[string]string
	json.Unmarshal([]byte(got), &gotMap)

	if gotMap["name"] != "john" {
		t.Errorf("expected name=john, got %v", gotMap)
	}
}

func TestNewRequest(t *testing.T) {
	req := NewRequest("POST", "/users", `{"name":"john"}`)

	if req.Method != "POST" {
		t.Errorf("expected POST, got %s", req.Method)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", req.Header.Get("Content-Type"))
	}
}

func TestNewRequestWithHeaders(t *testing.T) {
	req := NewRequestWithHeaders("GET", "/", "", map[string]string{
		"Authorization": "Bearer token",
	})

	if req.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("expected Bearer token, got %s", req.Header.Get("Authorization"))
	}
}

func TestNewJSONRequest(t *testing.T) {
	req := NewJSONRequest("POST", "/users", map[string]string{"name": "john"})

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", req.Header.Get("Content-Type"))
	}
}

func TestRunHandler(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}

	rr := RunHandler(t, handler, "GET", "/", "")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if rr.Body.String() != `{"status":"ok"}` {
		t.Errorf("expected body, got %s", rr.Body.String())
	}
}

func TestStubCache(t *testing.T) {
	cache := &StubCache{}

	cache.Set("key1", "value1")
	val, err := cache.Get("key1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	_, err = cache.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}

	cache.Del("key1")
	_, err = cache.Get("key1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStubDB(t *testing.T) {
	db := &StubDB{
		FnQuery: func() error { return nil },
		FnGet:   func() (any, error) { return "value", nil },
	}

	if err := db.Query(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	val, err := db.Get()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if val != "value" {
		t.Errorf("expected value, got %v", val)
	}
}

func TestStubWriter(t *testing.T) {
	w := NewStubWriter()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if w.Body != "hello" {
		t.Errorf("expected hello, got %s", w.Body)
	}

	w.Header().Set("X-Custom", "value")
	if w.Hdr.Get("X-Custom") != "value" {
		t.Errorf("expected header value, got %s", w.Hdr.Get("X-Custom"))
	}
}
