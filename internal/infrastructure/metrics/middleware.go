package metrics

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// Middleware returns an Echo middleware that records HTTP request metrics.
func Middleware(m *Metrics) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// process request
			err := next(c)

			// record duration after request completes
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(c.Response().Status)
			method := c.Request().Method
			path := normalizePath(c)

			m.RecordHTTPRequest(method, path, status, duration)

			return err
		}
	}
}

// normalizePath extracts the route pattern rather than the actual path
// to prevent high cardinality labels from things like IDs.
// e.g. /api/v1/communities/123 becomes /api/v1/communities/:id
func normalizePath(c echo.Context) string {
	// use the matched route pattern if available
	if path := c.Path(); path != "" {
		return path
	}
	// fallback to request path for unmatched routes (404s, etc)
	return c.Request().URL.Path
}
