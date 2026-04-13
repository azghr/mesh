package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
)

// Role represents a user role in the system
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleUser      Role = "user"
	RoleReadOnly  Role = "readonly"
	RolePremium   Role = "premium"
	RoleDeveloper Role = "developer"
)

// Permission represents a specific permission in the system
type Permission string

const (
	// Wallet permissions
	PermWalletRead   Permission = "wallet:read"
	PermWalletWrite  Permission = "wallet:write"
	PermWalletDelete Permission = "wallet:delete"

	// Portfolio permissions
	PermPortfolioRead   Permission = "portfolio:read"
	PermPortfolioWrite  Permission = "portfolio:write"
	PermPortfolioDelete Permission = "portfolio:delete"

	// Analytics permissions
	PermAnalyticsRead     Permission = "analytics:read"
	PermAnalyticsAdvanced Permission = "analytics:advanced"

	// Billing permissions
	PermBillingRead   Permission = "billing:read"
	PermBillingWrite  Permission = "billing:write"
	PermBillingDelete Permission = "billing:delete"

	// User permissions
	PermUserRead   Permission = "user:read"
	PermUserWrite  Permission = "user:write"
	PermUserDelete Permission = "user:delete"

	// Admin permissions
	PermAdminRead   Permission = "admin:read"
	PermAdminWrite  Permission = "admin:write"
	PermAdminDelete Permission = "admin:delete"

	// API permissions
	PermAPICreate Permission = "api:create"
	PermAPIRead   Permission = "api:read"
	PermAPIWrite  Permission = "api:write"
	PermAPIDelete Permission = "api:delete"

	// Workflow permissions
	PermWorkflowCreate Permission = "workflow:create"
	PermWorkflowRead   Permission = "workflow:read"
	PermWorkflowWrite  Permission = "workflow:write"
	PermWorkflowDelete Permission = "workflow:delete"

	// ZK proof permissions
	PermZKCreate Permission = "zk:create"
	PermZKRead   Permission = "zk:read"
	PermZKVerify Permission = "zk:verify"
)

// defaultRoleStore provides a default implementation that returns RoleUser for all users
type defaultRoleStore struct{}

func (s *defaultRoleStore) GetUserRole(ctx context.Context, userID string) (Role, error) {
	return RoleUser, nil
}

// RoleStore defines the interface for retrieving user roles from a data source
type RoleStore interface {
	// GetUserRole retrieves the role for a user from the data source
	GetUserRole(ctx context.Context, userID string) (Role, error)
}

// DatabaseRoleStore implements RoleStore for database-backed role retrieval
type DatabaseRoleStore struct {
	// Query function to retrieve user role from database
	// In a real implementation, this would be a database connection or query builder
	getRole func(ctx context.Context, userID string) (Role, error)
}

// NewDatabaseRoleStore creates a new database-backed role store
func NewDatabaseRoleStore(getRole func(ctx context.Context, userID string) (Role, error)) *DatabaseRoleStore {
	return &DatabaseRoleStore{
		getRole: getRole,
	}
}

// GetUserRole retrieves the user role from the database
func (s *DatabaseRoleStore) GetUserRole(ctx context.Context, userID string) (Role, error) {
	if s.getRole == nil {
		return RoleUser, fmt.Errorf("role store not configured")
	}
	return s.getRole(ctx, userID)
}

// CachedRoleStore wraps a RoleStore with in-memory caching
type CachedRoleStore struct {
	store      RoleStore
	cache      map[string]cacheEntry
	cacheMutex sync.RWMutex
	ttl        time.Duration
}

type cacheEntry struct {
	role      Role
	expiresAt time.Time
}

// NewCachedRoleStore creates a new cached role store with the specified TTL
func NewCachedRoleStore(store RoleStore, ttl time.Duration) *CachedRoleStore {
	cachedStore := &CachedRoleStore{
		store: store,
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}

	// Start background cleanup goroutine
	go cachedStore.cleanup()

	return cachedStore
}

