package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the base configuration for a service
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port         string
	Host         string
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host            string
	Port            string
	PortInt         int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string
	JSONFormat bool
	Enabled    bool
	Debug      bool
}

// LoadEnv loads environment variables from .env file if present
// Returns true if .env was loaded, false otherwise
func LoadEnv() bool {
	// Check if godotenv is available
	// For now, just return false - services can implement their own .env loading
	return false
}

// GetEnv returns the value of an environment variable or a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvAsInt returns the value of an environment variable as an integer or a default value
func GetEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetEnvAsFloat returns the value of an environment variable as a float64 or a default value
func GetEnvAsFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// GetEnvAsBool returns the value of an environment variable as a boolean or a default value
func GetEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetEnvAsDuration returns the value of an environment variable as a duration or a default value
func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if durationValue, err := time.ParseDuration(value); err == nil {
			return durationValue
		}
	}
	return defaultValue
}

// GetEnvAsSlice returns the value of an environment variable as a string slice or a default value
// The value should be comma-separated
func GetEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := GetEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	parts := strings.Split(valueStr, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}

	return result
}

// GetDatabaseURL returns a PostgreSQL connection URL from DatabaseConfig
func GetDatabaseURL(cfg DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)
}

// GetDatabaseURLWithPort returns a PostgreSQL connection URL with integer port
func GetDatabaseURLWithPort(cfg DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.PortInt,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)
}

// GetListenAddr returns a properly formatted listen address
func GetListenAddr(port string) string {
	if !strings.Contains(port, ":") {
		return ":" + port
	}
	return port
}

// ValidateConfig validates the base configuration
func ValidateConfig(cfg *Config) error {
	if cfg.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	return nil
}

// ValidateProduction validates production-specific settings
func ValidateProduction(cfg *Config) error {
	if err := ValidateConfig(cfg); err != nil {
		return err
	}

	if cfg.Server.Environment != "production" {
		return nil
	}

	if cfg.Database.SSLMode == "disable" {
		return fmt.Errorf("DB_SSLMODE must be enabled in production")
	}

	return nil
}

// IsProduction checks if the service is running in production mode
func IsProduction(cfg Config) bool {
	return cfg.Server.Environment == "production"
}

// IsDevelopment checks if the service is running in development mode
func IsDevelopment(cfg Config) bool {
	return cfg.Server.Environment == "development"
}

// IsStaging checks if the service is running in staging mode
func IsStaging(cfg Config) bool {
	return cfg.Server.Environment == "staging"
}

// GetLogLevel returns the appropriate log level for the environment
func GetLogLevel(cfg LogConfig) string {
	if cfg.Level != "" {
		return cfg.Level
	}

	if cfg.Debug {
		return "debug"
	}

	return "info"
}
