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