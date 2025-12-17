package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:            ":8080",
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// Server wraps the Echo instance and provides lifecycle management.
type Server struct {
	echo   *echo.Echo
	config ServerConfig
	logger *logging.Logger
}

// NewServer creates a new HTTP server with Echo.
func NewServer(config ServerConfig, logger *logging.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// configure base middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(requestLogger(logger))

	// configure CORS for api access
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// custom error handler
	e.HTTPErrorHandler = customErrorHandler(logger)

	return &Server{
		echo:   e,
		config: config,
		logger: logger.WithComponent("http_server"),
	}
}

// Echo returns the underlying Echo instance for route registration.
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// Start begins listening for HTTP requests.
// blocks until the server is stopped.
func (s *Server) Start() error {
	s.logger.Info("http server starting",
		"port", s.config.Port,
		"read_timeout", s.config.ReadTimeout.String(),
		"write_timeout", s.config.WriteTimeout.String(),
	)

	server := &http.Server{
		Addr:         s.config.Port,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	if err := s.echo.StartServer(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.echo.Shutdown(ctx)
}

// requestLogger creates a middleware that logs requests using our structured logger.
func requestLogger(logger *logging.Logger) echo.MiddlewareFunc {
	l := logger.WithComponent("http")

	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		LogLatency:  true,
		LogMethod:   true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error != nil {
				l.Warn("request error",
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
					"error", v.Error.Error(),
					"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
				)
			} else {
				l.Info("request",
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
					"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
				)
			}
			return nil
		},
	})
}

// customErrorHandler provides consistent error responses.
func customErrorHandler(logger *logging.Logger) echo.HTTPErrorHandler {
	l := logger.WithComponent("http_error")

	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		var he *echo.HTTPError
		if errors.As(err, &he) {
			if he.Internal != nil {
				if herr, ok := he.Internal.(*echo.HTTPError); ok {
					he = herr
				}
			}
		} else {
			he = echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		code := he.Code
		message := he.Message

		// log server errors
		if code >= 500 {
			l.Error("server error",
				"status", code,
				"error", err.Error(),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
			)
		}

		// send json response
		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				err = c.NoContent(code)
			} else {
				err = c.JSON(code, ErrorResponse{
					Error:   http.StatusText(code),
					Message: message,
				})
			}
			if err != nil {
				l.Error("failed to send error response", "error", err.Error())
			}
		}
	}
}

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message any    `json:"message"`
}
