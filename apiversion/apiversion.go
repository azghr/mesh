// Package apiversion provides API version negotiation and routing.
//
// This package helps manage API versioning with:
// - URL path versioning (/v1/, /v2/)
// - Accept header versioning
// - Version-specific handlers
// - Default version handling
//
// Example:
//
//	r := apiversion.New()
//	r.V1().Get("/users", handlerV1)
//	r.V2().Get("/users", handlerV2)
//
//	// Or use Accept header
//	app.Use(apiversion.Middleware("v1"))
package apiversion

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	// DefaultVersion is the default API version
	DefaultVersion = "v1"
	// HeaderAccept is the Accept header name
	HeaderAccept = "Accept"
	// HeaderAPIVersion is the custom API version header
	HeaderAPIVersion = "X-API-Version"
)

// Config holds API versioning configuration.
type Config struct {
	Default string
	Prefix  string
}

// Option configures API versioning.
type Option func(*Config)

// WithDefault sets the default version.
func WithDefault(version string) Option {
	return func(c *Config) {
		c.Default = version
	}
}

// WithPrefix sets the URL prefix for versioned routes.
func WithPrefix(prefix string) Option {
	return func(c *Config) {
		c.Prefix = prefix
	}
}

// VersionRouter routes requests to version-specific handlers.
type VersionRouter struct {
	versions map[string]fiber.Router
	app      *fiber.App
	config   Config
}

// New creates a new version router.
func New(opts ...Option) *VersionRouter {
	cfg := Config{
		Default: DefaultVersion,
		Prefix:  "/v",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	app := fiber.New()

	vr := &VersionRouter{
		versions: make(map[string]fiber.Router),
		app:      app,
		config:   cfg,
	}

	return vr
}

// V1 returns the v1 router group.
func (r *VersionRouter) V1() fiber.Router {
	return r.Version("v1")
}

// V2 returns the v2 router group.
func (r *VersionRouter) V2() fiber.Router {
	return r.Version("v2")
}

// Version returns a router group for the specified version.
func (r *VersionRouter) Version(version string) fiber.Router {
	if group, ok := r.versions[version]; ok {
		return group
	}

	prefix := r.config.Prefix + version
	group := r.app.Group(prefix)
	r.versions[version] = group

	return group
}

// App returns the underlying Fiber app.
func (r *VersionRouter) App() *fiber.App {
	return r.app
}

// Middleware returns a Fiber middleware for version detection.
func Middleware(defaultVersion ...string) fiber.Handler {
	defaultVer := DefaultVersion
	if len(defaultVersion) > 0 && defaultVersion[0] != "" {
		defaultVer = defaultVersion[0]
	}

	return func(c *fiber.Ctx) error {
		version := detectVersion(c, defaultVer)
		c.Locals("api_version", version)
		return c.Next()
	}
}

// detectVersion determines the API version from request.
func detectVersion(c *fiber.Ctx, defaultVersion string) string {
	// Check X-API-Version header first
	if v := c.Get(HeaderAPIVersion); v != "" {
		return cleanVersion(v)
	}

	// Check Accept header
	if v := c.Get(HeaderAccept); v != "" {
		if version := parseAcceptHeader(v); version != "" {
			return version
		}
	}

	// Check URL path
	path := c.Path()
	if version := parsePathVersion(path); version != "" {
		return version
	}

	return defaultVersion
}

// parseAcceptHeader parses the Accept header for version.
func parseAcceptHeader(header string) string {
	// Look for application/vnd.mesh.v1+json or similar
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "+json") {
			// Look for version pattern like v1, v2 after "vnd.mesh."
			if idx := strings.Index(part, ".v"); idx >= 0 {
				version := part[idx+2:] // Skip ".v"
				if i := strings.Index(version, "+"); i >= 0 {
					version = version[:i]
				}
				if i := strings.Index(version, ";"); i >= 0 {
					version = version[:i]
				}
				if len(version) > 0 && version[0] >= '0' && version[0] <= '9' {
					return "v" + version
				}
			}
			// Fallback: look for v1, v2 pattern anywhere
			if idx := strings.Index(part, "v1"); idx >= 0 {
				if idx+1 < len(part) && part[idx+1] == '1' {
					return "v1"
				}
			}
			if idx := strings.Index(part, "v2"); idx >= 0 {
				if idx+1 < len(part) && part[idx+1] == '2' {
					return "v2"
				}
			}
		}
	}
	return ""
}

// parsePathVersion extracts version from URL path.
func parsePathVersion(path string) string {
	// Match /v1/, /v2/, etc.
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "v") {
			if len(part) > 1 && part[1] >= '0' && part[1] <= '9' {
				return cleanVersion(part)
			}
		}
	}
	return ""
}

// cleanVersion ensures version starts with 'v'.
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// GetVersion returns the API version from context.
func GetVersion(c *fiber.Ctx) string {
	if v, ok := c.Locals("api_version").(string); ok && v != "" {
		return v
	}
	return DefaultVersion
}

// Handler is a versioned handler function.
type Handler func(c *fiber.Ctx, version string) error

// VersionedHandler returns a middleware that passes version to handler.
func VersionedHandler(fn Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		version := GetVersion(c)
		return fn(c, version)
	}
}

// NewRouter creates a simple versioned router with path-based versioning.
func NewRouter() *fiber.App {
	app := fiber.New()

	app.Use(Middleware())

	return app
}

// WithVersionRoutes creates routes with version path prefix.
func WithVersionRoutes(app *fiber.App, versions []string, fn func(fiber.Router, string)) {
	for _, version := range versions {
		group := app.Group("/" + version)
		fn(group, version)
	}
}

// IsLatest checks if the version is the latest (highest).
func IsLatest(version string, available []string) bool {
	if len(available) == 0 || version == "" {
		return true
	}

	latest := available[len(available)-1]
	return version == latest
}

// SupportedVersions returns list of supported version strings.
func SupportedVersions(versions ...string) []string {
	return versions
}

// FromRequest extracts version from standard HTTP request.
func FromRequest(r *http.Request, defaultVersion string) string {
	// Check X-API-Version header
	if v := r.Header.Get(HeaderAPIVersion); v != "" {
		return cleanVersion(v)
	}

	// Check Accept header
	if v := r.Header.Get(HeaderAccept); v != "" {
		if version := parseAcceptHeader(v); version != "" {
			return version
		}
	}

	// Check URL path
	if version := parsePathVersion(r.URL.Path); version != "" {
		return version
	}

	return defaultVersion
}
