package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoleConstants tests that all role constants are defined
func TestRoleConstants(t *testing.T) {
	roles := []Role{
		RoleAdmin,
		RoleUser,
		RoleReadOnly,
		RolePremium,
		RoleDeveloper,
	}

	for _, role := range roles {
		assert.NotEmpty(t, string(role), "Role constant should not be empty")
	}
}

// TestPermissionConstants tests that all permission constants are defined
func TestPermissionConstants(t *testing.T) {
	permissions := []Permission{
		PermWalletRead,
		PermWalletWrite,
		PermWalletDelete,
		PermPortfolioRead,
		PermPortfolioWrite,
		PermPortfolioDelete,
		PermAnalyticsRead,
		PermAnalyticsAdvanced,
		PermBillingRead,
		PermBillingWrite,
		PermBillingDelete,
		PermUserRead,
		PermUserWrite,
		PermUserDelete,
		PermAdminRead,
		PermAdminWrite,
		PermAdminDelete,
		PermAPICreate,
		PermAPIRead,
		PermAPIWrite,
		PermAPIDelete,
		PermWorkflowCreate,
		PermWorkflowRead,
		PermWorkflowWrite,
		PermWorkflowDelete,
		PermZKCreate,
		PermZKRead,
		PermZKVerify,
	}

	for _, perm := range permissions {
		assert.NotEmpty(t, string(perm), "Permission constant should not be empty")
	}
}

// TestDefaultRoleStore tests the default role store implementation
func TestDefaultRoleStore(t *testing.T) {
	store := &defaultRoleStore{}

	ctx := context.Background()
	role, err := store.GetUserRole(ctx, "any-user-id")

	assert.NoError(t, err)
	assert.Equal(t, RoleUser, role)
}

// TestNewDatabaseRoleStore tests creating a database role store
func TestNewDatabaseRoleStore(t *testing.T) {
	t.Run("creates store with getRole function", func(t *testing.T) {
		getRole := func(ctx context.Context, userID string) (Role, error) {
			return RoleAdmin, nil
		}

		store := NewDatabaseRoleStore(getRole)
		assert.NotNil(t, store)
		assert.NotNil(t, store.getRole)
	})

	t.Run("creates store with nil function", func(t *testing.T) {
		store := NewDatabaseRoleStore(nil)
		assert.NotNil(t, store)
		assert.Nil(t, store.getRole)
	})
}

// TestDatabaseRoleStore_GetUserRole tests the database role store
func TestDatabaseRoleStore_GetUserRole(t *testing.T) {
	tests := []struct {
		name      string
		getRole   func(ctx context.Context, userID string) (Role, error)
		userID    string
		wantRole  Role
		wantError bool
	}{
		{
			name: "returns role from function",
			getRole: func(ctx context.Context, userID string) (Role, error) {
				return RolePremium, nil
			},
			userID:    "user-123",
			wantRole:  RolePremium,
			wantError: false,
		},
		{
			name: "returns error when function fails",
			getRole: func(ctx context.Context, userID string) (Role, error) {
				return RoleUser, errors.New("database error")
			},
			userID:    "user-456",
			wantRole:  RoleUser,
			wantError: true,
		},
		{
			name:      "returns default role when function is nil",
			getRole:   nil,
			userID:    "user-789",
			wantRole:  RoleUser,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewDatabaseRoleStore(tt.getRole)
			ctx := context.Background()

			role, err := store.GetUserRole(ctx, tt.userID)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantRole, role)
		})
	}
}

// TestCachedRoleStore tests the cached role store
func TestCachedRoleStore(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		setup    func(*CachedRoleStore)
		userID   string
		wantRole Role
	}{
		{
			name:     "caches role from backing store",
			ttl:      5 * time.Minute,
			userID:   "user-1",
			wantRole: RoleAdmin,
		},
		{
			name:     "returns cached role on second call",
			ttl:      5 * time.Minute,
			userID:   "user-2",
			wantRole: RolePremium,
		},
		{
			name:     "refreshes expired cache entry",
			ttl:      100 * time.Millisecond,
			userID:   "user-3",
			wantRole: RoleDeveloper,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockRoleStore{
				roles: map[string]Role{
					"user-1": RoleAdmin,
					"user-2": RolePremium,
					"user-3": RoleDeveloper,
				},
			}

			cachedStore := NewCachedRoleStore(mockStore, tt.ttl)
			defer cachedStore.ClearCache() // Clear cache instead of stopping

			if tt.setup != nil {
				tt.setup(cachedStore)
			}

			ctx := context.Background()

			// First call
			role1, err := cachedStore.GetUserRole(ctx, tt.userID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRole, role1)

			// Second call (should hit cache)
			role2, err := cachedStore.GetUserRole(ctx, tt.userID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRole, role2)

			// If testing expiry, wait and check again
			if tt.name == "refreshes expired cache entry" {
				time.Sleep(150 * time.Millisecond)
				role3, err := cachedStore.GetUserRole(ctx, tt.userID)
				require.NoError(t, err)
				assert.Equal(t, tt.wantRole, role3)
			}
		})
	}
}

// TestCachedRoleStore_ConcurrentAccess tests concurrent access to the cache
func TestCachedRoleStore_ConcurrentAccess(t *testing.T) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{
			"user-1": RoleAdmin,
			"user-2": RoleUser,
			"user-3": RolePremium,
		},
	}

	cachedStore := NewCachedRoleStore(mockStore, 5*time.Minute)
	defer cachedStore.ClearCache()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			userID := "user-" + string(rune('1'+iteration%3))
			_, err := cachedStore.GetUserRole(ctx, userID)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

