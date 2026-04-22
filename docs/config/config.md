# Configuration

The config package loads YAML configuration with environment variable overrides.

## Features

- YAML loading with env variable expansion
- Environment variable overrides (DB_HOST, DB_PORT, etc.)
- .env file support
- Production validation

## Quick Example

```go
cfg, err := config.Load("config.yaml", config.WithDefaultConfig())
if err != nil {
    return err
}

// Env vars override YAML
export DB_HOST=prod.example.com
```

## Functions

### Load

```go
func Load(path string, opts ...Options) (*Config, error)
```

Loads config from YAML file, applies env overrides, then options.

### LoadWithHotReload

```go
func LoadWithHotReload(filePath string, opts ...HotReloadOption) (*ConfigLoader, error)
```

Loads config with automatic hot reload support (polling-based).

```go
// Load with hot reload
cfg, err := config.LoadWithHotReload("config.yaml",
    config.WithAutoReloadInterval(30*time.Second),
    config.WithOnChange(func(newCfg *config.Config) {
        log.Info("config reloaded", "environment", newCfg.Server.Environment)
    }),
)

// Get latest config anytime
current := cfg.Get()

// Stop reload on shutdown
cfg.Stop()
```

### LoadWithFSWatcher

```go
func LoadWithFSWatcher(filePath string, opts ...HotReloadOption) (*FSConfigLoader, error)
```

Loads config with real-time file system event-based hot reload (uses fsnotify). Faster than polling-based hot reload.

```go
// Load with file system watcher
cfg, err := config.LoadWithFSWatcher("config.yaml",
    config.WithOnChange(func(newCfg *config.Config) {
        log.Info("config reloaded", "environment", newCfg.Server.Environment)
    }),
)

// Get latest config anytime
current := cfg.Get()

// Stop reload on shutdown
cfg.Stop()
```

Benefits over LoadWithHotReload:
- Immediate detection of file changes (no polling interval)
- Uses OS-level file system notifications via fsnotify
- Lower CPU usage (no periodic checks)

### ValidateProduction

```go
func ValidateProduction(cfg *Config) error
```

Validates production requirements (SSL, required fields).

### Helper Functions

```go
config.GetDatabaseURL(cfg.Database)  // Build connection string
config.GetListenAddr(port)          // Format "host:port"
config.IsProduction(cfg)            // Check environment
```

## Config Types

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Log      LogConfig
    Redis    RedisConfig
}
```

For details, see [config package source](/database/README.md).

## Environment Variables

| Field | Variable |
|-------|----------|
| Database.Host | DB_HOST |
| Database.Port | DB_PORT |
| Database.Name | DB_NAME |
| Server.Port | SERVER_PORT |
| Log.Level | LOG_LEVEL |

## Feature Flags

Gradual rollouts, A/B testing, and feature targeting.

### Quick Start

```go
import "github.com/azghr/mesh/config"

// Create service with Redis backend
ffs := config.NewFeatureFlagService(redisClient)

// Check if feature is enabled
enabled, err := ffs.IsEnabled(ctx, "new_feature", "user-123", nil)
if enabled {
    // Show new feature
}
```

### Configuration

```go
type FeatureFlag struct {
    Key         string    // Flag identifier
    Description string    // Human-readable description
    Enabled    bool      // Default enabled state
    Rules      []FlagRule // Targeting rules
}

type FlagRule struct {
    Percentage  float64  // 0-100 rollout percentage
    UserIDs    []string // User whitelist
    Roles      []string // Role-based targeting
    Attributes map[string]string // Custom attributes
}
```

### Usage Examples

```go
// Simple percentage rollout
flag := config.FeatureFlag{
    Key:      "new_ui",
    Enabled:  true,
    Rules: []config.FlagRule{
        {Percentage: 10}, // 10% of users
    },
}
ffs.SetFlag(ctx, &flag)

