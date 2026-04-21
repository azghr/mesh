# featureflags

Feature flag system with Redis backend for gradual rollouts and A/B testing.

## What It Does

Provides feature flags for:
- **Gradual rollouts**: Slowly enable features to percentage of users
- **A/B testing**: Different experiences for different user groups
- **Kill switches**: Quickly disable features without deploying
- **Role-based access**: Enable features for specific roles
- **User targeting**: Whitelist specific users

## Usage

### Basic Setup

```go
ffs := config.NewFeatureFlagService(redisClient)
```

### Define a Flag

```go
flag := config.FeatureFlag{
    Key:         "new_user_profile",
    Description: "New user profile redesign",
    Enabled:     true,
    Rules: []config.FlagRule{
        {Percentage: 10},                    // 10% rollout
        {UserIDs: []string{"u1", "u2"}},     // beta users
        {Roles: []string{"admin"}},           // admin-only
    },
}
```

### Evaluate for User

```go
enabled, _ := ffs.IsEnabled(ctx, "new_user_profile", "user-123", map[string]string{
    "role": "admin",
})
```

### Set Defaults

```go
ffs := config.NewFeatureFlagServiceWithDefaults(redisClient, map[string]bool{
    "new_feature": false,  // default disabled
    "beta_feature": true, // default enabled
})
```

## Targeting Rules

Rules are evaluated in order. First match wins:

1. **User whitelist** (`UserIDs`) - explicit allow list
2. **Role match** (`Roles`) - role-based access
3. **Percentage rollout** - gradual rollout via consistent hashing

### Example Rules

```go
rules := []config.FlagRule{
    // Rule 1: Specific users (highest priority)
    {UserIDs: []string{"user-123", "user-456"}},

    // Rule 2: Role-based
    {Roles: []string{"admin", "developer"}},

    // Rule 3: Percentage rollout (consistent hashing)
    {Percentage: 10}, // 10% of users
}
```

## With Defaults

```go
ffs := config.NewFeatureFlagServiceWithDefaults(redisClient, map[string]bool{
    "new_dashboard": false,
    "beta_ui": true,
})

// Flag not in Redis: uses default
enabled, _ := ffs.IsEnabled(ctx, "new_dashboard", "user-1", nil)
```

## Middleware (Fiber)

### Required Flags

```go
app.Use(config.FeatureFlagMiddleware(ffs, "feature1", "feature2"))
// Features not in allowed list return 404
```

### Optional Flags

```go
app.Use(config.FeatureFlagMiddlewareOptional(ffs, "feature1", "feature2"))
// Adds flag to context but allows continue
```

### Access in Handler

```go
app.Get("/dashboard", func(c *fiber.Ctx) error {
    if c.Locals("feature1") == true {
        return c.Render("new_dashboard", nil)
    }
    return c.Render("old_dashboard", nil)
})
```

## Management Functions

### Enable/Disable

```go
// Enable a flag
ffs.EnableFlag(ctx, "new_feature")

// Disable a flag
ffs.DisableFlag(ctx, "new_feature")
```

### Set Specific Flag

```go
flag := config.FeatureFlag{
    Key:     "new_feature",
    Enabled: true,
    Rules: []config.FlagRule{
        {Percentage: 25},
    },
}
ffs.SetFlag(ctx, flag)
```

### Delete Flag

```go
ffs.DeleteFlag(ctx, "new_feature")
```

### List All Flags

```go
flags, _ := ffs.GetAllFlags(ctx)
for _, flag := range flags {
    fmt.Printf("Flag: %s, Enabled: %v\n", flag.Key, flag.Enabled)
}
```

## How It Works

### Caching

1. Check local in-memory cache first
2. On miss, fetch from Redis
3. Cache locally for 30 seconds

This reduces Redis load while keeping flags responsive to changes.

### Consistent Hashing

Percentage rollouts use FNV hash for consistent user assignment:

```go
hash := fnv.New64a()
hash.Write([]byte(userID + flagKey))
bucket := hash.Sum64() % 100
return float64(bucket) < rule.Percentage
```

Same user always gets same bucket.

## Best Practices

1. **Fail closed**: Default to disabled for safety
2. **Use consistent hashing**: Stable rollouts
3. **Monitor evaluation rates**: Track in metrics
4. **Document flags**: In code and management UI
5. **Clean up**: Remove deprecated flags after full rollout
6. **Layer rules**: whitelist → roles → percentage

## Example: Kill Switch

```go
// Quick disable without deploy
flag := config.FeatureFlag{
    Key:     "payment_feature",
    Enabled: false, // disabled - kill switch
}
ffs.SetFlag(ctx, flag)
```

## Example: A/B Testing

```go
// 50/50 test
flag := config.FeatureFlag{
    Key:         "checkout_v2",
    Enabled:     true,
    Description: "New checkout flow",
    Rules: []config.FlagRule{
        {Percentage: 50},
    },
}
```

## Example: Admin-Only

```go
// Admin preview
flag := config.FeatureFlag{
    Key:         "admin_preview",
    Enabled:     true,
    Description: "Admin preview of new features",
    Rules: []config.FlagRule{
        {Roles: []string{"admin"}},
    },
}
```

## Data Structure (Redis)

```
Key: mesh:flags:{flag_key}
Value: JSON of FeatureFlag
TTL: No expiry (managed by service)
```

## Error Handling

```go
enabled, err := ffs.IsEnabled(ctx, "flag", "user", attrs)
if err != nil {
    // Redis error - fail closed if critical
    return false
}
```

Default behavior on error: return default value.