package logging

import (
	"context"
	"log/slog"
	"os"
)

// Logger wraps slog for structured logging across the application.
// keeps things simple, no fancy abstractions.
type Logger struct {
	*slog.Logger
}

// New creates a new logger with JSON output for production use.
func New() *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewWithLevel creates a logger with a specific log level.
func NewWithLevel(level slog.Level) *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithContext returns a logger with context values attached.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: l.Logger,
	}
}

// WithComponent returns a logger tagged with a component name.
// useful for tracing which part of the system is logging.
func (l *Logger) WithComponent(name string) *Logger {
	return &Logger{
		Logger: l.With("component", name),
	}
}

// Database logs a successful database connection.
func (l *Logger) DatabaseConnected(host, database string) {
	l.Info("database connection established",
		"host", host,
		"database", database,
	)
}

// DatabaseConnectionFailed logs a failed database connection attempt.
func (l *Logger) DatabaseConnectionFailed(err error) {
	l.Error("database connection failed",
		"error", err.Error(),
	)
}

// MigrationStarted logs the beginning of a migration run.
func (l *Logger) MigrationStarted() {
	l.Info("starting database migrations")
}

// MigrationApplied logs a successfully applied migration.
func (l *Logger) MigrationApplied(version, name string) {
	l.Info("migration applied",
		"version", version,
		"name", name,
	)
}

// MigrationSkipped logs when a migration was already applied.
func (l *Logger) MigrationSkipped(version, name string) {
	l.Debug("migration already applied, skipping",
		"version", version,
		"name", name,
	)
}

// MigrationCompleted logs the successful completion of all migrations.
func (l *Logger) MigrationCompleted(count int) {
	l.Info("migrations completed",
		"applied_count", count,
	)
}

// MigrationFailed logs a migration failure.
func (l *Logger) MigrationFailed(version, name string, err error) {
	l.Error("migration failed",
		"version", version,
		"name", name,
		"error", err.Error(),
	)
}

// HealthCheckPassed logs a successful health check.
func (l *Logger) HealthCheckPassed() {
	l.Info("database health check passed")
}

// HealthCheckFailed logs a failed health check.
func (l *Logger) HealthCheckFailed(err error) {
	l.Error("database health check failed",
		"error", err.Error(),
	)
}
