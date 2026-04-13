package auth

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// RoleKey is the context key for storing user roles
	RoleKey ContextKey = "role"
	// UserIDKey is the context key for storing user IDs
	UserIDKey ContextKey = "user_id"
)

// RequirePermission creates an HTTP middleware that requires a specific permission
func (r *RBAC) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// Get user ID from context (set by auth middleware)
			userID, ok := ctx.Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, "unauthorized: missing user ID", http.StatusUnauthorized)
				return
			}

			// Check if user has the required permission
			if err := r.CheckPermission(ctx, userID, perm); err != nil {
				http.Error(w, "forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// RequireAnyPermission creates an HTTP middleware that requires any of the specified permissions
func (r *RBAC) RequireAnyPermission(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// Get user ID from context
			userID, ok := ctx.Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, "unauthorized: missing user ID", http.StatusUnauthorized)
				return
			}

			// Get user's role
			role, err := r.GetRole(ctx, userID)
			if err != nil {
				http.Error(w, "failed to get user role", http.StatusInternalServerError)
				return
			}

			// Check if user has any of the required permissions
			if !r.HasAnyPermission(role, perms...) {
				http.Error(w, "forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// RequireAllPermissions creates an HTTP middleware that requires all of the specified permissions
func (r *RBAC) RequireAllPermissions(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// Get user ID from context
			userID, ok := ctx.Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, "unauthorized: missing user ID", http.StatusUnauthorized)
				return
			}

			// Get user's role
			role, err := r.GetRole(ctx, userID)
			if err != nil {
				http.Error(w, "failed to get user role", http.StatusInternalServerError)
				return
			}

			// Check if user has all of the required permissions
			if !r.HasAllPermissions(role, perms...) {
				http.Error(w, "forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// RequireRole creates an HTTP middleware that requires a specific role
func (r *RBAC) RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// Get user ID from context
			userID, ok := ctx.Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, "unauthorized: missing user ID", http.StatusUnauthorized)
				return
			}

			// Get user's role
			userRole, err := r.GetRole(ctx, userID)
			if err != nil {
				http.Error(w, "failed to get user role", http.StatusInternalServerError)
				return
			}

			// Check if user has one of the required roles
			hasRequiredRole := false
			for _, role := range roles {
				if userRole == role {
					hasRequiredRole = true
					break
				}
			}

			if !hasRequiredRole {
				http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// GRPCRequirePermission creates a gRPC interceptor that requires a specific permission
func (r *RBAC) GRPCRequirePermission(perm Permission) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Get user ID from context (set by auth middleware)
		userID, ok := ctx.Value(UserIDKey).(string)
		if !ok || userID == "" {
			return nil, status.Error(codes.Unauthenticated, "missing user ID")
		}

		// Check if user has the required permission
		if err := r.CheckPermission(ctx, userID, perm); err != nil {
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		return handler(ctx, req)
	}
}

// GRPCRequireAnyPermission creates a gRPC interceptor that requires any of the specified permissions
func (r *RBAC) GRPCRequireAnyPermission(perms ...Permission) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Get user ID from context
		userID, ok := ctx.Value(UserIDKey).(string)
		if !ok || userID == "" {
			return nil, status.Error(codes.Unauthenticated, "missing user ID")
		}

		// Get user's role
		role, err := r.GetRole(ctx, userID)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get user role")
		}

		// Check if user has any of the required permissions
		if !r.HasAnyPermission(role, perms...) {
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		return handler(ctx, req)
	}
}

// GRPCRequireRole creates a gRPC interceptor that requires a specific role
func (r *RBAC) GRPCRequireRole(roles ...Role) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Get user ID from context
		userID, ok := ctx.Value(UserIDKey).(string)
		if !ok || userID == "" {
			return nil, status.Error(codes.Unauthenticated, "missing user ID")
		}

		// Get user's role
		userRole, err := r.GetRole(ctx, userID)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get user role")
		}

		// Check if user has one of the required roles
		hasRequiredRole := false
		for _, role := range roles {
			if userRole == role {
				hasRequiredRole = true
				break
			}
		}

		if !hasRequiredRole {
			return nil, status.Error(codes.PermissionDenied, "insufficient role")
		}

		return handler(ctx, req)
	}
}
