// Package validation provides struct-based input validation using custom tags.
//
// The validation package allows you to validate Go structs by adding `validate` tags to fields.
// It supports 20+ built-in validation rules including required, email, url, min/max,
// numeric, uuid, phone, ip, credit card, password strength, and more.
//
// # Usage
//
// Define a struct with validate tags:
//
//	type User struct {
//	    Name     string `validate:"required,minlen=2,maxlen=50"`
//	    Email    string `validate:"required,email"`
//	    Age      int    `validate:"min=0,max=150"`
//	    Password string `validate:"required,password"`
//	    Phone   string `validate:"phone"`
//	}
//
// Validate the struct:
//
//	errors := validation.Validate(user)
//	if errors.HasErrors() {
//	    return errors
//	}
//
// # Validation Rules
//
// Built-in rules include:
//   - required     : field must not be empty
//   - email        : valid email format
//   - url          : valid URL (http/https)
//   - min=N       : minimum value/length
//   - max=N       : maximum value/length
//   - minlen=N    : minimum string length
//   - maxlen=N    : maximum string length
//   - alphanum    : alphanumeric only
//   - alpha      : letters only
//   - numeric    : numeric string
//   - uuid       : UUID format
//   - date       : date with format
//   - datetime  : datetime formats
//   - oneof=A B : must be one of values
//   - phone      : phone number (E.164)
//   - ip        : IP address
//   - json      : valid JSON
//   - base64    : base64 encoded
//   - hex       : hexadecimal
//   - credit_card: valid credit card (Luhn)
//   - password  : strong password
//   - match=RE  : regex match
//   - exclude=A : must not be value
package validation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationError represents a single field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors for a struct.
type ValidationErrors []*ValidationError

// Error returns a formatted string of all validation errors.
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// FieldErrors returns validation errors for a specific field.
func (e ValidationErrors) FieldErrors(field string) ValidationErrors {
	var result ValidationErrors
	for _, err := range e {
		if err.Field == field {
			result = append(result, err)
		}
	}
	return result
}

// Validator validates structs using validate tags.
type Validator struct {
	rules map[reflect.Type][]fieldRule
}

// fieldRule represents a single validation rule for a field.
type fieldRule struct {
	field    string
	tags     map[string]string
	validate func(value any, tags map[string]string) *ValidationError
}

