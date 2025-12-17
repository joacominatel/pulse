package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joacominatel/pulse/internal/infrastructure/config"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

// Connection wraps a postgres connection pool.
// uses pgx for better performance and postgres-specific features.
type Connection struct {
	pool   *pgxpool.Pool
	config config.DatabaseConfig
	logger *logging.Logger
}

// New creates a new database connection.
func New(cfg config.DatabaseConfig, logger *logging.Logger) (*Connection, error) {
	componentLogger := logger.WithComponent("database")

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		componentLogger.DatabaseConnectionFailed(err)
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	// sensible pool defaults
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	// disable prepared statements for supabase transaction pooler (pgbouncer) compatibility
	// pgbouncer in transaction mode doesn't support prepared statements because
	// connections are recycled between transactions
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		componentLogger.DatabaseConnectionFailed(err)
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	conn := &Connection{
		pool:   pool,
		config: cfg,
		logger: componentLogger,
	}

	// verify connection works
	if err := conn.HealthCheck(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	componentLogger.DatabaseConnected(cfg.Host, cfg.Name)

	return conn, nil
}

// HealthCheck verifies the database connection is working.
func (c *Connection) HealthCheck(ctx context.Context) error {
	var result int
	err := c.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		c.logger.HealthCheckFailed(err)
		return fmt.Errorf("health check failed: %w", err)
	}

	c.logger.HealthCheckPassed()
	return nil
}

// Pool returns the underlying connection pool.
// needed for running migrations and queries.
func (c *Connection) Pool() *pgxpool.Pool {
	return c.pool
}

// Close shuts down the connection pool.
func (c *Connection) Close() {
	c.pool.Close()
	c.logger.Info("database connection closed")
}

// Schema returns the configured schema name.
func (c *Connection) Schema() string {
	return c.config.Schema
}
