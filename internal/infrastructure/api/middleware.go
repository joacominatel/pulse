package api

import (
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/infrastructure/auth"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user's external ID (sub claim).
	UserContextKey contextKey = "user_external_id"

	// ClaimsContextKey is the context key for the full JWT claims.
	ClaimsContextKey contextKey = "jwt_claims"
)

// AuthConfig holds authentication middleware configuration.
type AuthConfig struct {
	// JWTValidator is the validator for supabase JWT tokens.
	JWTValidator *auth.JWTValidator

	// Skipper defines a function to skip auth for certain routes.
	Skipper func(c echo.Context) bool
}

// AuthMiddleware creates a JWT authentication middleware using supabase tokens.
// validates the Authorization header (Bearer token) and extracts user claims.
//
// behavior:
// - extracts JWT from Authorization header
// - validates signature and expiration
// - stores user_id (sub claim) and full claims in context
// - returns 401 if token is missing or invalid on protected routes
func AuthMiddleware(config AuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// check if we should skip auth for this route
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// extract and validate JWT
			claims, err := validateRequest(c, config.JWTValidator)
			if err != nil {
				return mapAuthError(err)
			}

			// store in context for downstream handlers
			c.Set(string(UserContextKey), claims.UserID())
			c.Set(string(ClaimsContextKey), claims)

			return next(c)
		}
	}
}

// OptionalAuthMiddleware extracts user context if present but doesn't require it.
// useful for endpoints that work for both authenticated and anonymous users.
func OptionalAuthMiddleware(config AuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// try to validate JWT if present
			claims, err := validateRequest(c, config.JWTValidator)
			if err == nil && claims != nil {
				c.Set(string(UserContextKey), claims.UserID())
				c.Set(string(ClaimsContextKey), claims)
			}
			// continue regardless of auth status
			return next(c)
		}
	}
}

// validateRequest extracts and validates the JWT from the request
func validateRequest(c echo.Context, validator *auth.JWTValidator) (*auth.SupabaseClaims, error) {
	if validator == nil {
		return nil, auth.ErrMissingToken
	}

	authHeader := c.Request().Header.Get("Authorization")
	token := auth.ExtractBearerToken(authHeader)

	return validator.ValidateToken(token)
}

// mapAuthError converts auth errors to appropriate HTTP errors
func mapAuthError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, auth.ErrMissingToken):
		return echo.NewHTTPError(401, "missing authentication: Authorization header required")
	case errors.Is(err, auth.ErrTokenExpired):
		return echo.NewHTTPError(401, "token expired")
	case errors.Is(err, auth.ErrInvalidSignature):
		return echo.NewHTTPError(401, "invalid token signature")
	case errors.Is(err, auth.ErrInvalidToken):
		return echo.NewHTTPError(401, "invalid token format")
	case errors.Is(err, auth.ErrInvalidClaims):
		return echo.NewHTTPError(401, "invalid token claims")
	default:
		return echo.NewHTTPError(401, "authentication failed")
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

// GetClaims retrieves the full JWT claims from context.
// returns nil if not authenticated.
func GetClaims(c echo.Context) *auth.SupabaseClaims {
	if val := c.Get(string(ClaimsContextKey)); val != nil {
		if claims, ok := val.(*auth.SupabaseClaims); ok {
			return claims
		}
	}
	return nil
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
