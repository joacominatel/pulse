package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joacominatel/pulse/internal/infrastructure/config"
	"github.com/joacominatel/pulse/internal/infrastructure/database"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

func main() {
	logger := logging.New()
	logger.Info("pulse starting up")

	if err := run(logger); err != nil {
		logger.Error("application failed", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *logging.Logger) error {
	// load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err.Error())
		return err
	}

	// establish database connection
	conn, err := database.New(cfg.Database, logger)
	if err != nil {
		return err
	}
	defer conn.Close()

	// run migrations
	migrator := database.NewMigrator(conn, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := migrator.Run(ctx); err != nil {
		return err
	}

	// verify health after migrations
	if err := conn.HealthCheck(ctx); err != nil {
		return err
	}

	logger.Info("pulse infrastructure ready", "schema", conn.Schema())

	// wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("pulse shutting down")
	return nil
}
