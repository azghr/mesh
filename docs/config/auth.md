# auth

JWT authentication, Role-Based Access Control (RBAC), and token management.

## What It Does

Three related pieces:
1. **JWT Manager**: Token generation, validation, and refresh
2. **Token Blacklist**: Token invalidation for logout
3. **RBAC**: Role and permission authorization

## Quick Start

```go
// 1. Create JWT manager
jwtManager := auth.NewJWTManager(auth.JWTConfig{
    SigningKey:     []byte("your-secret-key"),
    Issuer:        "your-app",
    Audience:     "your-api",
    AccessTokenTTL:  15 * time.Minute,
    RefreshTokenTTL: 7 * 24 * time.Hour,
}, rbac)

// 2. Generate tokens
tokens, err := jwtManager.GenerateToken(ctx, "user123", "user@example.com", nil)

// 3. Validate token
claims, err := jwtManager.ValidateToken(tokenString)

// 4. Refresh tokens
newTokens, err := jwtManager.RefreshTokens(refreshToken)
```

## JWT Manager

### Configuration

```go
type JWTConfig struct {
    SigningKey      []byte          // Secret key for HMAC (HS256/HS384/HS512)
    PublicKey      *rsa.PublicKey  // RSA public key for RS256 verification
    PrivateKey     *rsa.PrivateKey // RSA private key for RS256 signing
    Algorithm     string         // "HS256", "HS384", "HS512", "RS256"
    Issuer        string         // Token issuer (iss claim)
    Audience     string         // Expected audience (aud claim)
    AccessTokenTTL time.Duration // Default: 15 minutes
    RefreshTokenTTL time.Duration // Default: 7 days
}
```

### Token Generation

```go
// Basic token generation
tokens, err := jwtManager.GenerateToken(ctx, "user123", "user@example.com", nil)

// With custom roles
tokens, err := jwtManager.GenerateToken(ctx, "user123", "email@example.com", []string{"admin", "user:read"})

// Multi-tenant with tenant ID and metadata
tokens, err := jwtManager.GenerateTokenWithTenant(
    ctx, 
    "user123", 
    "email@example.com",
    "tenant-abc",           // tenant_id
    []string{"admin"},       // roles
    map[string]string{"dept": "engineering"}, // metadata
)

// Returns: TokenPair{AccessToken, RefreshToken, ExpiresAt, TokenType}
```

### Token Validation

```go
claims, err := jwtManager.ValidateToken(tokenString)
if err != nil {
    return err // ErrInvalidToken, ErrTokenExpired, ErrInvalidIssuer, ErrInvalidAudience
}

// Claims contain: UserID, Email, Roles, Permissions, TenantID, Metadata
fmt.Println(claims.UserID)
fmt.Println(claims.Roles)
fmt.Println(claims.TenantID)
```

### Token Refresh

```go
// Use refresh token to get new access/refresh pair
newTokens, err := jwtManager.RefreshTokens(refreshToken)
```

### Token Invalidation (Logout)

```go
// Create blacklist (in-memory or Redis-backed)
blacklist := auth.NewTokenBlacklist()

// For distributed logout, use Redis:
// blacklist := auth.NewTokenBlacklist()
// blacklist.SetRedisClient(redisClient)

jwtManager.SetBlacklist(blacklist)

// Invalidate on logout
jwtManager.InvalidateToken(accessToken)

// Check if token is invalidated
isBlocked, _ := blacklist.IsBlacklisted(tokenID)
```

### RSA Keys (Production)

```go
// Create RSA maker
rsaMaker, err := auth.NewRSAJWTMaker(2048) // 2048 or 4096 bits

// Create token
token, _ := rsaMaker.CreateToken("user123")

// Verify token
claims, _ := rsaMaker.VerifyToken(token)

// Get keys
publicKey := rsaMaker.PublicKey()
privateKey := rsaMaker.PrivateKey()

// PEM formatted keys
pubPEM, _ := rsaMaker.PublicKeyPEM()
privPEM, _ := rsaMaker.PrivateKeyPEM()
```

## Fiber Middleware

See `middleware/jwt.go` for Fiber JWT middleware with permission checks.

