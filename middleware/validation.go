// Package middleware provides HTTP middleware for request validation and rate limiting.
//
// # Validation Middleware
//
// The ValidationMiddleware provides comprehensive request validation:
//   - Request body size limits (prevents memory exhaustion)
//   - Content-Type validation (ensures JSON for POST/PUT/PATCH)
//   - JSON format validation (prevents parsing errors)
//
// Example usage:
//
//	validation := middleware.NewValidationMiddleware()
//	validation.SetMaxBodySize(2 << 20) // 2MB
//	http.Handle("/api", validation.ValidateRequest()(handler))
//
// # Field Validators
//
// The FieldValidator provides custom field-level validation for form/JSON input:
//   - Email validation
//   - Password strength validation
//   - Custom rule registration
//
// Example usage:
//
//	v := middleware.NewFieldValidator()
//	v.AddRule("email", middleware.EmailValidator("invalid email"))
//	v.AddRule("password", middleware.PasswordValidator("password too weak"))
//
//	// Validate in handler
//	errs := v.Validate(data)
//	if len(errs) > 0 {
//	    return handleValidationError(w, errs)
//	}
//
// # Rate Limiting Middleware
//
// The RateLimiter provides token-bucket rate limiting per IP address:
//   - Configurable request limits per time window
//   - Automatic cleanup of old entries
//   - Rate limit headers in responses
//
// Example usage:
//
//	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
//	http.Handle("/api", rateLimiter.Middleware(handler))
//
// # Combining Middleware
//
// Middleware can be chained for comprehensive protection:
//
//	validation := middleware.NewValidationMiddleware()
//	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
//
//	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		// Your handler code
//	})
//
//	http.Handle("/api",
//		rateLimiter.Middleware(
//			validation.ValidateRequest()(handler,
//		),
//	)
package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// ValidationMiddleware provides HTTP request validation
type ValidationMiddleware struct {
	maxBodySize int64
}

// NewValidationMiddleware creates a new validation middleware
// maxBodySize is the maximum allowed request body size in bytes (default 1MB)
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		maxBodySize: 1 << 20, // 1MB default
	}
}

// SetMaxBodySize sets the maximum body size
func (v *ValidationMiddleware) SetMaxBodySize(size int64) *ValidationMiddleware {
	v.maxBodySize = size
	return v
}

// ValidateRequest returns an http.Handler that validates incoming requests
func (v *ValidationMiddleware) ValidateRequest() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate content length
			if r.ContentLength > v.maxBodySize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Validate content type for POST/PUT/PATCH
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				ct := r.Header.Get("Content-Type")
				if ct != "" && !strings.HasPrefix(ct, "application/json") {
					http.Error(w, "invalid content type, application/json required", http.StatusUnsupportedMediaType)
					return
				}

				// Validate JSON body if present
				if r.Body != nil && ct != "" && strings.HasPrefix(ct, "application/json") {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "invalid request body", http.StatusBadRequest)
						return
					}
					defer r.Body.Close()

					// Check if body is too large (Content-Length might be missing)
					if int64(len(body)) > v.maxBodySize {
						http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
						return
					}

					// Validate JSON
					if len(body) > 0 && !json.Valid(body) {
						http.Error(w, "invalid JSON format", http.StatusBadRequest)
						return
					}

					// Replace body for next handler
					r.Body = io.NopCloser(strings.NewReader(string(body)))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateJSON is a helper function to validate JSON bytes
func ValidateJSON(data []byte) bool {
	return json.Valid(data)
}

// ValidateContentType checks if the content type is valid
func ValidateContentType(ct string, allowedTypes []string) bool {
	if ct == "" {
		return true
	}
	for _, allowed := range allowedTypes {
		if strings.HasPrefix(ct, allowed) {
			return true
		}
	}
	return false
}

// ValidationError represents a validation error for a specific field
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Validator defines the interface for custom validators
type Validator interface {
	Validate(value interface{}) error
}

// ValidatorFunc is a function adapter for the Validator interface
type ValidatorFunc func(value interface{}) error

// Validate calls the validator function
func (f ValidatorFunc) Validate(value interface{}) error {
	return f(value)
}

// ValidationRule defines a single validation rule for a field
type ValidationRule struct {
	Name     string
	Fn       ValidatorFunc
	ErrorMsg string
}

// FieldValidator validates fields in map[string]interface{} data
type FieldValidator struct {
	rules map[string][]ValidationRule
}

// NewFieldValidator creates a new field validator
func NewFieldValidator() *FieldValidator {
	return &FieldValidator{
		rules: make(map[string][]ValidationRule),
	}
}

// AddRule adds a validation rule for a specific field
func (v *FieldValidator) AddRule(field string, rule ValidationRule) *FieldValidator {
	v.rules[field] = append(v.rules[field], rule)
	return v
}

// Validate validates the data against all registered rules
func (v *FieldValidator) Validate(data interface{}) map[string]*ValidationError {
	errs := make(map[string]*ValidationError)
	vv, ok := data.(map[string]interface{})
	if !ok {
		return errs
	}

	for field, rules := range v.rules {
		value := vv[field]
		for _, rule := range rules {
			if err := rule.Fn.Validate(value); err != nil {
				errs[field] = &ValidationError{
					Field:   field,
					Message: rule.ErrorMsg,
				}
				break
			}
		}
	}
	return errs
}

// ValidateEmail validates an email address format
func ValidateEmail(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	if str == "" {
		return nil
	}
	at := strings.Index(str, "@")
	if at <= 0 || at == len(str)-1 {
		return fmt.Errorf("invalid email format")
	}
	domain := str[at+1:]
	if !strings.Contains(domain, ".") || domain == "." {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// ValidatePassword validates password strength (min 8 chars, upper, lower, digit)
func ValidatePassword(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	if str == "" {
		return nil
	}
	if len(str) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hasUpper, hasLower, hasDigit := false, false, false
	for _, c := range str {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("password must contain uppercase, lowercase, and digit")
	}
	return nil
}

// EmailValidator creates a validation rule for email fields
func EmailValidator(errorMsg string) ValidationRule {
	return ValidationRule{
		Name:     "email",
		Fn:       ValidatorFunc(ValidateEmail),
		ErrorMsg: errorMsg,
	}
}

// PasswordValidator creates a validation rule for password fields
func PasswordValidator(errorMsg string) ValidationRule {
	return ValidationRule{
		Name:     "password",
		Fn:       ValidatorFunc(ValidatePassword),
		ErrorMsg: errorMsg,
	}
}
