package api

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user's external ID.
	UserContextKey contextKey = "user_external_id"
)

// AuthConfig holds authentication middleware configuration.
type AuthConfig struct {
	// HeaderName is the header to extract the external ID from.
	// defaults to "X-User-External-ID" for the placeholder implementation.
	HeaderName string

	// Skipper defines a function to skip auth for certain routes.
	Skipper func(c echo.Context) bool
}

// DefaultAuthConfig returns the default auth configuration.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		HeaderName: "X-User-External-ID",
		Skipper:    nil,
	}
}

// AuthMiddleware creates a placeholder authentication middleware.
// in production, this would validate JWT tokens or integrate with an auth provider.
// for now, it extracts the external user ID from a header.
//
// placeholder behavior:
// - extracts external_id from the configured header
// - stores it in context for downstream handlers
// - returns 401 if header is missing on protected routes
func AuthMiddleware(config AuthConfig) echo.MiddlewareFunc {
	if config.HeaderName == "" {
		config.HeaderName = DefaultAuthConfig().HeaderName
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// check if we should skip auth for this route
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// extract external ID from header
			externalID := strings.TrimSpace(c.Request().Header.Get(config.HeaderName))
			if externalID == "" {
				return echo.NewHTTPError(401, "missing authentication: "+config.HeaderName+" header required")
			}

			// store in context for downstream handlers
			c.Set(string(UserContextKey), externalID)

			return next(c)
		}
	}
}

// OptionalAuthMiddleware extracts user context if present but doesn't require it.
// useful for endpoints that work for both authenticated and anonymous users.
func OptionalAuthMiddleware(config AuthConfig) echo.MiddlewareFunc {
	if config.HeaderName == "" {
		config.HeaderName = DefaultAuthConfig().HeaderName
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// extract external ID from header if present
			externalID := strings.TrimSpace(c.Request().Header.Get(config.HeaderName))
			if externalID != "" {
				c.Set(string(UserContextKey), externalID)
			}

			return next(c)
		}
	}
}

// GetUserExternalID retrieves the authenticated user's external ID from context.
// returns empty string if not authenticated.
func GetUserExternalID(c echo.Context) string {
	if val := c.Get(string(UserContextKey)); val != nil {
		if externalID, ok := val.(string); ok {
			return externalID
		}
	}
	return ""
}

// PublicRoutesSkipper returns a skipper function that skips auth for public routes.
func PublicRoutesSkipper(publicPaths ...string) func(echo.Context) bool {
	pathSet := make(map[string]bool)
	for _, p := range publicPaths {
		pathSet[p] = true
	}

	return func(c echo.Context) bool {
		return pathSet[c.Path()]
	}
}
