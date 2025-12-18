package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joacominatel/pulse/internal/application"
	"github.com/joacominatel/pulse/internal/domain"
	"github.com/joacominatel/pulse/internal/infrastructure/api"
	"github.com/joacominatel/pulse/internal/infrastructure/auth"
	"github.com/joacominatel/pulse/internal/infrastructure/cache"
	"github.com/joacominatel/pulse/internal/infrastructure/config"
	"github.com/joacominatel/pulse/internal/infrastructure/database"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
	"github.com/joacominatel/pulse/internal/infrastructure/metrics"
	"github.com/joacominatel/pulse/internal/infrastructure/postgres"
	"github.com/joacominatel/pulse/internal/infrastructure/worker"
)

const (
	// momentumCalculationInterval is how often momentum is recalculated
	momentumCalculationInterval = 5 * time.Minute
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
	conn, err := database.New(&cfg.Database, logger)
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

	// initialize prometheus metrics
	appMetrics := metrics.New()
	logger.Info("prometheus metrics initialized")

	// initialize jwt validator
	jwtValidator := auth.NewJWTValidator(cfg.Auth.JWTSecret)

	// initialize repositories
	pool := conn.Pool()
	userRepo := postgres.NewUserRepository(pool)
	postgresCommunityRepo := postgres.NewCommunityRepository(pool)
	eventRepo := postgres.NewActivityEventRepository(pool)

	// initialize redis (optional - disabled if REDIS_URL is empty)
	var redisClient *cache.RedisClient
	var communityRepo domain.CommunityRepository = postgresCommunityRepo

	if cfg.Redis.URL != "" {
		redisClient, err = cache.NewRedisClient(cache.RedisConfig{URL: cfg.Redis.URL}, logger)
		if err != nil {
			logger.Error("failed to create redis client", "error", err.Error())
			return err
		}

		if err := redisClient.Connect(ctx); err != nil {
			logger.Warn("redis connection failed, continuing without cache", "error", err.Error())
			redisClient = nil
		} else {
			defer redisClient.Close()
			// wrap community repo with redis cache for reads
			communityRepo = cache.NewCommunityRepositoryWithCache(postgresCommunityRepo, redisClient, logger)
			logger.Info("redis leaderboard cache enabled")
		}
	}

	// initialize event ingestion worker (async buffer pattern)
	ingestionWorkerConfig := worker.DefaultEventIngestionConfig()
	ingestionWorker := worker.NewEventIngestionWorker(eventRepo, ingestionWorkerConfig, logger).
		WithMetrics(appMetrics)

	// start the ingestion worker before accepting requests
	workerCtx, workerCancel := context.WithCancel(context.Background())
	ingestionWorker.Start(workerCtx)

	// initialize webhook subscription repository
	webhookSubRepo := postgres.NewWebhookSubscriptionRepository(pool)

	// initialize webhook worker for momentum spike notifications
	webhookWorkerConfig := worker.DefaultWebhookWorkerConfig()
	webhookWorker := worker.NewWebhookWorker(webhookSubRepo, webhookWorkerConfig, logger)
	webhookWorker.Start(workerCtx)

	// initialize community existence cache for high-throughput ingestion
	// caches community exists/active checks to avoid DB hits on every event
	communityExistsCache := cache.NewCommunityExistsCache(postgresCommunityRepo, 1*time.Minute)

	// initialize use cases
	ingestEventUseCase := application.NewIngestEventUseCase(
		eventRepo,
		communityRepo,
		userRepo,
		logger,
	).WithEventChannel(ingestionWorker.EventChannel()). // enable async mode
								WithCommunityChecker(communityExistsCache) // use cache for existence checks

	calculateMomentumUseCase := application.NewCalculateMomentumUseCase(
		eventRepo,
		communityRepo,
		application.DefaultMomentumConfig(),
		logger,
	).WithNotifier(webhookWorker) // wire spike notifications

	// wire redis leaderboard to momentum use case if available
	if redisClient != nil {
		calculateMomentumUseCase = calculateMomentumUseCase.WithLeaderboard(redisClient)
	}

	createCommunityUseCase := application.NewCreateCommunityUseCase(
		communityRepo,
		userRepo,
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
		CreateCommunityUseCase:   createCommunityUseCase,
		CommunityRepo:            communityRepo,
		JWTValidator:             jwtValidator,
		Logger:                   logger,
		Metrics:                  appMetrics,
	})

	// start background momentum worker
	go runMomentumWorker(workerCtx, calculateMomentumUseCase, appMetrics, logger)

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

	// stop background workers
	workerCancel()

	// stop ingestion worker and drain buffer
	ingestionWorker.Stop()

	// stop webhook worker and drain buffer
	webhookWorker.Stop()

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

// runMomentumWorker runs the momentum calculation in the background
// every momentumCalculationInterval until context is cancelled
func runMomentumWorker(ctx context.Context, useCase *application.CalculateMomentumUseCase, appMetrics *metrics.Metrics, logger *logging.Logger) {
	logger.Info("momentum worker started", "interval", momentumCalculationInterval.String())

	ticker := time.NewTicker(momentumCalculationInterval)
	defer ticker.Stop()

	// run immediately on startup
	runMomentumCalculation(ctx, useCase, appMetrics, logger)

	for {
		select {
		case <-ctx.Done():
			logger.Info("momentum worker stopping")
			return
		case <-ticker.C:
			runMomentumCalculation(ctx, useCase, appMetrics, logger)
		}
	}
}

// runMomentumCalculation executes a single momentum calculation cycle
func runMomentumCalculation(ctx context.Context, useCase *application.CalculateMomentumUseCase, appMetrics *metrics.Metrics, logger *logging.Logger) {
	start := time.Now()
	result, err := useCase.ExecuteAll(ctx, application.CalculateAllInput{
		Limit: 0, // process all communities
	})
	duration := time.Since(start)

	// record metric regardless of success/failure
	if appMetrics != nil {
		appMetrics.RecordMomentumCalculation(duration.Seconds())
	}

	if err != nil {
		logger.Error("momentum calculation failed",
			"error", err.Error(),
			"duration_ms", duration.Milliseconds(),
		)
		return
	}

	logger.Info("momentum calculation completed",
		"processed", result.Processed,
		"succeeded", result.Succeeded,
		"failed", result.Failed,
		"duration_ms", duration.Milliseconds(),
	)
}
