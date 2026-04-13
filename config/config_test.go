package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name         string
		key          string
		defaultValue string
		expected     string
	}{
		{"Existing variable", "TEST_VAR", "default", "test_value"},
		{"Non-existing variable", "NON_EXISTING_VAR", "default", "default"},
		{"Empty string as default", "NON_EXISTING_VAR", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnv(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("GetEnv() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	os.Setenv("TEST_INVALID_INT", "not_a_number")
	defer os.Unsetenv("TEST_INT")
	defer os.Unsetenv("TEST_INVALID_INT")

	tests := []struct {
		name         string
		key          string
		defaultValue int
		expected     int
	}{
		{"Valid integer", "TEST_INT", 0, 42},
		{"Invalid integer", "TEST_INVALID_INT", 10, 10},
		{"Non-existing variable", "NON_EXISTING", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnvAsInt(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("GetEnvAsInt() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestGetEnvAsFloat(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	os.Setenv("TEST_INVALID_FLOAT", "not_a_float")
	defer os.Unsetenv("TEST_FLOAT")
	defer os.Unsetenv("TEST_INVALID_FLOAT")

	tests := []struct {
		name         string
		key          string
		defaultValue float64
		expected     float64
	}{
		{"Valid float", "TEST_FLOAT", 0.0, 3.14},
		{"Invalid float", "TEST_INVALID_FLOAT", 2.5, 2.5},
		{"Non-existing variable", "NON_EXISTING", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnvAsFloat(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("GetEnvAsFloat() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestGetEnvAsBool(t *testing.T) {
	os.Setenv("TEST_TRUE", "true")
	os.Setenv("TEST_FALSE", "false")
	os.Setenv("TEST_ONE", "1")
	os.Setenv("TEST_INVALID_BOOL", "not_a_bool")
	defer os.Unsetenv("TEST_TRUE")
	defer os.Unsetenv("TEST_FALSE")
	defer os.Unsetenv("TEST_ONE")
	defer os.Unsetenv("TEST_INVALID_BOOL")

	tests := []struct {
		name         string
		key          string
		defaultValue bool
		expected     bool
	}{
		{"True value", "TEST_TRUE", false, true},
		{"False value", "TEST_FALSE", true, false},
		{"One value", "TEST_ONE", false, true},
		{"Invalid bool", "TEST_INVALID_BOOL", true, true},
		{"Non-existing variable", "NON_EXISTING", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnvAsBool(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("GetEnvAsBool() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	os.Setenv("TEST_DURATION", "5s")
	os.Setenv("TEST_INVALID_DURATION", "not_a_duration")
	defer os.Unsetenv("TEST_DURATION")
	defer os.Unsetenv("TEST_INVALID_DURATION")

	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"Valid duration", "TEST_DURATION", 0, 5 * time.Second},
		{"Invalid duration", "TEST_INVALID_DURATION", time.Second, time.Second},
		{"Non-existing variable", "NON_EXISTING", 2 * time.Second, 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnvAsDuration(tt.key, tt.defaultValue); got != tt.expected {
				t.Errorf("GetEnvAsDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetEnvAsSlice(t *testing.T) {
	os.Setenv("TEST_SLICE", "a,b,c")
	os.Setenv("TEST_SLICE_SPACES", " a , b , c ")
	os.Setenv("TEST_EMPTY_SLICE", "")
	defer os.Unsetenv("TEST_SLICE")
	defer os.Unsetenv("TEST_SLICE_SPACES")
	defer os.Unsetenv("TEST_EMPTY_SLICE")

	defaultSlice := []string{"default"}

	tests := []struct {
		name         string
		key          string
		defaultValue []string
		expected     []string
	}{
		{"Valid slice", "TEST_SLICE", defaultSlice, []string{"a", "b", "c"}},
		{"Slice with spaces", "TEST_SLICE_SPACES", defaultSlice, []string{"a", "b", "c"}},
		{"Empty string", "TEST_EMPTY_SLICE", defaultSlice, defaultSlice},
		{"Non-existing variable", "NON_EXISTING", defaultSlice, defaultSlice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnvAsSlice(tt.key, tt.defaultValue)
			if len(got) != len(tt.expected) {
				t.Errorf("GetEnvAsSlice() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("GetEnvAsSlice()[%d] = %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetDatabaseURL(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	if got := GetDatabaseURL(cfg); got != expected {
		t.Errorf("GetDatabaseURL() = %s, want %s", got, expected)
	}
}

func TestGetDatabaseURLWithPort(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		PortInt:  5432,
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	if got := GetDatabaseURLWithPort(cfg); got != expected {
		t.Errorf("GetDatabaseURLWithPort() = %s, want %s", got, expected)
	}
}

func TestGetListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected string
	}{
		{"Port only", "8080", ":8080"},
		{"Full address", "0.0.0.0:8080", "0.0.0.0:8080"},
		{"Localhost", "localhost:8080", "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetListenAddr(tt.port); got != tt.expected {
				t.Errorf("GetListenAddr() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			cfg: &Config{
				Database: DatabaseConfig{
					Host: "localhost",
					Name: "testdb",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing host",
			cfg: &Config{
				Database: DatabaseConfig{
					Name: "testdb",
				},
			},
			wantErr: true,
			errMsg:  "DB_HOST is required",
		},
		{
			name: "Missing name",
			cfg: &Config{
				Database: DatabaseConfig{
					Host: "localhost",
				},
			},
			wantErr: true,
			errMsg:  "DB_NAME is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateConfig() error = %v, want error containing %s", err, tt.errMsg)
			}
		})
	}
}

func TestValidateProduction(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid production config",
			cfg: &Config{
				Server: ServerConfig{
					Environment: "production",
				},
				Database: DatabaseConfig{
					Host:    "localhost",
					Name:    "testdb",
					SSLMode: "require",
				},
			},
			wantErr: false,
		},
		{
			name: "Production without SSL",
			cfg: &Config{
				Server: ServerConfig{
					Environment: "production",
				},
				Database: DatabaseConfig{
					Host:    "localhost",
					Name:    "testdb",
					SSLMode: "disable",
				},
			},
			wantErr: true,
			errMsg:  "DB_SSLMODE must be enabled in production",
		},
		{
			name: "Development config",
			cfg: &Config{
				Server: ServerConfig{
					Environment: "development",
				},
				Database: DatabaseConfig{
					Host:    "localhost",
					Name:    "testdb",
					SSLMode: "disable",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProduction(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProduction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateProduction() error = %v, want error containing %s", err, tt.errMsg)
			}
		})
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		environment string
		expected bool
	}{
		{"Production", "production", true},
		{"Development", "development", false},
		{"Staging", "staging", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Server: ServerConfig{Environment: tt.environment}}
			if got := IsProduction(cfg); got != tt.expected {
				t.Errorf("IsProduction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		environment string
		expected bool
	}{
		{"Development", "development", true},
		{"Production", "production", false},
		{"Staging", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Server: ServerConfig{Environment: tt.environment}}
			if got := IsDevelopment(cfg); got != tt.expected {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsStaging(t *testing.T) {
	tests := []struct {
		name     string
		environment string
		expected bool
	}{
		{"Staging", "staging", true},
		{"Development", "development", false},
		{"Production", "production", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Server: ServerConfig{Environment: tt.environment}}
			if got := IsStaging(cfg); got != tt.expected {
				t.Errorf("IsStaging() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		cfg      LogConfig
		expected string
	}{
		{"Explicit level", LogConfig{Level: "debug"}, "debug"},
		{"Debug mode", LogConfig{Debug: true}, "debug"},
		{"Default", LogConfig{}, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetLogLevel(tt.cfg); got != tt.expected {
				t.Errorf("GetLogLevel() = %s, want %s", got, tt.expected)
			}
		})
	}
}