// GetUserRole retrieves the user role from cache or backing store
func (s *CachedRoleStore) GetUserRole(ctx context.Context, userID string) (Role, error) {
	// Check cache first
	s.cacheMutex.RLock()
	if entry, ok := s.cache[userID]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMutex.RUnlock()
		logger.GetGlobal().Debug("Role cache hit", "user_id", userID, "role", entry.role)
		return entry.role, nil
	}
	s.cacheMutex.RUnlock()

	// Cache miss or expired, fetch from backing store
	role, err := s.store.GetUserRole(ctx, userID)
	if err != nil {
		return RoleUser, fmt.Errorf("failed to get user role: %w", err)
	}

	// Update cache
	s.cacheMutex.Lock()
	s.cache[userID] = cacheEntry{
		role:      role,
		expiresAt: time.Now().Add(s.ttl),
	}
	s.cacheMutex.Unlock()

	logger.GetGlobal().Debug("Role cache miss - fetched from store", "user_id", userID, "role", role)
	return role, nil
}

// cleanup removes expired entries from the cache
func (s *CachedRoleStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cacheMutex.Lock()
		now := time.Now()
		for userID, entry := range s.cache {
			if now.After(entry.expiresAt) {
				delete(s.cache, userID)
			}
		}
		s.cacheMutex.Unlock()
	}
}

// ClearCache clears all cached roles
func (s *CachedRoleStore) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache = make(map[string]cacheEntry)
}

// InvalidateCache invalidates the cached role for a specific user
func (s *CachedRoleStore) InvalidateCache(userID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.cache, userID)
}

// RBAC handles role-based access control
type RBAC struct {
	roleStore       RoleStore
	rolePermissions map[Role][]Permission
}

// NewRBAC creates a new RBAC instance with default role permissions
// If a roleStore is provided, it will be used for database-backed role retrieval.
// If nil, a default in-memory role store will be used (returns RoleUser for all users).
func NewRBAC(roleStore RoleStore) *RBAC {
	// If no role store provided, create a default one that returns RoleUser
	if roleStore == nil {
		roleStore = &defaultRoleStore{}
	}

	return &RBAC{
		roleStore: roleStore,
		rolePermissions: map[Role][]Permission{
			RoleAdmin: {
				// Admin has all permissions
				PermWalletRead, PermWalletWrite, PermWalletDelete,
				PermPortfolioRead, PermPortfolioWrite, PermPortfolioDelete,
				PermAnalyticsRead, PermAnalyticsAdvanced,
				PermBillingRead, PermBillingWrite, PermBillingDelete,
				PermUserRead, PermUserWrite, PermUserDelete,
				PermAdminRead, PermAdminWrite, PermAdminDelete,
				PermAPICreate, PermAPIRead, PermAPIWrite, PermAPIDelete,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowWrite, PermWorkflowDelete,
				PermZKCreate, PermZKRead, PermZKVerify,
			},
			RoleDeveloper: {
				// Developers have broad access for development
				PermWalletRead, PermWalletWrite,
				PermPortfolioRead, PermPortfolioWrite,
				PermAnalyticsRead, PermAnalyticsAdvanced,
				PermBillingRead, PermBillingWrite,
				PermAPICreate, PermAPIRead, PermAPIWrite,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowWrite,
				PermZKCreate, PermZKRead, PermZKVerify,
			},
			RolePremium: {
				// Premium users have enhanced permissions
				PermWalletRead, PermWalletWrite,
				PermPortfolioRead, PermPortfolioWrite,
				PermAnalyticsRead, PermAnalyticsAdvanced,
				PermBillingRead, PermBillingWrite,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowWrite,
				PermZKCreate, PermZKRead, PermZKVerify,
			},
			RoleUser: {
				// Regular users have basic permissions
				PermWalletRead, PermWalletWrite,
				PermPortfolioRead, PermPortfolioWrite,
				PermAnalyticsRead,
				PermBillingRead,
				PermWorkflowCreate, PermWorkflowRead,
				PermZKCreate, PermZKRead,
			},
			RoleReadOnly: {
				// Read-only users can only read
				PermWalletRead,
				PermPortfolioRead,
				PermAnalyticsRead,
				PermBillingRead,
				PermWorkflowRead,
				PermZKRead,
			},
		},
	}
}

