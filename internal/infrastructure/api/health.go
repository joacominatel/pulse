package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// HealthResponse is the response for health check endpoints.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// RegisterHealthRoutes registers health check endpoints.
// these are public and don't require authentication.
func RegisterHealthRoutes(e *echo.Echo) {
	e.GET("/health", healthHandler)
	e.GET("/ready", readyHandler)
}

// healthHandler returns the basic health status.
// used for liveness probes.
func healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{
		Status:  "healthy",
		Service: "pulse",
	})
}

// readyHandler returns the readiness status.
// used for readiness probes. in a full implementation,
// this would check database connectivity and other dependencies.
func readyHandler(c echo.Context) error {
	// placeholder: always ready for now
	// production would check db.HealthCheck() here
	return c.JSON(http.StatusOK, HealthResponse{
		Status:  "ready",
		Service: "pulse",
	})
}
