package api

import (
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/auth"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
	"github.com/joacominatel/pulse/internal/infrastructure/metrics"
)

// RouterConfig holds dependencies for route registration.
type RouterConfig struct {
	IngestEventUseCase       *application.IngestEventUseCase
	CalculateMomentumUseCase *application.CalculateMomentumUseCase
	CreateCommunityUseCase   *application.CreateCommunityUseCase
	CommunityRepo            domain.CommunityRepository
	JWTValidator             *auth.JWTValidator
	Logger                   *logging.Logger
	Metrics                  *metrics.Metrics
}

// RegisterRoutes sets up all API routes on the server.
// follows RESTful conventions and groups routes logically.
func RegisterRoutes(e *echo.Echo, config RouterConfig) {
	// prometheus metrics endpoint (no auth, standard scraping path)
	if config.Metrics != nil {
		e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(
			config.Metrics.Registry,
			promhttp.HandlerOpts{
				Registry:          config.Metrics.Registry,
				EnableOpenMetrics: true,
			},
		)))

		// apply metrics middleware to all routes
		e.Use(metrics.Middleware(config.Metrics))
	}

	// health endpoints (no auth required)
	RegisterHealthRoutes(e)

	// api v1 group with auth
	v1 := e.Group("/api/v1")

	// configure auth middleware with public routes skipper
	authConfig := AuthConfig{
		JWTValidator: config.JWTValidator,
		Skipper: PublicRoutesSkipper(
			"/health",
			"/ready",
		),
	}

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

	if config.CommunityRepo != nil {
		communityHandler := NewCommunityHandler(config.CommunityRepo, config.CreateCommunityUseCase)
		communityHandler.RegisterRoutes(v1)
	}

	metricsEnabled := config.Metrics != nil
	config.Logger.Info("api routes registered",
		"version", "v1",
		"health_endpoints", []string{"/health", "/ready"},
		"metrics_enabled", metricsEnabled,
		"api_prefix", "/api/v1",
	)
}
