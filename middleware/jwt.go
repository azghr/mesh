// Package middleware provides HTTP middleware for Fiber applications.
//
// This package includes JWT authentication, RBAC authorization, CORS, security
// headers, rate limiting, and other common HTTP middleware.
//
// # JWT Middleware
//
// The JWT middleware validates tokens and extracts claims into context.
// It supports multiple token locations (header, query, cookie) and integrates
// with the auth package for token management.
//
// # Basic Usage
//
// Configure JWT middleware:
//
//	jwtMiddleware := middleware.JWT(middleware.JWTConfig{
//	    JWTManager:  jwtManager,
//	    TokenLookup: "header:Authorization",
//	    ErrorHandler: func(c *fiber.Ctx) error {
//	        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
//	            "error": "invalid token",
//	        })
//	    },
//	})
//
//	app.Use(jwtMiddleware)
//
// Token claims are available in handlers:
//
//	func handler(c *fiber.Ctx) error {
//	    claims := c.Locals("jwt_claims").(*auth.Claims)
//	    userID := claims.UserID
//	    // ...
//	}
//
// # Permission Middleware
//
// Require permission checks:
//
//	app.Get("/admin", middleware.RequirePermission("admin:read"), adminHandler)
//
// Require any of multiple permissions:
//
//	app.Get("/resource", middleware.RequireAnyPermission("resource:read", "resource:write"), handler)
//
// Require role:
//
//	app.Get("/dashboard", middleware.RequireRole("admin", "manager"), dashboardHandler)
//
// # Token Lookup Format
//
// Token location format: "source:key"
//   - "header:Authorization" - Bearer token in header
//   - "query:token" - Token in query parameter
//   - "cookie:session" - Token in cookie
//
// # Best Practices
//
//   - Place JWT middleware early in the middleware chain
//   - Use RequirePermission after JWT validation
//   - Return structured error responses
//   - Log failed authentication attempts
//   - Use HTTPS in production

package middleware

import (
	"strings"

	"github.com/azghr/mesh/auth"
	"github.com/gofiber/fiber/v2"
)

type JWTConfig struct {
	JWTManager     *auth.JWTManager
	TokenLookup    string
	Extractor      func(*fiber.Ctx) string
	SuccessHandler fiber.Handler
	ErrorHandler   fiber.Handler
}

func JWT(config JWTConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractJWTToken(c, config.TokenLookup, config.Extractor)

		if token == "" {
			if config.ErrorHandler != nil {
				return config.ErrorHandler(c)
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing or invalid token",
				"code":  "UNAUTHORIZED",
			})
		}

		claims, err := config.JWTManager.ValidateToken(token)
		if err != nil {
			if config.ErrorHandler != nil {
				return config.ErrorHandler(c)
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
				"code":  "INVALID_TOKEN",
			})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("roles", claims.Roles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("tenant_id", claims.TenantID)
		c.Locals("jwt_claims", claims)

		if config.SuccessHandler != nil {
			return config.SuccessHandler(c)
		}

		return c.Next()
	}
}

func extractJWTToken(c *fiber.Ctx, tokenLookup string, extractor func(*fiber.Ctx) string) string {
	if extractor != nil {
		return extractor(c)
	}

	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return ""
	}

	source := strings.ToLower(parts[0])
	key := parts[1]

	switch source {
	case "header":
		authHeader := c.Get(key)
		if authHeader == "" {
			return ""
		}
		authParts := strings.Split(authHeader, " ")
		if len(authParts) != 2 || strings.ToLower(authParts[0]) != "bearer" {
			return ""
		}
		return authParts[1]
	case "query":
		return c.Query(key)
	case "cookie":
		cookie := c.Cookies(key)
		return cookie
	default:
		return ""
	}
}

type RequirePermissionConfig struct {
	Permission  string
	PrefixMatch bool
}

func RequirePermission(perm string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		permissions, ok := c.Locals("permissions").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "no permissions found",
				"code":  "FORBIDDEN",
			})
		}

		for _, p := range permissions {
			if p == perm || (strings.HasPrefix(p, perm+":") && len(perm) > 0) {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
			"code":  "INSUFFICIENT_PERMISSIONS",
		})
	}
}

func RequireAnyPermission(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userPerms, ok := c.Locals("permissions").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "no permissions found",
				"code":  "FORBIDDEN",
			})
		}

		for _, userPerm := range userPerms {
			for _, needPerm := range perms {
				if userPerm == needPerm || strings.HasPrefix(userPerm, needPerm+":") {
					return c.Next()
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient permissions",
			"code":  "INSUFFICIENT_PERMISSIONS",
		})
	}
}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles, ok := c.Locals("roles").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "no roles found",
				"code":  "FORBIDDEN",
			})
		}

		for _, userRole := range userRoles {
			for _, needRole := range roles {
				if userRole == needRole {
					return c.Next()
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "insufficient role",
			"code":  "INSUFFICIENT_ROLE",
		})
	}
}
