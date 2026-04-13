package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBAC_HasPermission(t *testing.T) {
	rbac := NewRBAC(nil)

	tests := []struct {
		name      string
		role      Role
		permission Permission
		expected  bool
	}{
		{"Admin can delete wallet", RoleAdmin, PermWalletDelete, true},
		{"User can read wallet", RoleUser, PermWalletRead, true},
		{"User cannot delete wallet", RoleUser, PermWalletDelete, false},
		{"ReadOnly can read analytics", RoleReadOnly, PermAnalyticsRead, true},
		{"ReadOnly cannot write portfolio", RoleReadOnly, PermPortfolioWrite, false},
		{"Premium has advanced analytics", RolePremium, PermAnalyticsAdvanced, true},
		{"User does not have advanced analytics", RoleUser, PermAnalyticsAdvanced, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rbac.HasPermission(tt.role, tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBAC_HasAnyPermission(t *testing.T) {
	rbac := NewRBAC(nil)

	tests := []struct {
		name       string
		role       Role
		permissions []Permission
		expected   bool
	}{
		{
			"User has read or write",
			RoleUser,
			[]Permission{PermWalletRead, PermWalletWrite},
			true,
		},
		{
			"ReadOnly doesn't have write or delete",
			RoleReadOnly,
			[]Permission{PermWalletWrite, PermWalletDelete},
			false,
		},
		{
			"Admin has all requested permissions",
			RoleAdmin,
			[]Permission{PermAdminRead, PermAdminWrite, PermAdminDelete},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rbac.HasAnyPermission(tt.role, tt.permissions...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBAC_HasAllPermissions(t *testing.T) {
	rbac := NewRBAC(nil)

	tests := []struct {
		name       string
		role       Role
		permissions []Permission
		expected   bool
	}{
		{
			"User has both read and write wallet",
			RoleUser,
			[]Permission{PermWalletRead, PermWalletWrite},
			true,
		},
		{
			"User doesn't have all admin permissions",
			RoleUser,
			[]Permission{PermAdminRead, PermAdminWrite, PermAdminDelete},
			false,
		},
		{
			"Admin has all wallet permissions",
			RoleAdmin,
			[]Permission{PermWalletRead, PermWalletWrite, PermWalletDelete},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rbac.HasAllPermissions(tt.role, tt.permissions...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBAC_CheckPermission(t *testing.T) {
	rbac := NewRBAC(nil)
	ctx := context.Background()

	t.Run("User has required permission", func(t *testing.T) {
		err := rbac.CheckPermission(ctx, "user123", PermWalletRead)
		assert.NoError(t, err)
	})

	t.Run("User doesn't have required permission", func(t *testing.T) {
		err := rbac.CheckPermission(ctx, "user123", PermWalletDelete)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not have permission")
	})
}

func TestRBAC_AddPermissionToRole(t *testing.T) {
	rbac := NewRBAC(nil)

	// ReadOnly initially cannot delete wallet
	assert.False(t, rbac.HasPermission(RoleReadOnly, PermWalletDelete))

	// Add delete permission
	rbac.AddPermissionToRole(RoleReadOnly, PermWalletDelete)
	assert.True(t, rbac.HasPermission(RoleReadOnly, PermWalletDelete))
}

func TestRBAC_RemovePermissionFromRole(t *testing.T) {
	rbac := NewRBAC(nil)

	// User initially can write wallet
	assert.True(t, rbac.HasPermission(RoleUser, PermWalletWrite))

	// Remove write permission
	rbac.RemovePermissionFromRole(RoleUser, PermWalletWrite)
	assert.False(t, rbac.HasPermission(RoleUser, PermWalletWrite))
}

func TestRBAC_SetRolePermissions(t *testing.T) {
	rbac := NewRBAC(nil)

	// Set custom permissions for a role
	customPerms := []Permission{
		PermWalletRead,
		PermPortfolioRead,
		PermAnalyticsRead,
	}
	rbac.SetRolePermissions(RoleReadOnly, customPerms)

	// Verify all custom permissions are present
	for _, perm := range customPerms {
		assert.True(t, rbac.HasPermission(RoleReadOnly, perm))
	}

	// Verify other permissions are not present
	assert.False(t, rbac.HasPermission(RoleReadOnly, PermWalletWrite))
}

func TestRBAC_GetUserPermissions(t *testing.T) {
	rbac := NewRBAC(nil)
	ctx := context.Background()

	permissions, err := rbac.GetUserPermissions(ctx, "user123")
	require.NoError(t, err)
	assert.NotEmpty(t, permissions)

	// User should have read permissions
	assert.Contains(t, permissions, PermWalletRead)
}
