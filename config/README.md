# config

Configuration loading and management for services.

## What It Does

Loads configuration from YAML files with environment variable overrides. Handles validation for production deployments and provides helper functions for common config values.

## Key Features

- **YAML loading** with environment variable expansion (`$DB_HOST` or `${DB_HOST}`)
- **Environment overrides** - `DB_HOST`, `DB_PORT`, `DB_NAME`, etc. take precedence over YAML
- **.env file support** - load defaults from `.env` files
- **Production validation** - ensure required fields are set
- **Type conversion** - helpers for strings, ints, bools, durations, slices

## Usage

```go
// Load config from YAML + env overrides
cfg, err := config.Load("config.yaml", config.WithDefaultConfig())

// Or load just from env (path = "")
cfg, err := config.Load("", config.WithDefaultConfig())

// Validate for production
if err := config.ValidateProduction(cfg); err != nil {
    log.Fatal(err)
}

// Helper functions
url := config.GetDatabaseURL(cfg.Database)
listenAddr := config.GetListenAddr(cfg.Server.Port)

// Individual env helpers
host := config.GetEnv("DB_HOST", "localhost")
port := config.GetEnvAsInt("DB_PORT", 5432)
timeout := config.GetEnvAsDuration("TIMEOUT", 5*time.Second)
```

## Config Struct

```go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
    Port         string        `yaml:"port"`
    Host         string        `yaml:"host"`
    Environment  string        `yaml:"environment"`
    ReadTimeout  time.Duration `yaml:"read_timeout"`
    WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DatabaseConfig struct {
    Host            string        `yaml:"host"`
    Port            string        `yaml:"port"`
    PortInt         int           `yaml:"port_int"`
    User            string        `yaml:"user"`
    Password        string        `yaml:"-"` // Never serialize
    Name            string        `yaml:"name"`
    SSLMode         string        `yaml:"ssl_mode"`
    MaxOpenConns    int           `yaml:"max_open_conns"`
    MaxIdleConns    int           `yaml:"max_idle_conns"`
    ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
    Host         string        `yaml:"host"`
    Port         int           `yaml:"port"`
    Password     string        `yaml:"-"` // Never serialize
    DB           int           `yaml:"db"`
    PoolSize     int           `yaml:"pool_size"`
    MinIdleConns int           `yaml:"min_idle_conns"`
    DialTimeout  time.Duration `yaml:"dial_timeout"`
    ReadTimeout  time.Duration `yaml:"read_timeout"`
    WriteTimeout time.Duration `yaml:"write_timeout"`
}
```

## Environment Variable Mapping

| Config Field | Env Variable |
|--------------|--------------|
| `Database.Host` | `DB_HOST` |
| `Database.Port` | `DB_PORT` |
| `Database.Name` | `DB_NAME` |
| `Database.User` | `DB_USER` |
| `Database.Password` | `DB_PASSWORD` |
| `Database.SSLMode` | `DB_SSL_MODE` |
| `Server.Port` | `SERVER_PORT` |
| `Server.Host` | `SERVER_HOST` |
| `Server.Environment` | `ENV` or `ENVIRONMENT` |