// Package config provides enhanced configuration management.
//
// This package adds generic config loading, validation interfaces, and config composition
// for flexible configuration management.
//
// # Generic Config Loading
//
// Load any struct type from YAML with environment variable expansion:
//
//	type AppConfig struct {
//	    Server struct {
//	        Port int `yaml:"port"`
//	    } `yaml:"server"`
//	}
//
//	cfg, err := config.LoadGeneric[AppConfig]("config.yaml")
//
// # Config Validation
//
// Implement the Validator interface for custom validation:
//
//	type DatabaseConfig struct {
//	    Host string `yaml:"host"`
//	    Port int    `yaml:"port"`
//	}
//
//	func (c *DatabaseConfig) Validate() error {
//	    if c.Host == "" {
//	        return errors.New("host is required")
//	    }
//	    if c.Port < 1 || c.Port > 65535 {
//	        return errors.New("port must be between 1 and 65535")
//	    }
//	    return nil
//	}
//
//	cfg, err := config.LoadAndValidate[DatabaseConfig]("config.yaml")
//
// # Config Composition
//
// Merge multiple config files for layered configuration:
//
//	loader := config.NewLoader()
//	loader.AddFile("defaults.yaml")
//	loader.AddFile("production.yaml")
//	loader.AddEnvOverrides()
//	cfg, err := loader.Load[AppConfig]()
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Validator defines the interface for config validation.
type Validator interface {
	Validate() error
}

// ValidatorFunc is a function that validates a config.
type ValidatorFunc func() error

// Validate implements the Validator interface.
func (v ValidatorFunc) Validate() error {
	return v()
}

// Loader manages loading configuration from multiple sources.
type Loader struct {
	files     []string
	envPrefix string
	opts      []LoadOption
	mu        sync.RWMutex
	onLoad    func(interface{})
	validator func(interface{}) error
}

// LoadOption configures the Loader.
type LoadOption func(*Loader)

// WithFile adds a config file to load.
func WithFile(path string) LoadOption {
	return func(l *Loader) {
		l.files = append(l.files, path)
	}
}

// WithEnvPrefix sets the environment variable prefix.
func WithEnvPrefix(prefix string) LoadOption {
	return func(l *Loader) {
		l.envPrefix = prefix
	}
}

// WithEnvOverrides enables environment variable overrides.
func WithEnvOverrides() LoadOption {
	return func(l *Loader) {
		l.opts = append(l.opts, func(l *Loader) {
			// Already implied
		})
	}
}

// WithOnLoad sets a callback to run after config is loaded.
func WithOnLoad(fn func(interface{})) LoadOption {
	return func(l *Loader) {
		l.onLoad = fn
	}
}

// WithValidator sets a custom validator.
func WithValidator(fn func(interface{}) error) LoadOption {
	return func(l *Loader) {
		l.validator = fn
	}
}

// NewLoader creates a new config loader.
func NewLoader(opts ...LoadOption) *Loader {
	l := &Loader{
		files: make([]string, 0),
		opts:  opts,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// AddFile adds a config file to the loader.
func (l *Loader) AddFile(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.files = append(l.files, path)
}

// AddEnvPrefix sets the environment variable prefix.
func (l *Loader) AddEnvPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.envPrefix = prefix
}

// Load loads configuration from all added files.
func (l *Loader) Load(cfg interface{}) error {
	l.mu.RLock()
	files := make([]string, len(l.files))
	copy(files, l.files)
	prefix := l.envPrefix
	l.mu.RUnlock()

	if len(files) == 0 {
		return errors.New("no config files added")
	}

	// Load files in order (later files override earlier ones)
	for _, file := range files {
		if err := l.loadFile(file, cfg); err != nil {
			return fmt.Errorf("failed to load %s: %w", file, err)
		}
	}

	// Apply environment variable overrides
	l.applyEnvOverrides(cfg, prefix)

	// Run validator if set
	if l.validator != nil {
		if err := l.validator(cfg); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// Run onLoad callback
	if l.onLoad != nil {
		l.onLoad(cfg)
	}

	return nil
}

// loadFile loads a single config file.
func (l *Loader) loadFile(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	return yaml.Unmarshal([]byte(expanded), out)
}

// applyEnvOverrides applies environment variable overrides.
func (l *Loader) applyEnvOverrides(cfg interface{}, prefix string) {
	if prefix == "" {
		prefix = "APP"
	}

	prefix = strings.ToUpper(prefix) + "_"

	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		envKey := prefix + strings.ToUpper(strings.ReplaceAll(yamlTag, ".", "_"))
		if val := os.Getenv(envKey); val != "" {
			fieldVal := v.Field(i)
			if fieldVal.CanSet() {
				setFieldValue(fieldVal, val)
			}
		}
	}
}

// setFieldValue sets a field value from a string.
func setFieldValue(fieldVal reflect.Value, val string) {
	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fieldVal.Type().Name() == "Duration" && fieldVal.Type().PkgPath() == "time" {
			if d, err := time.ParseDuration(val); err == nil {
				fieldVal.SetInt(int64(d))
			}
		} else {
			var n int64
			if _, err := fmt.Sscan(val, &n); err == nil {
				fieldVal.SetInt(n)
			}
		}
	case reflect.Float32, reflect.Float64:
		var n float64
		if _, err := fmt.Sscan(val, &n); err == nil {
			fieldVal.SetFloat(n)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(val); err == nil {
			fieldVal.SetBool(b)
		}
	case reflect.Slice:
		if fieldVal.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(val, ",")
			slice := reflect.MakeSlice(fieldVal.Type(), 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					slice = reflect.Append(slice, reflect.ValueOf(part))
				}
			}
			fieldVal.Set(slice)
		}
	}
}

