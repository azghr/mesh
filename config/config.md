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