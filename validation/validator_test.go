package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	Name     string `json:"name" validate:"required,minlen=2,maxlen=50"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"min=0,max=150"`
	Password string `json:"password" validate:"required,password"`
	Phone    string `json:"phone" validate:"phone"`
	URL      string `json:"url" validate:"url"`
	IP       string `json:"ip" validate:"ip"`
	UUID     string `json:"uuid" validate:"uuid"`
	Status   string `json:"status" validate:"oneof=active inactive pending"`
	Code     string `json:"code" validate:"alphanum"`
	Balance  int64  `json:"balance" validate:"min=0"`
}

type NestedStruct struct {
	Inner struct {
		Value string `json:"value" validate:"required"`
	} `json:"inner"`
}

type OptionalFields struct {
	Name  string `json:"name" validate:"email"`
	Age   int    `json:"age" validate:"min=18"`
	Phone string `json:"phone"`
}

func TestValidate_Required(t *testing.T) {
	tests := []struct {
		name    string
		data    TestUser
		wantErr bool
	}{
		{
			name: "valid user",
			data: TestUser{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      25,
				Password: "Password1!",
				Phone:    "+1234567890",
				URL:      "https://example.com",
				IP:       "192.168.1.1",
				UUID:     "550e8400-e29b-41d4-a716-446655440000",
				Status:   "active",
				Code:     "ABC123",
				Balance:  100,
			},
			wantErr: false,
		},
		{
			name: "missing required name",
			data: TestUser{
				Email:    "john@example.com",
				Password: "Password1!",
			},
			wantErr: true,
		},
		{
			name: "missing required email",
			data: TestUser{
				Name:     "John",
				Password: "Password1!",
			},
			wantErr: true,
		},
		{
			name: "missing required password",
			data: TestUser{
				Name:  "John",
				Email: "john@example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.data)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Email(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "test@sub.example.com", false},
		{"invalid email no @", "testexample.com", true},
		{"invalid email no domain", "test@", true},
		{"invalid email no tld", "test@example", true},
		{"empty email", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type EmailOnly struct {
				Email string `validate:"email"`
			}
			user := EmailOnly{Email: tt.email}
			errs := Validate(user)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_MinLen(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		minLen  int
		wantErr bool
	}{
		{"valid length", "hello", 3, false},
		{"exactly min length", "hi", 2, false},
		{"too short", "h", 2, true},
		{"empty", "", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Value string `validate:"minlen=2"`
			}
			s := TestStruct{Value: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_MaxLen(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		maxLen  int
		wantErr bool
	}{
		{"valid length", "hello", 10, false},
		{"exactly max length", "hello", 5, false},
		{"too long", "hello world", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Value string `validate:"maxlen=5"`
			}
			s := TestStruct{Value: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_MinMax(t *testing.T) {
	tests := []struct {
		name    string
		age     int
		wantErr bool
	}{
		{"valid age", 25, false},
		{"zero age", 0, false},
		{"min boundary", 0, false},
		{"max boundary", 150, false},
		{"below min", -1, true},
		{"above max", 151, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Age int `validate:"min=0,max=150"`
			}
			s := TestStruct{Age: tt.age}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_URL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http URL", "http://example.com", false},
		{"valid https URL", "https://example.com", false},
		{"valid https with path", "https://example.com/path", false},
		{"invalid no protocol", "example.com", true},
		{"invalid ftp", "ftp://example.com", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				URL string `validate:"url"`
			}
			s := TestStruct{URL: tt.url}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Phone(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{"valid E164", "+14155551234", false},
		{"valid with dashes", "+1-415-555-1234", false},
		{"valid simple", "14155551234", false},
		{"invalid too short", "123", true},
		{"invalid starts with 0", "+04155551234", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Phone string `validate:"phone"`
			}
			s := TestStruct{Phone: tt.phone}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_IP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 localhost", "127.0.0.1", false},
		{"valid IPv4 max", "255.255.255.255", false},
		{"invalid IPv4 too high octet", "256.1.1.1", true},
		{"invalid format", "192.168.1", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				IP string `validate:"ip"`
			}
			s := TestStruct{IP: tt.ip}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_UUID(t *testing.T) {
	tests := []struct {
		name    string
		uuid    string
		wantErr bool
	}{
		{"valid UUID v4", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid lowercase", "550e8400-e29b-41d4-a716-446655440000", false},
		{"invalid no dashes", "550e8400e29b41d4a716446655440000", true},
		{"invalid too short", "550e8400-e29b-41d4-a716", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				UUID string `validate:"uuid"`
			}
			s := TestStruct{UUID: tt.uuid}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_OneOf(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"valid active", "active", false},
		{"valid inactive", "inactive", false},
		{"valid pending", "pending", false},
		{"invalid status", "deleted", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Status string `validate:"oneof=active inactive pending"`
			}
			s := TestStruct{Status: tt.status}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Alphanum(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid alphanumeric", "ABC123", false},
		{"valid lowercase", "abc123", false},
		{"invalid with special char", "ABC-123", true},
		{"invalid with space", "ABC 123", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Code string `validate:"alphanum"`
			}
			s := TestStruct{Code: tt.code}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Password(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "Password1!", false},
		{"valid complex", "MyP@ssw0rd#2024", false},
		{"too short", "Pass1!", true},
		{"no uppercase", "password1!", true},
		{"no lowercase", "PASSWORD1!", true},
		{"no digit", "Password!", true},
		{"no special", "Password1", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Password string `validate:"password"`
			}
			s := TestStruct{Password: tt.password}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_CreditCard(t *testing.T) {
	tests := []struct {
		name    string
		card    string
		wantErr bool
	}{
		{"valid Visa", "4111111111111111", false},
		{"valid Mastercard", "5500000000000004", false},
		{"valid Amex", "340000000000009", false},
		{"invalid number", "1234567890123456", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Card string `validate:"credit_card"`
			}
			s := TestStruct{Card: tt.card}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_JSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{"valid JSON object", `{"key":"value"}`, false},
		{"valid JSON array", `["a","b"]`, false},
		{"valid JSON number", `123`, false},
		{"invalid JSON", `{key:value}`, true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Data string `validate:"json"`
			}
			s := TestStruct{Data: tt.json}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Base64(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{"valid base64", "SGVsbG8gV29ybGQ=", false},
		{"valid base64 no padding", "SGVsbG8gV29ybGQ", false},
		{"invalid base64", "Invalid!", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Data string `validate:"base64"`
			}
			s := TestStruct{Data: tt.data}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Hex(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{"valid hex lowercase", "deadbeef", false},
		{"valid hex uppercase", "DEADBEEF", false},
		{"valid hex mixed", "DeAdBeEf", false},
		{"invalid hex", "hello", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Data string `validate:"hex"`
			}
			s := TestStruct{Data: tt.data}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Match(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		pattern string
		wantErr bool
	}{
		{"matches pattern", "abc123", `^[a-z]+\d+$`, false},
		{"doesn't match", "abc", `^[a-z]+\d+$`, true},
		{"empty", "", `^[a-z]+$`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Value string `validate:"match=^[a-z]+\\d+$"`
			}
			s := TestStruct{Value: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Exclude(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid not excluded", "hello", false},
		{"invalid excluded", "admin", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Value string `validate:"exclude=admin,root,superuser"`
			}
			s := TestStruct{Value: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_NestedStruct(t *testing.T) {
	tests := []struct {
		name    string
		nested  NestedStruct
		wantErr bool
	}{
		{
			name: "valid nested",
			nested: NestedStruct{
				Inner: struct {
					Value string `json:"value" validate:"required"`
				}{Value: "test"},
			},
			wantErr: false,
		},
		{
			name: "missing nested value",
			nested: NestedStruct{
				Inner: struct {
					Value string `json:"value" validate:"required"`
				}{Value: ""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.nested)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_OptionalFields(t *testing.T) {
	t.Run("all empty valid", func(t *testing.T) {
		s := OptionalFields{}
		errs := Validate(s)
		assert.False(t, errs.HasErrors())
	})

	t.Run("valid partial", func(t *testing.T) {
		s := OptionalFields{
			Name: "test@example.com",
		}
		errs := Validate(s)
		assert.False(t, errs.HasErrors())
	})

	t.Run("invalid email", func(t *testing.T) {
		s := OptionalFields{
			Name: "not-an-email",
		}
		errs := Validate(s)
		assert.True(t, errs.HasErrors())
	})

	t.Run("invalid age too young", func(t *testing.T) {
		s := OptionalFields{
			Age: 17,
		}
		errs := Validate(s)
		assert.True(t, errs.HasErrors())
	})
}

func TestValidationErrors_Error(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		errs := ValidationErrors{}
		assert.Equal(t, "", errs.Error())
		assert.False(t, errs.HasErrors())
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "name", Message: "is required"},
			{Field: "email", Message: "must be valid"},
		}
		errStr := errs.Error()
		assert.Contains(t, errStr, "name: is required")
		assert.Contains(t, errStr, "email: must be valid")
	})
}

func TestValidationErrors_FieldErrors(t *testing.T) {
	errs := ValidationErrors{
		{Field: "name", Message: "is required"},
		{Field: "email", Message: "must be valid"},
		{Field: "name", Message: "too short"},
	}

	nameErrors := errs.FieldErrors("name")
	require.Equal(t, 2, len(nameErrors))

	emailErrors := errs.FieldErrors("email")
	require.Equal(t, 1, len(emailErrors))

	otherErrors := errs.FieldErrors("other")
	require.Equal(t, 0, len(otherErrors))
}

func TestValidateStruct(t *testing.T) {
	t.Run("valid struct returns nil", func(t *testing.T) {
		user := TestUser{
			Name:     "John",
			Email:    "john@example.com",
			Password: "Password1!",
		}
		err := ValidateStruct(user)
		assert.Nil(t, err)
	})

	t.Run("invalid struct returns error", func(t *testing.T) {
		user := TestUser{
			Name: "Jo",
		}
		err := ValidateStruct(user)
		assert.NotNil(t, err)
	})
}

func TestValidate_Numeric(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid integer", "123", false},
		{"valid float", "123.45", false},
		{"invalid", "abc", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Num string `validate:"numeric"`
			}
			s := TestStruct{Num: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_Alpha(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid letters", "hello", false},
		{"valid uppercase", "HELLO", false},
		{"invalid with number", "hello1", true},
		{"invalid with space", "hello world", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Value string `validate:"alpha"`
			}
			s := TestStruct{Value: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}

func TestValidate_DateTime(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid RFC3339", "2024-01-15T10:30:00Z", false},
		{"valid date time", "2024-01-15 10:30:00", false},
		{"valid ISO", "2024-01-15T10:30:00", false},
		{"invalid", "not a date", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Date string `validate:"datetime"`
			}
			s := TestStruct{Date: tt.value}
			errs := Validate(s)
			if tt.wantErr {
				assert.True(t, errs.HasErrors())
			} else {
				assert.False(t, errs.HasErrors())
			}
		})
	}
}