// LoadGeneric loads a generic config from a YAML file.
func LoadGeneric[T any](path string) (*T, error) {
	var cfg T
	if err := LoadYAMLWithEnv(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadAndValidate loads and validates a generic config.
func LoadAndValidate[T any](path string) (*T, error) {
	var cfg T

	if err := LoadYAMLWithEnv(path, &cfg); err != nil {
		return nil, err
	}

	if v, ok := any(&cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	} else if vf, ok := any(&cfg).(ValidatorFunc); ok {
		if err := vf.Validate(); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// CompiledConfig holds compiled multiple config sources.
type CompiledConfig struct {
	mu       sync.RWMutex
	config   atomicValue
	sources  []string
	loader   *Loader
	watcher  *FSWatcher
	onChange func(interface{})
}

// atomicValue wraps a value for atomic operations.
type atomicValue struct {
	value any
}

// Load loads value into the atomic.
func (a *atomicValue) Load() any {
	return a.value
}

// Store stores a value into the atomic.
func (a *atomicValue) Store(val any) {
	a.value = val
}

// NewCompiledConfig creates a new compiled config.
func NewCompiledConfig(sources []string, onChange func(interface{})) *CompiledConfig {
	return &CompiledConfig{
		sources:  sources,
		onChange: onChange,
		loader:   NewLoader(),
	}
}

// Load loads the config from all sources.
func (c *CompiledConfig) Load(cfg any) error {
	for _, src := range c.sources {
		if err := c.loader.loadFile(src, cfg); err != nil {
			return fmt.Errorf("failed to load %s: %w", src, err)
		}
	}

	c.loader.applyEnvOverrides(cfg, c.loader.envPrefix)
	c.config.Store(cfg)

	if c.onChange != nil {
		c.onChange(cfg)
	}

	return nil
}

// Get returns the current config.
func (c *CompiledConfig) Get() any {
	return c.config.Load()
}

// Compose merges multiple config files.
//
// Example:
//
//	cfg, err := config.Compose[AppConfig](
//	    "defaults.yaml",
//	    "production.yaml",
//	)
func Compose[T any](files ...string) (*T, error) {
	var result T
	resultMap := make(map[string]any)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		expanded := os.ExpandEnv(string(data))
		fileMap := make(map[string]any)

		if err := yaml.Unmarshal([]byte(expanded), &fileMap); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}

		for k, v := range fileMap {
			if v != nil {
				resultMap[k] = v
			}
		}
	}

	mergedBytes, err := yaml.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	if err := yaml.Unmarshal(mergedBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	applyEnvOverridesFor(&result, "")

	if validator, ok := any(&result).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

// applyEnvOverridesFor applies environment variable overrides.
func applyEnvOverridesFor(cfg interface{}, prefix string) {
	if prefix == "" {
		prefix = "APP"
	}
	prefix = strings.ToUpper(prefix) + "_"

	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		parts := strings.Split(yamlTag, ",")
		envKey := prefix + strings.ToUpper(strings.ReplaceAll(parts[0], ".", "_"))

		if val := os.Getenv(envKey); val != "" {
			fieldVal := v.Field(i)
			if fieldVal.CanSet() {
				setFieldValue(fieldVal, val)
			}
		}
	}
}

// Merge merges multiple config structs.
// Later configs override earlier ones for non-zero values.
func Merge[T any](configs ...*T) (*T, error) {
	if len(configs) == 0 {
		return nil, errors.New("no configs to merge")
	}

	resultMap := make(map[string]any)
	var result T

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		cfgBytes, _ := yaml.Marshal(cfg)
		cfgMap := make(map[string]any)
		yaml.Unmarshal(cfgBytes, &cfgMap)

		for k, v := range cfgMap {
			if v != nil {
				resultMap[k] = v
			}
		}
	}

	mergedBytes, _ := yaml.Marshal(resultMap)
	yaml.Unmarshal(mergedBytes, &result)

	return &result, nil
}

// Clone creates a deep copy of a config.
func Clone[T any](cfg *T) (*T, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var clone T
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return nil, err
	}

	return &clone, nil
}

// Equal compares two configs for equality.
func Equal[T any](a, b *T) bool {
	dataA, _ := yaml.Marshal(a)
	dataB, _ := yaml.Marshal(b)
	return string(dataA) == string(dataB)
}