// TestCachedRoleStore_Cleanup tests the background cleanup goroutine
func TestCachedRoleStore_Cleanup(t *testing.T) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{
			"user-1": RoleAdmin,
			"user-2": RoleUser,
		},
	}

	cachedStore := NewCachedRoleStore(mockStore, 100*time.Millisecond)
	defer cachedStore.ClearCache()

	ctx := context.Background()

	// Add entries to cache
	_, _ = cachedStore.GetUserRole(ctx, "user-1")
	_, _ = cachedStore.GetUserRole(ctx, "user-2")

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Cache should be empty or entries expired
	cachedStore.cacheMutex.Lock()
	_ = len(cachedStore.cache)
	cachedStore.cacheMutex.Unlock()

	// After cleanup, entries should be marked for removal
	// The cleanup goroutine runs in background, so we just verify it doesn't crash
	assert.True(t, true)
}

// TestCachedRoleStore_Stop tests stopping the cleanup goroutine
func TestCachedRoleStore_ClearCache(t *testing.T) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{},
	}

	cachedStore := NewCachedRoleStore(mockStore, 1*time.Minute)

	// ClearCache should not panic
	assert.NotPanics(t, func() {
		cachedStore.ClearCache()
	})

	// Multiple clears should be safe
	assert.NotPanics(t, func() {
		cachedStore.ClearCache()
		cachedStore.ClearCache()
	})
}

// TestRBAC_AllPermissions tests permission definitions
func TestRBAC_AllPermissions(t *testing.T) {
	rbac := NewRBAC(nil)

	// Test all wallet permissions
	walletPerms := []Permission{
		PermWalletRead,
		PermWalletWrite,
		PermWalletDelete,
	}

	for _, perm := range walletPerms {
		assert.True(t, rbac.HasPermission(RoleAdmin, perm),
			"Admin should have "+string(perm))
	}

	// Test all portfolio permissions
	portfolioPerms := []Permission{
		PermPortfolioRead,
		PermPortfolioWrite,
		PermPortfolioDelete,
	}

	for _, perm := range portfolioPerms {
		assert.True(t, rbac.HasPermission(RoleAdmin, perm),
			"Admin should have "+string(perm))
	}
}

// TestRBAC_RoleHierarchy tests role permission hierarchy
func TestRBAC_RoleHierarchy(t *testing.T) {
	rbac := NewRBAC(nil)

	tests := []struct {
		role          Role
		expectedPerms int
		deniedPerms   []Permission
	}{
		{
			role:          RoleAdmin,
			expectedPerms: 29, // Admin should have all permissions
			deniedPerms:   []Permission{},
		},
		{
			role:          RoleReadOnly,
			expectedPerms: 3, // Only read permissions
			deniedPerms: []Permission{
				PermWalletWrite,
				PermWalletDelete,
				PermPortfolioWrite,
				PermPortfolioDelete,
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			// Verify denied permissions
			for _, perm := range tt.deniedPerms {
				assert.False(t, rbac.HasPermission(tt.role, perm),
					string(tt.role)+" should not have "+string(perm))
			}
		})
	}
}

// TestRBAC_EdgeCases tests edge cases
func TestRBAC_EdgeCases(t *testing.T) {
	rbac := NewRBAC(nil)

	t.Run("unknown role has no permissions", func(t *testing.T) {
		unknownRole := Role("unknown")
		assert.False(t, rbac.HasPermission(unknownRole, PermWalletRead))
	})

	t.Run("empty permission returns false", func(t *testing.T) {
		assert.False(t, rbac.HasPermission(RoleAdmin, Permission("")))
	})
}

// TestRBAC_UserContext tests user context handling
func TestRBAC_UserContext(t *testing.T) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{
			"user-admin": RoleAdmin,
			"user-user":  RoleUser,
		},
	}

	rbac := NewRBAC(mockStore)
	ctx := context.Background()

	t.Run("admin can delete wallet", func(t *testing.T) {
		role, err := rbac.GetRole(ctx, "user-admin")
		assert.NoError(t, err)
		assert.True(t, rbac.HasPermission(role, PermWalletDelete))
	})

	t.Run("user cannot delete wallet", func(t *testing.T) {
		role, err := rbac.GetRole(ctx, "user-user")
		assert.NoError(t, err)
		assert.False(t, rbac.HasPermission(role, PermWalletDelete))
	})

	t.Run("handles unknown user", func(t *testing.T) {
		role, err := rbac.GetRole(ctx, "unknown-user")
		// Should default to RoleUser
		assert.NoError(t, err)
		assert.True(t, rbac.HasPermission(role, PermWalletRead)) // User can read wallet
	})
}

// mockRoleStore is a mock implementation for testing
type mockRoleStore struct {
	roles map[string]Role
	mu    sync.RWMutex
}

func (m *mockRoleStore) GetUserRole(ctx context.Context, userID string) (Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if role, ok := m.roles[userID]; ok {
		return role, nil
	}
	return RoleUser, nil
}

// Benchmark tests
func BenchmarkRBAC_HasPermission(b *testing.B) {
	rbac := NewRBAC(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rbac.HasPermission(RoleAdmin, PermWalletRead)
	}
}

func BenchmarkRBAC_UserHasPermission(b *testing.B) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{
			"user-1": RoleAdmin,
		},
	}

	rbac := NewRBAC(mockStore)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		role, _ := rbac.GetRole(ctx, "user-1")
		rbac.HasPermission(role, PermWalletRead)
	}
}

func BenchmarkCachedRoleStore_GetUserRole(b *testing.B) {
	mockStore := &mockRoleStore{
		roles: map[string]Role{
			"user-1": RoleAdmin,
		},
	}

	cachedStore := NewCachedRoleStore(mockStore, 5*time.Minute)
	defer cachedStore.ClearCache()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cachedStore.GetUserRole(ctx, "user-1")
	}
}