// HasPermission checks if a role has a specific permission
func (r *RBAC) HasPermission(role Role, perm Permission) bool {
	permissions, exists := r.rolePermissions[role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if a role has any of the specified permissions
func (r *RBAC) HasAnyPermission(role Role, perms ...Permission) bool {
	for _, perm := range perms {
		if r.HasPermission(role, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if a role has all of the specified permissions
func (r *RBAC) HasAllPermissions(role Role, perms ...Permission) bool {
	for _, perm := range perms {
		if !r.HasPermission(role, perm) {
			return false
		}
	}
	return true
}

// GetRole retrieves the role for a user from the role store
// It uses the configured RoleStore (database, cache, etc.) for role lookup
func (r *RBAC) GetRole(ctx context.Context, userID string) (Role, error) {
	if r.roleStore == nil {
		return RoleUser, nil
	}

	role, err := r.roleStore.GetUserRole(ctx, userID)
	if err != nil {
		logger.GetGlobal().Error("Failed to retrieve user role", "user_id", userID, "error", err)
		// Return default role on error
		return RoleUser, nil
	}

	return role, nil
}

// SetRoleStore sets the role store for role retrieval
func (r *RBAC) SetRoleStore(store RoleStore) {
	r.roleStore = store
}

// InvalidateRoleCache invalidates any cached role data for a specific user
// This is useful when a user's role changes
func (r *RBAC) InvalidateRoleCache(userID string) {
	if cachedStore, ok := r.roleStore.(*CachedRoleStore); ok {
		cachedStore.InvalidateCache(userID)
	}
}

// ClearAllRoleCaches clears all cached role data
func (r *RBAC) ClearAllRoleCaches() {
	if cachedStore, ok := r.roleStore.(*CachedRoleStore); ok {
		cachedStore.ClearCache()
	}
}

// SetRolePermissions allows customizing permissions for a role
func (r *RBAC) SetRolePermissions(role Role, permissions []Permission) {
	r.rolePermissions[role] = permissions
}

// AddPermissionToRole adds a permission to a role
func (r *RBAC) AddPermissionToRole(role Role, perm Permission) {
	if _, exists := r.rolePermissions[role]; !exists {
		r.rolePermissions[role] = []Permission{}
	}
	r.rolePermissions[role] = append(r.rolePermissions[role], perm)
}

// RemovePermissionFromRole removes a permission from a role
func (r *RBAC) RemovePermissionFromRole(role Role, perm Permission) {
	permissions, exists := r.rolePermissions[role]
	if !exists {
		return
	}

	newPerms := make([]Permission, 0, len(permissions))
	for _, p := range permissions {
		if p != perm {
			newPerms = append(newPerms, p)
		}
	}
	r.rolePermissions[role] = newPerms
}

// GetUserPermissions retrieves all permissions for a user
func (r *RBAC) GetUserPermissions(ctx context.Context, userID string) ([]Permission, error) {
	role, err := r.GetRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	permissions, exists := r.rolePermissions[role]
	if !exists {
		return []Permission{}, nil
	}

	return permissions, nil
}

// CheckPermission checks if a user has a specific permission
func (r *RBAC) CheckPermission(ctx context.Context, userID string, perm Permission) error {
	role, err := r.GetRole(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}

	if !r.HasPermission(role, perm) {
		return fmt.Errorf("user %s does not have permission %s", userID, perm)
	}

	return nil
}
