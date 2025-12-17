package api

import (
	"github.com/labstack/echo/v4"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// RouterConfig holds dependencies for route registration.
type RouterConfig struct {
	IngestEventUseCase       *application.IngestEventUseCase
	CalculateMomentumUseCase *application.CalculateMomentumUseCase
	Logger                   *logging.Logger
}

// RegisterRoutes sets up all API routes on the server.
// follows RESTful conventions and groups routes logically.
func RegisterRoutes(e *echo.Echo, config RouterConfig) {
	// health endpoints (no auth required)
	RegisterHealthRoutes(e)

	// api v1 group with auth
	v1 := e.Group("/api/v1")

	// configure auth middleware with public routes skipper
	authConfig := DefaultAuthConfig()
	authConfig.Skipper = PublicRoutesSkipper(
		"/health",
		"/ready",
	)

	// apply optional auth to allow both authenticated and anonymous requests
	// individual handlers decide what to do with the user context
	v1.Use(OptionalAuthMiddleware(authConfig))

	// register domain handlers
	if config.IngestEventUseCase != nil {
		eventHandler := NewEventHandler(config.IngestEventUseCase)
		eventHandler.RegisterRoutes(v1)
	}

	if config.CalculateMomentumUseCase != nil {
		momentumHandler := NewMomentumHandler(config.CalculateMomentumUseCase)
		momentumHandler.RegisterRoutes(v1)
	}

	config.Logger.Info("api routes registered",
		"version", "v1",
		"health_endpoints", []string{"/health", "/ready"},
		"api_prefix", "/api/v1",
	)
}
