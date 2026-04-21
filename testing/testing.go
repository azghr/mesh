// Package testing provides testing utilities for HTTP handlers and common test patterns.
//
// This package provides helpers for testing HTTP handlers, including:
// - Response status and body assertions
// - JSON comparison
// - HTTP handler testing helpers
// - Stub helpers
//
// Example:
//
//	func TestCreateUser(t *testing.T) {
//	    rr := testing.RunHandler(handler, http.MethodPost, "/users", userJSON)
//
//	    testing.AssertStatus(t, rr.Code, http.StatusCreated)
//	    testing.AssertJSON(t, rr.Body.String(), expected)
//	}
package testing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// NewRequest creates a new HTTP request for testing.
func NewRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// NewRequestWithHeaders creates a request with custom headers.
func NewRequestWithHeaders(method, path, body string, headers map[string]string) *http.Request {
	req := NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// NewJSONRequest creates a JSON request from a value.
func NewJSONRequest(method, path string, body any) *http.Request {
	b, _ := json.Marshal(body)
	return NewRequest(method, path, string(b))
}

// AssertStatus asserts the response status matches expected.
func AssertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

// AssertStatusOK asserts the response is 200 OK.
func AssertStatusOK(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusOK)
}

// AssertStatusCreated asserts the response is 201 Created.
func AssertStatusCreated(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusCreated)
}

// AssertStatusBadRequest asserts the response is 400 Bad Request.
func AssertStatusBadRequest(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusBadRequest)
}

// AssertStatusUnauthorized asserts the response is 401 Unauthorized.
func AssertStatusUnauthorized(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusUnauthorized)
}

// AssertStatusForbidden asserts the response is 403 Forbidden.
func AssertStatusForbidden(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusForbidden)
}

// AssertStatusNotFound asserts the response is 404 Not Found.
func AssertStatusNotFound(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusNotFound)
}

// AssertStatusInternalError asserts the response is 500 Internal Server Error.
func AssertStatusInternalError(t *testing.T, code int) {
	t.Helper()
	AssertStatus(t, code, http.StatusInternalServerError)
}

// AssertJSON asserts the JSON body matches expected (order-independent).
func AssertJSON(t *testing.T, got, want string) {
	t.Helper()

	var gotObj, wantObj any
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		t.Errorf("invalid JSON in got: %v", err)
		return
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Errorf("invalid JSON in want: %v", err)
		return
	}

	if !jsonEqual(gotObj, wantObj) {
		t.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

// AssertJSONContains asserts the JSON contains the expected field/value.
func AssertJSONContains(t *testing.T, body, want string) {
	t.Helper()

	var wantObj map[string]any
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Errorf("invalid JSON in want: %v", err)
		return
	}

	var bodyObj map[string]any
	if err := json.Unmarshal([]byte(body), &bodyObj); err != nil {
		t.Errorf("invalid JSON in body: %v", err)
		return
	}

	for k, v := range wantObj {
		if bv, ok := bodyObj[k]; !ok {
			t.Errorf("missing key %q in JSON", k)
		} else if !equal(v, bv) {
			t.Errorf("key %q: got %v, want %v", k, bv, v)
		}
	}
}

// AssertHeader asserts a response header matches expected.
func AssertHeader(t *testing.T, header http.Header, key, want string) {
	t.Helper()
	got := header.Get(key)
	if got != want {
		t.Errorf("header %q: got %q, want %q", key, got, want)
	}
}

// AssertBody asserts the body matches expected.
func AssertBody(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("body: got %q, want %q", got, want)
	}
}

// AssertError asserts the error is not nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// AssertNoError asserts the error is nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// JSONBody creates JSON string from value.
func JSONBody(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// JSONEqual compares two JSON strings for equality (order-independent).
func JSONEqual(t *testing.T, got, want string) bool {
	t.Helper()

	var gotObj, wantObj any
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		t.Errorf("invalid JSON in got: %v", err)
		return false
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Errorf("invalid JSON in want: %v", err)
		return false
	}

	return jsonEqual(gotObj, wantObj)
}

// RunHandler tests a standard HTTP handler.
func RunHandler(t *testing.T, handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := NewRequest(method, path, body)
	rr := httptest.NewRecorder()
	handler(rr, req)

	return rr
}

// jsonEqual compares two JSON values for equality (order-independent for objects).
func jsonEqual(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for k, av := range av {
			if !equal(av, bv[k]) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !equal(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// equal compares primitive values for equality.
func equal(a, b any) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

// StubDB is a stub database for testing.
type StubDB struct {
	FnQuery  func() error
	FnInsert func() error
	FnGet    func() (any, error)
}

// Query stubs database query.
func (s *StubDB) Query() error {
	if s.FnQuery != nil {
		return s.FnQuery()
	}
	return nil
}

// Insert stubs database insert.
func (s *StubDB) Insert() error {
	if s.FnInsert != nil {
		return s.FnInsert()
	}
	return nil
}

// Get stubs database get.
func (s *StubDB) Get() (any, error) {
	if s.FnGet != nil {
		return s.FnGet()
	}
	return nil, nil
}

// StubCache is a stub cache for testing.
type StubCache struct {
	Data map[string]any
}

// Get returns cached value.
func (s *StubCache) Get(key string) (any, error) {
	if v, ok := s.Data[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}

// Set stores value in cache.
func (s *StubCache) Set(key string, value any) {
	if s.Data == nil {
		s.Data = make(map[string]any)
	}
	s.Data[key] = value
}

// Del removes value from cache.
func (s *StubCache) Del(key string) {
	if s.Data != nil {
		delete(s.Data, key)
	}
}

// Clear removes all values from cache.
func (s *StubCache) Clear() {
	s.Data = make(map[string]any)
}

// StubWriter is a stub http.ResponseWriter for testing.
type StubWriter struct {
	Code int
	Body string
	Hdr  http.Header
}

// NewStubWriter creates a new stub writer.
func NewStubWriter() *StubWriter {
	return &StubWriter{
		Code: http.StatusOK,
		Hdr:  make(http.Header),
	}
}

// Header returns the header map.
func (w *StubWriter) Header() http.Header {
	return w.Hdr
}

// WriteHeader writes the status code.
func (w *StubWriter) WriteHeader(code int) {
	w.Code = code
}

// Write writes the body.
func (w *StubWriter) Write(b []byte) (int, error) {
	w.Body = string(b)
	return len(b), nil
}
