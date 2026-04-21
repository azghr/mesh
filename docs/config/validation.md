# Validation

The validation package provides struct-based input validation using custom `validate` tags. It supports 20+ built-in validation rules and nested struct validation.

## Features

- Struct tag validation via `validate` tags
- 20+ built-in validation rules
- Nested struct validation
- Custom rule registration
- JSON-compatible error output

## Installation

```bash
go get github.com/anomalyco/mesh/validation
```

## Quick Example

```go
import "github.com/anomalyco/mesh/validation"

type User struct {
    Name     string `json:"name" validate:"required,minlen=2,maxlen=50"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"min=0,max=150"`
    Password string `json:"password" validate:"required,password"`
    Phone   string `json:"phone" validate:"phone"`
}

func CreateUser(u User) error {
    if err := validation.ValidateStruct(u); err != nil {
        return err
    }
    // proceed with creation
    return nil
}
```

## Validation Rules

### Required Fields

| Rule | Description | Example |
|------|-------------|---------|
| `required` | Field must not be empty | `validate:"required"` |

### String Validation

| Rule | Description | Example |
|------|-------------|---------|
| `email` | Valid email format | `validate:"email"` |
| `url` | Valid URL (http/https) | `validate:"url"` |
| `phone` | Phone number (E.164) | `validate:"phone"` |
| `ip` | IP address (IPv4) | `validate:"ip"` |
| `uuid` | UUID format | `validate:"uuid"` |
| `alpha` | Letters only (a-z, A-Z) | `validate:"alpha"` |
| `alphanum` | Alphanumeric only | `validate:"alphanum"` |
| `numeric` | Numeric string | `validate:"numeric"` |
| `hex` | Hexadecimal string | `validate:"hex"` |
| `base64` | Base64 encoded | `validate:"base64"` |
| `json` | Valid JSON | `validate:"json"` |

### Length Validation

| Rule | Description | Example |
|------|-------------|---------|
| `min=N` | Minimum value/length | `validate:"min=5"` |
| `max=N` | Maximum value/length | `validate:"max=100"` |
| `minlen=N` | Minimum string length | `validate:"minlen=2"` |
| `maxlen=N` | Maximum string length | `validate:"maxlen=50"` |

### Format Validation

| Rule | Description | Example |
|------|-------------|---------|
| `date` | Date (default RFC3339) | `validate:"date"` |
| `date=format` | Date with custom format | `validate:"date=2006-01-02"` |
| `datetime` | DateTime (multiple formats) | `validate:"datetime"` |
| `oneof=A B` | Must be one of values | `validate:"oneof=active inactive"` |
| `exclude=A` | Must not be value | `validate:"exclude=admin"` |
| `match=RE` | Must match regex | `validate:"match=^[a-z]+$"` |

### Special Validation

| Rule | Description | Example |
|------|-------------|---------|
| `password` | Strong password | `validate:"password"` |
| `credit_card` | Valid card (Luhn) | `validate:"credit_card"` |

## Functions

### Validate

```go
func Validate(data any) ValidationErrors
```

Validates a struct and returns validation errors.

```go
errors := validation.Validate(user)
if errors.HasErrors() {
    for _, err := range errors {
        fmt.Println(err.Field, err.Message)
    }
}
```

### ValidateStruct

```go
func ValidateStruct(data any) error
```

Validates a struct and returns an error if validation fails. Returns `nil` if valid.

```go
if err := validation.ValidateStruct(user); err != nil {
    return err
}
```

### New

```go
func New() *Validator
```

Creates a new Validator instance for custom configuration.

```go
v := validation.New()
errors := v.Validate(user)
```

## Error Handling

### ValidationError

```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Value   any    `json:"value,omitempty"`
}
```

### ValidationErrors

```go
type ValidationErrors []*ValidationError
```

Collection of validation errors with helper methods:

```go
errors.HasErrors() bool                    // Check if any errors
errors.Error() string                     // Formatted error string
errors.FieldErrors(fieldName) ValidationErrors  // Filter by field
```

### Example Error Handling

```go
func handler(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    if err := validation.ValidateStruct(req); err != nil {
        validationErrs := err.(validation.ValidationErrors)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(validationErrs)
        return
    }
    // proceed
}
```

## Nested Structs

```go
type Address struct {
    Street string `json:"street" validate:"required"`
    City   string `json:"city" validate:"required"`
}

type User struct {
    Name    string  `json:"name" validate:"required"`
    Address Address `json:"address"`
}

user := User{
    Name: "John",
    Address: Address{},
}
errors := validation.Validate(user)
// errors will contain "address.street: is required"
```

## Custom Rules

```go
v := validation.New()
v.RegisterRule(func(value any, tags map[string]string) *validation.ValidationError {
    str, ok := value.(string)
    if !ok {
        return &validation.ValidationError{Field: tags["field"], Message: "must be a string"}
    }
    if !strings.HasPrefix(str, "http") {
        return &validation.ValidationError{Field: tags["field"], Message: "must start with http"}
    }
    return nil
})
```

## Password Validation

The `password` rule requires:
- At least 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character (!@#$%^&*)

```go
type RegisterRequest struct {
    Password string `json:"password" validate:"required,password"`
}
```

## Credit Card Validation

The `credit_card` rule uses the Luhn algorithm to validate card numbers.

```go
type PaymentRequest struct {
    CardNumber string `json:"card_number" validate:"credit_card"`
}
```

Valid test cards:
- Visa: `4111111111111111`
- Mastercard: `5500000000000004`
- Amex: `340000000000009`

## Combining Rules

Multiple rules can be combined with commas:

```go
type User struct {
    Name     string `validate:"required,minlen=2,maxlen=50"`
    Email    string `validate:"required,email"`
    Age      int    `validate:"min=0,max=150"`
    Status   string `validate:"oneof=active inactive pending"`
}
```

## Testing

Run tests with:

```bash
go test ./validation/...
```

The package includes comprehensive tests for all validation rules.