```go
import "github.com/azghr/mesh/middleware"

// Configure JWT middleware
app.Use(middleware.JWT(middleware.JWTConfig{
    JWTManager: jwtManager,
    TokenLookup: "header:Authorization", // or "query:token" or "cookie:token"
}))

// Protect routes with permission checks
app.Get("/profile", middleware.RequirePermission("user:read"), profileHandler)
app.Post("/admin", middleware.RequireRole("admin"), adminHandler)
app.Get("/manage", middleware.RequireAnyPermission("user:write", "admin:write"), manageHandler)
```

## RBAC (Role-Based Access Control)

### Creating RBAC

```go
// With database-backed role store
roleStore := auth.NewDatabaseRoleStore(func(ctx context.Context, userID string) (auth.Role, error) {
    return db.GetUserRole(ctx, userID)
})
rbac := auth.NewRBAC(roleStore)

// Or with caching
cachedStore := auth.NewCachedRoleStore(roleStore, 5*time.Minute)
rbac := auth.NewRBAC(cachedStore)

// Or default (always returns RoleUser)
rbac := auth.NewRBAC(nil)
```

### Built-in Roles

```go
const (
    RoleAdmin     Role = "admin"
    RoleUser      Role = "user"
    RoleReadOnly  Role = "readonly"
    RolePremium   Role = "premium"
    RoleDeveloper Role = "developer"
)
```

### Built-in Permissions

```go
// Wallet
PermWalletRead, PermWalletWrite, PermWalletDelete

// Portfolio
PermPortfolioRead, PermPortfolioWrite, PermPortfolioDelete

// Analytics
PermAnalyticsRead, PermAnalyticsAdvanced

// Billing
PermBillingRead, PermBillingWrite, PermBillingDelete

// User
PermUserRead, PermUserWrite, PermUserDelete

// Admin
PermAdminRead, PermAdminWrite, PermAdminDelete

// API
PermAPICreate, PermAPIRead, PermAPIDelete

// Workflow
PermWorkflowCreate, PermWorkflowRead, PermWorkflowWrite, PermWorkflowDelete
```

### Checking Permissions

```go
// Check if role has permission
hasPerm := rbac.HasPermission(role, PermWalletRead)

// Check if role has ANY of multiple permissions
hasAny := rbac.HasAnyPermission(role, PermWalletRead, PermAdminRead)

// Check if role has ALL permissions
hasAll := rbac.HasAllPermissions(role, PermWalletRead, PermAnalyticsRead)

// Get all permissions for a user
perms, err := rbac.GetUserPermissions(ctx, userID)
```

### HTTP Middleware

```go
// Require specific permission
rbac.RequirePermission(PermWalletWrite)(handler)

// Require any of multiple permissions
rbac.RequireAnyPermission(PermWalletRead, PermAdminRead)(handler)

// Require all permissions
rbac.RequireAllPermissions(PermWalletRead, PermAnalyticsRead)(handler)

// Require specific role
rbac.RequireRole(RoleAdmin)(handler)
```

### gRPC Interceptors

```go
server := grpc.NewServer(
    grpc.UnaryInterceptor(rbac.GRPCRequirePermission(PermUserRead)),
)
// Or require role
server := grpc.NewServer(
    grpc.UnaryInterceptor(rbac.GRPCRequireRole(RoleAdmin)),
)
```

### Customizing Permissions

```go
// Add permission to role
rbac.AddPermissionToRole(RoleUser, PermCustom)

// Remove permission from role
rbac.RemovePermissionFromRole(RoleUser, PermCustom)

// Set all permissions for a role
rbac.SetRolePermissions(RoleUser, []Permission{PermRead, PermWrite})

// Clear role cache (when user's role changes)
rbac.ClearAllRoleCaches()
rbac.InvalidateRoleCache(userID)
```

## Context Helpers

```go
// Put user in context
ctx := auth.WithUserID(ctx, userID)
ctx = auth.WithRole(ctx, RoleAdmin)

// Get from context
userID := auth.GetUserID(ctx)
role, ok := auth.GetRole(ctx)
```

## OAuth Helpers

```go
// Generate secure random token (for CSRF, OAuth state, etc.)
state := auth.GenerateRandomState()
secureToken := auth.GenerateSecureToken(32) // 32 bytes

// Verify OAuth state (constant-time comparison)
valid := auth.VerifyState(expectedState, actualState)
```