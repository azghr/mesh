package errors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeNotFound represents resource not found errors
	ErrorTypeNotFound ErrorType = "not_found"
	// ErrorTypeConflict represents conflict errors (e.g., duplicate resources)
	ErrorTypeConflict ErrorType = "conflict"
	// ErrorTypeUnauthorized represents authentication errors
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	// ErrorTypeForbidden represents authorization errors
	ErrorTypeForbidden ErrorType = "forbidden"
	// ErrorTypeInternal represents internal server errors
	ErrorTypeInternal ErrorType = "internal"
	// ErrorTypeDatabase represents database-related errors
	ErrorTypeDatabase ErrorType = "database"
	// ErrorTypeExternal represents external service errors
	ErrorTypeExternal ErrorType = "external"
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeRateLimit represents rate limiting errors
	ErrorTypeRateLimit ErrorType = "rate_limit"
)

// Logger is a minimal logging interface for error logging.
// This allows the errors package to remain decoupled from any specific logger implementation.
// Consumers can pass their own logger that implements this interface.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	WithFields(fields map[string]any) Logger
	WithField(key string, value any) Logger
	WithError(err error) Logger
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

// AppError represents a structured application error
type AppError struct {
	Type        ErrorType      `json:"type"`
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Details     string         `json:"details,omitempty"`
	Field       string         `json:"field,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	RequestID   string         `json:"request_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	Service     string         `json:"service,omitempty"`
	Operation   string         `json:"operation,omitempty"`
	OriginalErr error          `json:"-"`
}

// Error returns the error message
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// Unwrap returns the original error
func (e *AppError) Unwrap() error {
	return e.OriginalErr
}

// WithField adds a single field to the error metadata
func (e *AppError) WithField(key string, value any) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// WithFields adds multiple fields to the error metadata
func (e *AppError) WithFields(fields map[string]any) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	for k, v := range fields {
		e.Metadata[k] = v
	}
	return e
}

// WithRequestID adds a request ID to the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithUserID adds a user ID to the error
func (e *AppError) WithUserID(userID string) *AppError {
	e.UserID = userID
	return e
}

// WithService adds a service name to the error
func (e *AppError) WithService(service string) *AppError {
	e.Service = service
	return e
}

// WithOperation adds an operation name to the error
func (e *AppError) WithOperation(operation string) *AppError {
	e.Operation = operation
	return e
}