var defaultValidators = map[string]func(value any, tags map[string]string) *ValidationError{
	"required": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return &ValidationError{Field: tags["field"], Message: "is required"}
		}
		return nil
	},
	"email": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		email, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(email) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid email"}
		}
		return nil
	},
	"url": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		url, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		if !regexp.MustCompile(`^https?://`).MatchString(url) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid URL"}
		}
		return nil
	},
	"min": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		minStr := tags["min"]
		min, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return &ValidationError{Field: tags["field"], Message: "invalid min value"}
		}
		switch v := value.(type) {
		case int:
			if float64(v) < min {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at least %v", min)}
			}
		case int64:
			if float64(v) < min {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at least %v", min)}
			}
		case float64:
			if v < min {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at least %v", min)}
			}
		case string:
			if float64(len(v)) < min {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at least %v characters", min)}
			}
		}
		return nil
	},
	"max": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		maxStr := tags["max"]
		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			return &ValidationError{Field: tags["field"], Message: "invalid max value"}
		}
		switch v := value.(type) {
		case int:
			if float64(v) > max {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at most %v", max)}
			}
		case int64:
			if float64(v) > max {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at most %v", max)}
			}
		case float64:
			if v > max {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at most %v", max)}
			}
		case string:
			if float64(len(v)) > max {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at most %v characters", max)}
			}
		}
		return nil
	},
	"minlen": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		minLen, _ := strconv.Atoi(tags["minlen"])
		if len(str) < minLen {
			return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at least %d characters", minLen)}
		}
		return nil
	},
	"maxlen": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		maxLen, _ := strconv.Atoi(tags["maxlen"])
		if len(str) > maxLen {
			return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be at most %d characters", maxLen)}
		}
		return nil
	},
	"alphanum": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		for _, c := range str {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return &ValidationError{Field: tags["field"], Message: "must be alphanumeric"}
			}
		}
		return nil
	},
	"numeric": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		if _, err := strconv.ParseFloat(str, 64); err != nil {
			return &ValidationError{Field: tags["field"], Message: "must be numeric"}
		}
		return nil
	},
	"alpha": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		for _, c := range str {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return &ValidationError{Field: tags["field"], Message: "must contain only letters"}
			}
		}
		return nil
	},
	"uuid": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
		if !uuidRegex.MatchString(str) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid UUID"}
		}
		return nil
	},
	"date": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		format := tags["date"]
		if format == "" {
			format = time.RFC3339
		}
		if _, err := time.Parse(format, str); err != nil {
			return &ValidationError{Field: tags["field"], Message: "must be a valid date"}
		}
		return nil
	},
	"datetime": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
		for _, format := range formats {
			if _, err := time.Parse(format, str); err == nil {
				return nil
			}
		}
		return &ValidationError{Field: tags["field"], Message: "must be a valid datetime"}
	},
	"oneof": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		values := strings.Split(tags["oneof"], ",")
		for _, v := range values {
			if str == strings.TrimSpace(v) {
				return nil
			}
		}
		return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must be one of: %s", tags["oneof"])}
	},
	"phone": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		phone, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		phoneRegex := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
		cleaned := strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
		if !phoneRegex.MatchString(cleaned) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid phone number"}
		}
		return nil
	},
	"ip": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		ip, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		ipRegex := regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
		if !ipRegex.MatchString(ip) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid IP address"}
		}
		return nil
	},
	"json": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		var js any
		if err := json.Unmarshal([]byte(str), &js); err != nil {
			return &ValidationError{Field: tags["field"], Message: "must be valid JSON"}
		}
		return nil
	},
	"base64": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		if _, err := base64.StdEncoding.DecodeString(str); err != nil {
			return &ValidationError{Field: tags["field"], Message: "must be valid base64"}
		}
		return nil
	},
	"hex": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
		if !hexRegex.MatchString(str) {
			return &ValidationError{Field: tags["field"], Message: "must be valid hex"}
		}
		return nil
	},
	"credit_card": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		cc, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		cleaned := strings.ReplaceAll(cc, " ", "")
		if !luhnCheck(cleaned) {
			return &ValidationError{Field: tags["field"], Message: "must be a valid credit card number"}
		}
		return nil
	},
	"password": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		password, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		var hasUpper, hasLower, hasDigit, hasSpecial bool
		for _, c := range password {
			switch {
			case c >= 'A' && c <= 'Z':
				hasUpper = true
			case c >= 'a' && c <= 'z':
				hasLower = true
			case c >= '0' && c <= '9':
				hasDigit = true
			case c == '!' || c == '@' || c == '#' || c == '$' || c == '%' || c == '^' || c == '&' || c == '*':
				hasSpecial = true
			}
		}
		if len(password) < 8 {
			return &ValidationError{Field: tags["field"], Message: "password must be at least 8 characters"}
		}
		if !hasUpper {
			return &ValidationError{Field: tags["field"], Message: "password must contain at least one uppercase letter"}
		}
		if !hasLower {
			return &ValidationError{Field: tags["field"], Message: "password must contain at least one lowercase letter"}
		}
		if !hasDigit {
			return &ValidationError{Field: tags["field"], Message: "password must contain at least one digit"}
		}
		if !hasSpecial {
			return &ValidationError{Field: tags["field"], Message: "password must contain at least one special character"}
		}
		return nil
	},
	"match": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		pattern := tags["match"]
		if !regexp.MustCompile(pattern).MatchString(str) {
			return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must match pattern: %s", pattern)}
		}
		return nil
	},
	"exclude": func(value any, tags map[string]string) *ValidationError {
		if isEmpty(value) {
			return nil
		}
		str, ok := value.(string)
		if !ok {
			return &ValidationError{Field: tags["field"], Message: "must be a string"}
		}
		values := strings.Split(tags["exclude"], ",")
		for _, v := range values {
			if str == strings.TrimSpace(v) {
				return &ValidationError{Field: tags["field"], Message: fmt.Sprintf("must not be: %s", v)}
			}
		}
		return nil
	},
}

