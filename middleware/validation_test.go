package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidationMiddleware_ValidRequest(t *testing.T) {
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := validation.ValidateRequest()(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestValidationMiddleware_ValidJSON(t *testing.T) {
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := validation.ValidateRequest()(handler)

	body := `{"test": "data"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestValidationMiddleware_InvalidJSON(t *testing.T) {
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := validation.ValidateRequest()(handler)

	body := `{"test": invalid}` // Invalid JSON
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}
}

func TestValidationMiddleware_BodyTooLarge(t *testing.T) {
	validation := NewValidationMiddleware()
	validation.SetMaxBodySize(100) // Set small limit

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := validation.ValidateRequest()(handler)

	// Create body larger than 100 bytes
	body := strings.Repeat("x", 101)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for body too large, got %d", w.Code)
	}
}

func TestValidationMiddleware_InvalidContentType(t *testing.T) {
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := validation.ValidateRequest()(handler)

	body := `{"test": "data"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain") // Invalid content type
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415 for invalid content type, got %d", w.Code)
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid JSON", []byte(`{"test": "data"}`), true},
		{"invalid JSON", []byte(`{"test": invalid}`), false},
		{"empty array", []byte(`[]`), true},
		{"empty object", []byte(`{}`), true},
		{"empty string", []byte(``), false}, // Empty string is not valid JSON
		{"null", []byte(`null`), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateJSON(tt.data)
			if got != tt.want {
				t.Errorf("ValidateJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name         string
		contentType  string
		allowedTypes []string
		want         bool
	}{
		{"json allowed", "application/json", []string{"application/json"}, true},
		{"json with charset", "application/json; charset=utf-8", []string{"application/json"}, true},
		{"text not allowed", "text/plain", []string{"application/json"}, false},
		{"empty content type", "", []string{"application/json"}, true},
		{"multiple allowed", "application/json", []string{"application/json", "text/plain"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateContentType(tt.contentType, tt.allowedTypes)
			if got != tt.want {
				t.Errorf("ValidateContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHandlerIs an example showing how to use the middleware in tests
func TestHandlerIs(t *testing.T) {
	// This test shows that the handler receives the original body
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body to verify it's still intact
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
	})

	middleware := validation.ValidateRequest()(handler)

	originalBody := `{"message": "hello"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Body.String() != originalBody {
		t.Errorf("Handler received different body than original")
	}
}

// TestConcurrentReads tests that the middleware handles concurrent requests safely
func TestConcurrentReads(t *testing.T) {
	validation := NewValidationMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := validation.ValidateRequest()(handler)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			body := `{"test": "data"}`
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   interface{}
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with dot", "test.user@example.com", false},
		{"missing @", "testexample.com", true},
		{"@ at start", "@example.com", true},
		{"@ at end", "test@", true},
		{"no domain dot", "test@com", true},
		{"not string", 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%v) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		pw      interface{}
		wantErr bool
	}{
		{"valid password", "Password1", false},
		{"valid complex", "MyP@ssw0rd!", false},
		{"too short", "Pass1", true},
		{"no uppercase", "password1", true},
		{"no lowercase", "PASSWORD1", true},
		{"no digit", "Password", true},
		{"not string", 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%v) error = %v, wantErr %v", tt.pw, err, tt.wantErr)
			}
		})
	}
}

func TestFieldValidator(t *testing.T) {
	v := NewFieldValidator()
	v.AddRule("email", EmailValidator("invalid email"))
	v.AddRule("password", PasswordValidator("password too weak"))

	data := map[string]interface{}{
		"email":    "test@example.com",
		"password": "ValidPass1",
	}

	errs := v.Validate(data)
	if len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}
}

func TestFieldValidator_Error(t *testing.T) {
	v := NewFieldValidator()
	v.AddRule("email", EmailValidator("invalid email"))
	v.AddRule("password", PasswordValidator("password too weak"))

	data := map[string]interface{}{
		"email":    "invalid-email",
		"password": "weak",
	}

	errs := v.Validate(data)
	if len(errs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs))
	}
	if errs["email"] == nil || errs["password"] == nil {
		t.Errorf("Expected both email and password errors")
	}
}