// ToHTTPStatus converts the error type to an HTTP status code
func (e *AppError) ToHTTPStatus() int {
	switch e.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case ErrorTypeForbidden:
		return http.StatusForbidden
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeDatabase, ErrorTypeInternal:
		return http.StatusInternalServerError
	case ErrorTypeExternal:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// ToGRPCStatus converts the error type to a gRPC status
func (e *AppError) ToGRPCStatus() *status.Status {
	var grpcCode codes.Code

	switch e.Type {
	case ErrorTypeValidation:
		grpcCode = codes.InvalidArgument
	case ErrorTypeNotFound:
		grpcCode = codes.NotFound
	case ErrorTypeConflict:
		grpcCode = codes.AlreadyExists
	case ErrorTypeUnauthorized:
		grpcCode = codes.Unauthenticated
	case ErrorTypeForbidden:
		grpcCode = codes.PermissionDenied
	case ErrorTypeTimeout:
		grpcCode = codes.DeadlineExceeded
	case ErrorTypeDatabase, ErrorTypeInternal:
		grpcCode = codes.Internal
	case ErrorTypeExternal:
		grpcCode = codes.Unavailable
	case ErrorTypeRateLimit:
		grpcCode = codes.ResourceExhausted
	default:
		grpcCode = codes.Internal
	}

	return status.New(grpcCode, e.Error())
}

// New creates a new AppError
func New(errorType ErrorType, code, message string) *AppError {
	return &AppError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewWithDetails creates a new AppError with details
func NewWithDetails(errorType ErrorType, code, message, details string) *AppError {
	return &AppError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errorType ErrorType, code, message string) *AppError {
	return &AppError{
		Type:        errorType,
		Code:        code,
		Message:     message,
		Timestamp:   time.Now(),
		OriginalErr: err,
	}
}

// WrapWithDetails wraps an existing error with additional context and details
func WrapWithDetails(err error, errorType ErrorType, code, message, details string) *AppError {
	return &AppError{
		Type:        errorType,
		Code:        code,
		Message:     message,
		Details:     details,
		Timestamp:   time.Now(),
		OriginalErr: err,
	}
}

// ValidationError creates a validation error for a specific field
func ValidationError(field, message string) *AppError {
	err := New(ErrorTypeValidation, "VALIDATION_ERROR", message)
	err.Field = field
	return err
}

// ValidationErrorWithDetails creates a validation error with details
func ValidationErrorWithDetails(field, message, details string) *AppError {
	err := NewWithDetails(ErrorTypeValidation, "VALIDATION_ERROR", message, details)
	err.Field = field
	return err
}

// NotFoundError creates a not found error for a resource
func NotFoundError(resource, identifier string) *AppError {
	return New(ErrorTypeNotFound, "NOT_FOUND", fmt.Sprintf("%s not found", resource)).
		WithField("resource", resource).
		WithField("identifier", identifier)
}

// ConflictError creates a conflict error
func ConflictError(resource, message string) *AppError {
	return New(ErrorTypeConflict, "CONFLICT", message).
		WithField("resource", resource)
}

// UnauthorizedError creates an unauthorized error
func UnauthorizedError(message string) *AppError {
	return New(ErrorTypeUnauthorized, "UNAUTHORIZED", message)
}

// UnauthenticatedError creates an unauthenticated error
func UnauthenticatedError() *AppError {
	return New(ErrorTypeUnauthorized, "UNAUTHENTICATED", "Authentication required")
}

// BadRequestError creates a bad request error
func BadRequestError(message string) *AppError {
	return New(ErrorTypeValidation, "BAD_REQUEST", message)
}

// ForbiddenError creates a forbidden error
func ForbiddenError(message string) *AppError {
	return New(ErrorTypeForbidden, "FORBIDDEN", message)
}

// InternalError creates an internal server error
func InternalError(message string) *AppError {
	return New(ErrorTypeInternal, "INTERNAL_ERROR", message)
}

// DatabaseError creates a database error
func DatabaseError(operation string, err error) *AppError {
	return Wrap(err, ErrorTypeDatabase, "DATABASE_ERROR", "Database operation failed").
		WithField("operation", operation)
}

// ExternalServiceError creates an external service error
func ExternalServiceError(service, operation string, err error) *AppError {
	return Wrap(err, ErrorTypeExternal, "EXTERNAL_SERVICE_ERROR", "External service error").
		WithField("service", service).
		WithField("operation", operation)
}

// TimeoutError creates a timeout error
func TimeoutError(operation string, timeout time.Duration) *AppError {
	return New(ErrorTypeTimeout, "TIMEOUT", "Operation timed out").
		WithField("operation", operation).
		WithField("timeout", timeout.String())
}

// RateLimitError creates a rate limit error
func RateLimitError(message string) *AppError {
	return New(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", message)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError retrieves the AppError from an error
func GetAppError(err error) (*AppError, bool) {
	appErr, ok := err.(*AppError)
	return appErr, ok
}

// FromContext creates an error with context information
func FromContext(ctx context.Context, errorType ErrorType, code, message string) *AppError {
	err := New(errorType, code, message)

	if requestID := ctx.Value(requestIDKey); requestID != nil {
		if rid, ok := requestID.(string); ok {
			err.RequestID = rid
		}
	} else if requestID := ctx.Value("request_id"); requestID != nil {
		if rid, ok := requestID.(string); ok {
			err.RequestID = rid
		}
	}

	if userID := ctx.Value(userIDKey); userID != nil {
		if uid, ok := userID.(string); ok {
			err.UserID = uid
		}
	} else if userID := ctx.Value("user_id"); userID != nil {
		if uid, ok := userID.(string); ok {
			err.UserID = uid
		}
	}

	return err
}

// LogError logs the error using the provided logger.
// The logger parameter must implement the Logger interface defined in this package.
// If log is nil, this function does nothing.
func (e *AppError) LogError(log Logger) {
	if log == nil {
		return
	}

	// Build logger with all error fields
	log = log.WithFields(map[string]any{
		"error_type": e.Type,
		"error_code": e.Code,
		"timestamp":  e.Timestamp,
		"service":    e.Service,
		"operation":  e.Operation,
		"request_id": e.RequestID,
		"user_id":    e.UserID,
	})

	// Add metadata fields
	if e.Metadata != nil {
		for k, v := range e.Metadata {
			log = log.WithField(k, v)
		}
	}

	// Add original error if present
	if e.OriginalErr != nil {
		log = log.WithError(e.OriginalErr)
	}

	// Log at appropriate level based on error type
	switch e.Type {
	case ErrorTypeValidation:
		log.Warn(e.Message)
	case ErrorTypeNotFound:
		log.Info(e.Message)
	case ErrorTypeConflict:
		log.Warn(e.Message)
	case ErrorTypeUnauthorized, ErrorTypeForbidden:
		log.Warn(e.Message)
	case ErrorTypeDatabase, ErrorTypeInternal, ErrorTypeExternal:
		log.Error(e.Message)
	case ErrorTypeTimeout:
		log.Warn(e.Message)
	case ErrorTypeRateLimit:
		log.Warn(e.Message)
	default:
		log.Error(e.Message)
	}
}
