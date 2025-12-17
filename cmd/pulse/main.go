package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/infrastructure/api"
	"github.com/joacominatel/pulse/internal/infrastructure/config"
	"github.com/joacominatel/pulse/internal/infrastructure/database"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
	"github.com/joacominatel/pulse/internal/infrastructure/postgres"
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

	// initialize repositories
	pool := conn.Pool()
	userRepo := postgres.NewUserRepository(pool)
	communityRepo := postgres.NewCommunityRepository(pool)
	eventRepo := postgres.NewActivityEventRepository(pool)

	// initialize use cases
	ingestEventUseCase := application.NewIngestEventUseCase(
		eventRepo,
		communityRepo,
		userRepo,
		logger,
	)

	calculateMomentumUseCase := application.NewCalculateMomentumUseCase(
		eventRepo,
		communityRepo,
		application.DefaultMomentumConfig(),
		logger,
	)

	// initialize http server
	serverConfig := api.DefaultServerConfig()
	if port := os.Getenv("PORT"); port != "" {
		serverConfig.Port = ":" + port
	}

	server := api.NewServer(serverConfig, logger)

	// register routes
	api.RegisterRoutes(server.Echo(), api.RouterConfig{
		IngestEventUseCase:       ingestEventUseCase,
		CalculateMomentumUseCase: calculateMomentumUseCase,
		Logger:                   logger,
	})

	// start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("http server error", "error", err.Error())
		}
	}()

	// wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("pulse shutting down")

	// graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), serverConfig.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err.Error())
		return err
	}

	logger.Info("pulse shutdown complete")
	return nil
}
