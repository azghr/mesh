package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type enhancedTestConfig struct {
	Server struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Database struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"database"`
}

func (c *enhancedTestConfig) Validate() error {
	if c.Database.Host == "" {
		return errors.New("database host is required")
	}
	return nil
}

func TestLoadGeneric(t *testing.T) {
	content := `server:
  port: 8080
  host: localhost
database:
  host: db.example.com
  port: 5432
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGeneric[enhancedTestConfig](path)
	if err != nil {
		t.Fatalf("LoadGeneric() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %s, want db.example.com", cfg.Database.Host)
	}
}

func TestLoadAndValidate(t *testing.T) {
	content := `database:
  host: db.example.com
  port: 5432
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAndValidate[enhancedTestConfig](path)
	if err != nil {
		t.Fatalf("LoadAndValidate() error = %v", err)
	}

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %s, want db.example.com", cfg.Database.Host)
	}
}

func TestLoadAndValidate_Error(t *testing.T) {
	content := `server:
  port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAndValidate[enhancedTestConfig](path)
	if err == nil {
		t.Fatal("LoadAndValidate() expected error, got nil")
	}
}

func TestLoader(t *testing.T) {
	content := `server:
  port: 8080
  host: localhost
database:
  host: db.example.com
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loader.AddFile(path)

	var cfg enhancedTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
}

func TestLoader_WithValidator(t *testing.T) {
	content := `server:
  port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithValidator(func(v any) error {
		cfg, ok := v.(*enhancedTestConfig)
		if !ok {
			return errors.New("invalid type")
		}
		if cfg.Database.Host == "" {
			return errors.New("database host required")
		}
		return nil
	}))
	loader.AddFile(path)

	var cfg enhancedTestConfig
	err := loader.Load(&cfg)
	if err == nil {
		t.Fatal("Loader.Load() expected validation error, got nil")
	}
}

func TestLoader_MultipleFiles(t *testing.T) {
	content1 := `server:
  port: 8080
  host: localhost
`
	content2 := `server:
  host: 0.0.0.0
database:
  host: db.example.com
`
	dir := t.TempDir()
	path1 := filepath.Join(dir, "config1.yaml")
	path2 := filepath.Join(dir, "config2.yaml")

	if err := os.WriteFile(path1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loader.AddFile(path1)
	loader.AddFile(path2)

	var cfg enhancedTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %s, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %s, want db.example.com", cfg.Database.Host)
	}
}

func TestLoader_EnvOverrides(t *testing.T) {
	type simpleLoaderConfig struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	}

	content := `port: 8080
host: localhost
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("APP_PORT", "9090")
	os.Setenv("APP_HOST", "0.0.0.0")
	defer os.Unsetenv("APP_PORT")
	defer os.Unsetenv("APP_HOST")

	loader := NewLoader(WithEnvPrefix("APP"))
	loader.AddFile(path)

	var cfg simpleLoaderConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %s, want 0.0.0.0", cfg.Host)
	}
}

func TestLoader_NoFiles(t *testing.T) {
	loader := NewLoader()
	var cfg enhancedTestConfig
	err := loader.Load(&cfg)
	if err == nil {
		t.Fatal("Loader.Load() expected error for no files, got nil")
	}
}

type simpleConfig struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

func TestCompose(t *testing.T) {
	content1 := `name: app1
port: 8080
`
	content2 := `port: 9090
`
	dir := t.TempDir()
	path1 := filepath.Join(dir, "defaults.yaml")
	path2 := filepath.Join(dir, "prod.yaml")

	if err := os.WriteFile(path1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Compose[simpleConfig](path1, path2)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	if cfg.Name != "app1" {
		t.Errorf("Name = %s, want app1", cfg.Name)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

func TestCompose_EmptyFiles(t *testing.T) {
	cfg, err := Compose[simpleConfig]()
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if cfg.Name != "" {
		t.Errorf("Name = %s, want empty", cfg.Name)
	}
}

func TestCompose_SingleFile(t *testing.T) {
	content := `name: test
port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Compose[simpleConfig](path)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Name = %s, want test", cfg.Name)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestCompose_SkipsMissingFiles(t *testing.T) {
	content := `name: test
port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.yaml")
	nonexistent := filepath.Join(dir, "nonexistent.yaml")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Compose[simpleConfig](nonexistent, path)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Name = %s, want test", cfg.Name)
	}
}

func TestMerge(t *testing.T) {
	cfg1 := &simpleConfig{Name: "app1", Port: 8080}
	cfg2 := &simpleConfig{Name: "app2", Port: 9090}

	result, err := Merge(cfg1, cfg2)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Name != "app2" {
		t.Errorf("Name = %s, want app2", result.Name)
	}
	if result.Port != 9090 {
		t.Errorf("Port = %d, want 9090", result.Port)
	}
}

func TestMerge_NilConfigs(t *testing.T) {
	cfg1 := &simpleConfig{Name: "app1"}
	cfg2 := (*simpleConfig)(nil)

	result, err := Merge(cfg1, cfg2)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Name != "app1" {
		t.Errorf("Name = %s, want app1", result.Name)
	}
}

func TestMerge_NoConfigs(t *testing.T) {
	_, err := Merge[simpleConfig]()
	if err == nil {
		t.Fatal("Merge() expected error for no configs, got nil")
	}
}

func TestClone(t *testing.T) {
	original := &simpleConfig{Name: "test", Port: 8080}

	clone, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	if clone.Name != original.Name {
		t.Errorf("Clone().Name = %s, want %s", clone.Name, original.Name)
	}
	if clone.Port != original.Port {
		t.Errorf("Clone().Port = %d, want %d", clone.Port, original.Port)
	}

	clone.Name = "modified"
	if original.Name == clone.Name {
		t.Error("Clone() should create independent copy")
	}
}

func TestEqual(t *testing.T) {
	cfg1 := &simpleConfig{Name: "test", Port: 8080}
	cfg2 := &simpleConfig{Name: "test", Port: 8080}
	cfg3 := &simpleConfig{Name: "test", Port: 9090}

	if !Equal(cfg1, cfg2) {
		t.Error("Equal() should return true for equal configs")
	}
	if Equal(cfg1, cfg3) {
		t.Error("Equal() should return false for different configs")
	}
}

func TestWithEnvPrefix(t *testing.T) {
	os.Setenv("MYAPP_PORT", "3000")
	defer os.Unsetenv("MYAPP_PORT")

	type portConfig struct {
		Port int `yaml:"port"`
	}

	content := `port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithEnvPrefix("MYAPP"))
	loader.AddFile(path)

	var cfg portConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000", cfg.Port)
	}
}

func TestWithOnLoad(t *testing.T) {
	content := `server:
  port: 8080
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loaded := false
	loader := NewLoader(WithOnLoad(func(v any) {
		loaded = true
	}))
	loader.AddFile(path)

	var cfg enhancedTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Loader.Load() error = %v", err)
	}

	if !loaded {
		t.Error("OnLoad callback was not called")
	}
}
