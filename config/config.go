package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the base configuration for a service
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Log      LogConfig      `yaml:"log" json:"log"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port         string        `yaml:"port" json:"port"`
	Host         string        `yaml:"host" json:"host"`
	Environment  string        `yaml:"environment" json:"environment"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host            string        `yaml:"host" json:"host"`
	Port            string        `yaml:"port" json:"port"`
	PortInt         int           `yaml:"port_int" json:"port_int"`
	User            string        `yaml:"user" json:"user"`
	Password        string        `yaml:"-" json:"-"` // Never serialize password
	Name            string        `yaml:"name" json:"name"`
	SSLMode         string        `yaml:"ssl_mode" json:"ssl_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string `yaml:"level" json:"level"`
	JSONFormat bool   `yaml:"json_format" json:"json_format"`
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Debug      bool   `yaml:"debug" json:"debug"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	Password     string        `yaml:"-" json:"-"` // Never serialize password
	DB           int           `yaml:"db" json:"db"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}

// Options is a functional options pattern for Config
type Options func(*Config)

// WithDefaultConfig returns options that apply sensible defaults
func WithDefaultConfig() Options {
	return func(c *Config) {
		if c.Server.Port == "" {
			c.Server.Port = "8080"
		}
		if c.Server.Environment == "" {
			c.Server.Environment = "development"
		}
		if c.Database.MaxOpenConns == 0 {
			c.Database.MaxOpenConns = 25
		}
		if c.Database.MaxIdleConns == 0 {
			c.Database.MaxIdleConns = 5
		}
		if c.Database.ConnMaxLifetime == 0 {
			c.Database.ConnMaxLifetime = time.Hour
		}
		if c.Log.Level == "" {
			c.Log.Level = "info"
		}
	}
}

// NewConfig creates a new Config with optional defaults applied
func NewConfig(opts ...Options) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Load loads configuration from multiple sources in order of priority:
// 1. YAML file (lowest priority)
// 2. Environment variables (highest priority)
func Load(path string, opts ...Options) (*Config, error) {
	cfg := &Config{}

	// Load from YAML if provided
	if path != "" {
		if err := LoadYAMLWithEnv(path, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
		}
	}

	// Apply environment variables
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		cfg.Database.Port = port
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		cfg.Database.Name = name
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.Database.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Database.Password = password
	}
	if sslmode := os.Getenv("DB_SSL_MODE"); sslmode != "" {
		cfg.Database.SSLMode = sslmode
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg, nil
}

// LoadEnv loads environment variables from .env file if present
// Returns true if .env was loaded, false otherwise
func LoadEnv() bool {
	err := godotenv.Load()
	return err == nil
}

// LoadEnvFrom loads environment variables from a specific .env file
func LoadEnvFrom(path string) bool {
	err := godotenv.Load(path)
	return err == nil
}

// LoadEnvOverride loads environment variables from .env file and overrides existing vars
func LoadEnvOverride() bool {
	err := godotenv.Overload()
	return err == nil
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

// LoadYAML loads configuration from a YAML file
func LoadYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	return yaml.Unmarshal(data, out)
}

// LoadYAMLWithEnv loads configuration from a YAML file and expands environment variables
func LoadYAMLWithEnv(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	expanded := os.ExpandEnv(string(data))
	return yaml.Unmarshal([]byte(expanded), out)
}