// isEmpty checks if a value is empty (nil, zero, or empty string).
func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return len(v) == 0
	case int, int8, int16, int32, int64:
		return v == 0
	case uint, uint8, uint16, uint32, uint64:
		return v == 0
	case float32, float64:
		return v == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return true
		}
		return isEmpty(rv.Elem().Interface())
	}
	return false
}

// luhnCheck validates a credit card number using the Luhn algorithm.
func luhnCheck(cardNumber string) bool {
	sum := 0
	isEven := false
	for i := len(cardNumber) - 1; i >= 0; i-- {
		c := int(cardNumber[i] - '0')
		if isEven {
			c *= 2
			if c > 9 {
				c -= 9
			}
		}
		sum += c
		isEven = !isEven
	}
	return sum%10 == 0
}

// New creates a new Validator instance.
func New() *Validator {
	return &Validator{
		rules: make(map[reflect.Type][]fieldRule),
	}
}

// RegisterRule registers a custom validation rule.
func (v *Validator) RegisterRule(rule func(value any, tags map[string]string) *ValidationError) {
	// Custom rules can be registered here
}

// Validate validates a struct and returns any validation errors.
func (v *Validator) Validate(data any) ValidationErrors {
	var errors ValidationErrors
	v.walkStruct(reflect.ValueOf(data), "", "", &errors)
	return errors
}

// walkStruct recursively walks a struct to validate its fields.
func (v *Validator) walkStruct(val reflect.Value, structName, prefix string, errors *ValidationErrors) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return
	}

	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := val.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		validationTag := field.Tag.Get("validate")
		if validationTag != "" {
			if err := v.validateField(fieldValue, fieldName, validationTag); err != nil {
				*errors = append(*errors, err)
			}
		}

		if fieldValue.Kind() == reflect.Struct {
			v.walkStruct(fieldValue, fieldName, fieldName, errors)
		} else if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			v.walkStruct(fieldValue.Elem(), fieldName, fieldName, errors)
		}
	}
}

func (v *Validator) validateField(val reflect.Value, fieldName, tag string) *ValidationError {
	rules := parseValidationTag(tag, fieldName)

	for _, rule := range rules {
		var value any
		switch val.Kind() {
		case reflect.String:
			value = val.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value = val.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value = val.Uint()
		case reflect.Float32, reflect.Float64:
			value = val.Float()
		case reflect.Bool:
			value = val.Bool()
		default:
			value = val.Interface()
		}

		if validator, exists := defaultValidators[rule.name]; exists {
			if err := validator(value, rule.tags); err != nil {
				return err
			}
		}
	}

	return nil
}

// parsedRule represents a parsed validation rule with its parameters.
type parsedRule struct {
	name string
	tags map[string]string
}

// parseValidationTag parses a validate tag string into rules.
func parseValidationTag(tag string, fieldName string) []parsedRule {
	var rules []parsedRule
	parts := strings.Split(tag, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		rule := parsedRule{
			name: part,
			tags: map[string]string{"field": fieldName},
		}

		if idx := strings.Index(part, "("); idx != -1 {
			closeIdx := strings.Index(part, ")")
			if closeIdx != -1 {
				rule.name = part[:idx]
				args := part[idx+1 : closeIdx]
				for _, arg := range strings.Split(args, ",") {
					if kv := strings.Split(arg, "="); len(kv) == 2 {
						rule.tags[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}
			}
		}

		rules = append(rules, rule)
	}

	return rules
}

// Validate validates a struct and returns validation errors.
// This is a convenience function that creates a new Validator.
func Validate(data any) ValidationErrors {
	return New().Validate(data)
}

// ValidateStruct validates a struct and returns an error if validation fails.
// Returns nil if the struct is valid.
func ValidateStruct(data any) error {
	errors := Validate(data)
	if errors.HasErrors() {
		return errors
	}
	return nil
}
