package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/fernandobarroso/microservices/services/profile-service/internal/api/routes"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/cache"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/session"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// Create background context
	ctx := context.Background()

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize logger
	loggerCfg := &logger.Config{
		Level:       cfg.Logging.Level,
		Environment: cfg.Environment,
		ServiceName: "profile-api",
		Format:      cfg.Logging.Format,
		LogFile:     cfg.Logging.LogFile,
		Shipping: &logger.ShippingConfig{
			Enabled:    cfg.Logging.Shipping.Enabled,
			Endpoint:   cfg.Logging.Shipping.Endpoint,
			BufferSize: cfg.Logging.Shipping.BufferSize,
			MaxRetries: cfg.Logging.Shipping.MaxRetries,
			RetryDelay: cfg.Logging.Shipping.RetryDelay,
		},
	}
	if err := logger.Initialize(loggerCfg); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Get logger instance
	zapLogger := logger.Logger

	// Initialize auth service client
	authClient := services.NewAuthServiceClient(cfg)

	// Initialize HTTP cache client for cache-service integration
	logger.LogInfo(ctx, "Initializing HTTP cache client for cache-service integration")
	cacheClient, err := cache.NewCacheClient(&cfg.Cache, zapLogger)
	if err != nil {
		logger.LogError(ctx, "Failed to initialize cache client", err)
		log.Fatalf("Failed to initialize cache client: %v", err)
	}
	defer cacheClient.Close()

	// Initialize session manager with HTTP cache client
	logger.LogInfo(ctx, "Initializing session manager with cache-service HTTP integration")
	sessionManager, err := session.NewSessionManager(authClient, cacheClient, zapLogger)
	if err != nil {
		logger.LogError(ctx, "Failed to initialize session manager", err)
		log.Fatalf("Failed to initialize session manager: %v", err)
	}
	defer sessionManager.Close()

	// Initialize storage client
	storageClient := services.NewStorageClient(cfg)

	// Initialize profile service with cache integration
	profileService := services.NewProfileService(
		cfg,
		storageClient,
		cacheClient,
		authClient,
		zapLogger,
	)

	// Initialize router
	router := api.NewRouter(cfg, authClient, sessionManager, profileService)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.LogInfo(ctx, "Starting server",
		zap.String("address", addr))

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-done
		logger.LogInfo(ctx, "Shutting down server")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.LogError(ctx, "Failed to shutdown server", err)
			log.Fatalf("Failed to shutdown server: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.LogError(ctx, "Failed to start server", err)
		log.Fatalf("Failed to start server: %v", err)
	}
}
