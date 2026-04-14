# auth

JWT authentication middleware and Role-Based Access Control (RBAC).

## What It Does

Two related but independent pieces:
1. **Middleware**: Extract and validate JWT tokens, store user info in context
2. **RBAC**: Define roles and permissions, check if users can perform actions

## JWT Middleware

### Basic Usage

```go
// Create middleware with secret
authMiddleware := auth.NewJWTmiddleware(secretKey)

// Use with Fiber, Gin, etc.
app.Use(authMiddleware.Handler)

// In handlers, get user info from context
userID := auth.GetUserID(ctx)
role := auth.GetRole(ctx)
```

### Configuration

```go
config := &auth.JWTConfig{
    Secret:        "your-secret-key",
    TokenLookup:   "header:Authorization",  // Where to find token
    TokenPrefix:   "Bearer",                 // Prefix to strip
    Expiration:    time.Hour,                // Token expiry
    SigningMethod: "HS256",
}
middleware := auth.NewJWTmiddlewareFromConfig(config)
```

## RBAC (Role-Based Access Control)

### Creating RBAC

```go
// With database-backed role store
roleStore := auth.NewDatabaseRoleStore(func(ctx context.Context, userID string) (auth.Role, error) {
    // Query your database
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

// ZK Proof
PermZKCreate, PermZKRead, PermZKVerify
```

### Checking Permissions

```go
// Check if role has permission
hasPerm := rbac.HasPermission(role, PermWalletRead)

// Check if role has ANY of multiple permissions
hasAny := rbac.HasAnyPermission(role, PermWalletRead, PermAdminRead)

// Check if role has ALL permissions
hasAll := rbac.HasAllPermissions(role, PermWalletRead, PermPortfolioRead)

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
// gRPC server with permission checking
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