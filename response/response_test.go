package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/azghr/mesh/errors"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()

	type User struct {
		Name string `json:"name"`
	}
	Success(w, User{Name: "john"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp Response[User]
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Name != "john" {
		t.Errorf("expected name john, got %s", resp.Data.Name)
	}
}

func TestSuccessWithMeta(t *testing.T) {
	w := httptest.NewRecorder()

	type User struct {
		Name string `json:"name"`
	}
	SuccessWithMeta(w, []User{{Name: "john"}, {Name: "jane"}}, 2, 1, 10)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp Response[[]User]
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 users, got %d", len(resp.Data))
	}

	if resp.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Meta.Total)
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()

	Created(w, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	err := errors.NotFoundError("user", "123")
	Error(w, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var resp Response[any]
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error.Code != string(errors.ErrorTypeNotFound) {
		t.Errorf("expected error code %s, got %s", errors.ErrorTypeNotFound, resp.Error.Code)
	}
}

func TestErrorWithCode(t *testing.T) {
	w := httptest.NewRecorder()

	ErrorWithCode(w, http.StatusBadRequest, "invalid request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp Response[any]
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error.Message != "invalid request" {
		t.Errorf("expected message, got %s", resp.Error.Message)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()

	BadRequest(w, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()

	Unauthorized(w, "not authenticated")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()

	Forbidden(w, "access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()

	NotFound(w, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()

	Conflict(w, "resource already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()

	InternalError(w, "internal server error")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestWithOptions(t *testing.T) {
	w := httptest.NewRecorder()

	Success(w, "data", WithStatus(http.StatusAccepted), WithHeader("X-Custom", "value"))

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}

	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header")
	}
}

func TestErrorNonAppError(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, genericError{})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

type genericError struct{}

func (e genericError) Error() string {
	return "generic error"
}

func TestJSONContentType(t *testing.T) {
	w := httptest.NewRecorder()

	Success(w, "data")

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}
}