// Beta users whitelist
flag = config.FeatureFlag{
    Key:      "beta_feature",
    Enabled:  true,
    Rules: []config.FlagRule{
        {UserIDs: []string{"user-123", "user-456"}},
    },
}

// Role-based feature
flag = config.FeatureFlag{
    Key:      "admin_panel",
    Enabled:  false,
    Rules: []config.FlagRule{
        {Roles: []string{"admin", "superuser"}},
    },
}

// Check in handlers
enabled, _ := ffs.IsEnabled(ctx, "new_ui", userID, map[string]string{
    "role": userRole,
})
if enabled {
    return renderNewUI()
}
```

### Helper Functions

```go
// Enable/disable flags
ffs.EnableFlag(ctx, "feature_key")
ffs.DisableFlag(ctx, "feature_key")

// Set defaults for fallback
ffs.SetDefault("feature_key", false)

// Get all flags
flags, _ := ffs.GetAllFlags(ctx)
```

### Integration

Use with Fiber middleware for automatic flag evaluation:

```go
app.Use(config.FeatureFlagMiddleware(ffs, "new_ui", "beta_feature"))

// In handlers, check via c.Locals
if ffNewUI, ok := c.Locals("ff_new_ui").(bool); ok && ffNewUI {
    return renderNewUI()
}
```

### Best Practices

1. **Fail closed**: Default to disabled for safety
2. **Use consistent hashing**: Same user sees same feature state
3. **Layer rules**: Whitelist users → roles → percentage
4. **Monitor**: Track flag evaluation rates in metrics

## Enhanced Config

Generic config loading, validation, and composition for flexible configuration.

### Generic Loading

```go
type AppConfig struct {
    Server struct {
        Port int    `yaml:"port"`
        Host string `yaml:"host"`
    } `yaml:"server"`
}

// Load any config struct from YAML
cfg, err := config.LoadGeneric[AppConfig]("config.yaml")
```

### Validation Interface

Implement validation in your config struct:

```go
type DatabaseConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

func (c *DatabaseConfig) Validate() error {
    if c.Host == "" {
        return errors.New("host is required")
    }
    if c.Port < 1 || c.Port > 65535 {
        return errors.New("port must be between 1 and 65535")
    }
    return nil
}

// Load and validate
cfg, err := config.LoadAndValidate[DatabaseConfig]("config.yaml")
```

### Config Composition

Merge multiple config files with environment variable overrides:

```go
// Compose merges config from multiple files
// Later files override earlier ones
cfg, err := config.Compose[AppConfig](
    "defaults.yaml",
    "production.yaml",
)
```

### Loader with Multiple Sources

```go
// Create a loader with multiple files
loader := config.NewLoader()
loader.AddFile("defaults.yaml")
loader.AddFile("production.yaml")

// With custom env prefix
loader.AddEnvPrefix("MYAPP")

// With on-load callback
loader := config.NewLoader(
    config.WithOnLoad(func(v any) {
        log.Info("config loaded", "config", v)
    }),
)

// With custom validator
loader := config.NewLoader(
    config.WithValidator(func(v any) error {
        // custom validation logic
        return nil
    }),
)

// Load into struct
var cfg AppConfig
if err := loader.Load(&cfg); err != nil {
    return err
}
```

### Config Operations

```go
// Merge multiple config structs
result, err := config.Merge(cfg1, cfg2, cfg3)

// Clone a config
copy, err := config.Clone(original)

// Compare configs
if config.Equal(cfg1, cfg2) {
    log.Info("configs are equal")
}
```

### Enhanced Functions

| Function | Description |
|----------|-------------|
| LoadGeneric[T] | Load any struct from YAML |
| LoadAndValidate[T] | Load and validate struct |
| Compose[T] | Merge multiple config files |
| NewLoader | Create multi-source loader |
| Merge[T] | Merge config structs |
| Clone[T] | Deep clone config |
| Equal[T] | Compare configs |