package errors

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST_CODE", "Test message")

	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected type %s, got %s", ErrorTypeValidation, err.Type)
	}
	if err.Code != "TEST_CODE" {
		t.Errorf("Expected code TEST_CODE, got %s", err.Code)
	}
	if err.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", err.Message)
	}
}

func TestNewWithDetails(t *testing.T) {
	err := NewWithDetails(ErrorTypeInternal, "INTERNAL_ERROR", "Internal error", "Database connection failed")

	if err.Details != "Database connection failed" {
		t.Errorf("Expected details 'Database connection failed', got %s", err.Details)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrapped := Wrap(originalErr, ErrorTypeDatabase, "DB_ERROR", "Database query failed")

	if wrapped.OriginalErr != originalErr {
		t.Error("Original error not preserved")
	}
	if unwrapped := wrapped.Unwrap(); unwrapped != originalErr {
		t.Error("Unwrap did not return original error")
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "Without details",
			err:      New(ErrorTypeValidation, "TEST", "Test message"),
			expected: "Test message",
		},
		{
			name:     "With details",
			err:      NewWithDetails(ErrorTypeValidation, "TEST", "Test message", "Field is required"),
			expected: "Test message: Field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestAppError_WithField(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	err = err.WithField("key1", "value1").WithField("key2", 123)

	if err.Metadata == nil {
		t.Fatal("Metadata not initialized")
	}
	if err.Metadata["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", err.Metadata["key1"])
	}
	if err.Metadata["key2"] != 123 {
		t.Errorf("Expected key2=123, got %v", err.Metadata["key2"])
	}
}

func TestAppError_WithFields(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	fields := map[string]any{
		"user_id": "123",
		"action":  "test",
	}
	err = err.WithFields(fields)

	if err.Metadata["user_id"] != "123" {
		t.Errorf("Expected user_id=123, got %v", err.Metadata["user_id"])
	}
	if err.Metadata["action"] != "test" {
		t.Errorf("Expected action=test, got %v", err.Metadata["action"])
	}
}

func TestAppError_WithRequestID(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	err = err.WithRequestID("req-123")

	if err.RequestID != "req-123" {
		t.Errorf("Expected RequestID=req-123, got %s", err.RequestID)
	}
}

func TestAppError_WithUserID(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	err = err.WithUserID("user-123")

	if err.UserID != "user-123" {
		t.Errorf("Expected UserID=user-123, got %s", err.UserID)
	}
}

func TestAppError_WithService(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	err = err.WithService("test-service")

	if err.Service != "test-service" {
		t.Errorf("Expected Service=test-service, got %s", err.Service)
	}
}

func TestAppError_WithOperation(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test")
	err = err.WithOperation("test-operation")

	if err.Operation != "test-operation" {
		t.Errorf("Expected Operation=test-operation, got %s", err.Operation)
	}
}

func TestAppError_Chaining(t *testing.T) {
	err := New(ErrorTypeValidation, "TEST", "Test").
		WithRequestID("req-123").
		WithUserID("user-123").
		WithService("test-service").
		WithOperation("test-operation").
		WithField("key", "value")

	if err.RequestID != "req-123" {
		t.Error("RequestID not set in chain")
	}
	if err.UserID != "user-123" {
		t.Error("UserID not set in chain")
	}
	if err.Service != "test-service" {
		t.Error("Service not set in chain")
	}
	if err.Operation != "test-operation" {
		t.Error("Operation not set in chain")
	}
	if err.Metadata["key"] != "value" {
		t.Error("Field not set in chain")
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError("email", "Invalid email format")

	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected type %s, got %s", ErrorTypeValidation, err.Type)
	}
	if err.Field != "email" {
		t.Errorf("Expected field email, got %s", err.Field)
	}
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("User", "123")

	if err.Type != ErrorTypeNotFound {
		t.Errorf("Expected type %s, got %s", ErrorTypeNotFound, err.Type)
	}
	if err.Metadata["resource"] != "User" {
		t.Errorf("Expected resource User, got %v", err.Metadata["resource"])
	}
	if err.Metadata["identifier"] != "123" {
		t.Errorf("Expected identifier 123, got %v", err.Metadata["identifier"])
	}
}

func TestConflictError(t *testing.T) {
	err := ConflictError("User", "User already exists")

	if err.Type != ErrorTypeConflict {
		t.Errorf("Expected type %s, got %s", ErrorTypeConflict, err.Type)
	}
	if err.Metadata["resource"] != "User" {
		t.Errorf("Expected resource User, got %v", err.Metadata["resource"])
	}
}

func TestUnauthorizedError(t *testing.T) {
	err := UnauthorizedError("Invalid token")

	if err.Type != ErrorTypeUnauthorized {
		t.Errorf("Expected type %s, got %s", ErrorTypeUnauthorized, err.Type)
	}
}

func TestForbiddenError(t *testing.T) {
	err := ForbiddenError("Access denied")

	if err.Type != ErrorTypeForbidden {
		t.Errorf("Expected type %s, got %s", ErrorTypeForbidden, err.Type)
	}
}

func TestInternalError(t *testing.T) {
	err := InternalError("Something went wrong")

	if err.Type != ErrorTypeInternal {
		t.Errorf("Expected type %s, got %s", ErrorTypeInternal, err.Type)
	}
}

func TestDatabaseError(t *testing.T) {
	originalErr := errors.New("connection failed")
	err := DatabaseError("query_users", originalErr)

	if err.Type != ErrorTypeDatabase {
		t.Errorf("Expected type %s, got %s", ErrorTypeDatabase, err.Type)
	}
	if err.OriginalErr != originalErr {
		t.Error("Original error not preserved")
	}
	if err.Metadata["operation"] != "query_users" {
		t.Errorf("Expected operation query_users, got %v", err.Metadata["operation"])
	}
}

func TestExternalServiceError(t *testing.T) {
	originalErr := errors.New("service unavailable")
	err := ExternalServiceError("Stripe", "CreateCharge", originalErr)

	if err.Type != ErrorTypeExternal {
		t.Errorf("Expected type %s, got %s", ErrorTypeExternal, err.Type)
	}
	if err.Metadata["service"] != "Stripe" {
		t.Errorf("Expected service Stripe, got %v", err.Metadata["service"])
	}
	if err.Metadata["operation"] != "CreateCharge" {
		t.Errorf("Expected operation CreateCharge, got %v", err.Metadata["operation"])
	}
}

func TestTimeoutError(t *testing.T) {
	err := TimeoutError("FetchData", 5*time.Second)

	if err.Type != ErrorTypeTimeout {
		t.Errorf("Expected type %s, got %s", ErrorTypeTimeout, err.Type)
	}
	if err.Metadata["operation"] != "FetchData" {
		t.Errorf("Expected operation FetchData, got %v", err.Metadata["operation"])
	}
	if err.Metadata["timeout"] != "5s" {
		t.Errorf("Expected timeout 5s, got %v", err.Metadata["timeout"])
	}
}

func TestRateLimitError(t *testing.T) {
	err := RateLimitError("Too many requests")

	if err.Type != ErrorTypeRateLimit {
		t.Errorf("Expected type %s, got %s", ErrorTypeRateLimit, err.Type)
	}
}

func TestToHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		expected int
	}{
		{"Validation", ErrorTypeValidation, http.StatusBadRequest},
		{"NotFound", ErrorTypeNotFound, http.StatusNotFound},
		{"Conflict", ErrorTypeConflict, http.StatusConflict},
		{"Unauthorized", ErrorTypeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", ErrorTypeForbidden, http.StatusForbidden},
		{"Timeout", ErrorTypeTimeout, http.StatusRequestTimeout},
		{"RateLimit", ErrorTypeRateLimit, http.StatusTooManyRequests},
		{"Database", ErrorTypeDatabase, http.StatusInternalServerError},
		{"Internal", ErrorTypeInternal, http.StatusInternalServerError},
		{"External", ErrorTypeExternal, http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, "TEST", "Test")
			if got := err.ToHTTPStatus(); got != tt.expected {
				t.Errorf("ToHTTPStatus() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestIsAppError(t *testing.T) {
	appErr := New(ErrorTypeValidation, "TEST", "Test")
	stdErr := errors.New("standard error")

	if !IsAppError(appErr) {
		t.Error("IsAppError returned false for AppError")
	}
	if IsAppError(stdErr) {
		t.Error("IsAppError returned true for standard error")
	}
}

func TestGetAppError(t *testing.T) {
	appErr := New(ErrorTypeValidation, "TEST", "Test")
	stdErr := errors.New("standard error")

	got, ok := GetAppError(appErr)
	if !ok || got != appErr {
		t.Error("GetAppError failed to return AppError")
	}

	_, ok = GetAppError(stdErr)
	if ok {
		t.Error("GetAppError returned true for standard error")
	}
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-123")

	err := FromContext(ctx, ErrorTypeValidation, "TEST", "Test")

	if err.RequestID != "req-123" {
		t.Errorf("Expected RequestID=req-123, got %s", err.RequestID)
	}
	if err.UserID != "user-123" {
		t.Errorf("Expected UserID=user-123, got %s", err.UserID)
	}
}

func TestFromContext_NoValues(t *testing.T) {
	ctx := context.Background()
	err := FromContext(ctx, ErrorTypeValidation, "TEST", "Test")

	if err.RequestID != "" {
		t.Errorf("Expected empty RequestID, got %s", err.RequestID)
	}
	if err.UserID != "" {
		t.Errorf("Expected empty UserID, got %s", err.UserID)
	}
}
