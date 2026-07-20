package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/api"
	"github.com/fernandobarroso/microservices/api-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/api-service/internal/config"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/document"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/auth"
	minioInfra "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/minio"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/outbox"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/postgres"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/rabbitmq"
	redisInfra "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/redis"
	"github.com/fernandobarroso/microservices/api-service/internal/pkg/logger"
	"github.com/fernandobarroso/microservices/api-service/internal/pkg/tracing"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Logging)
	if err != nil {
		fmt.Printf("failed to init logger: %v\n", err)
		os.Exit(1)
	}
	zap.ReplaceGlobals(log)

	// OpenTelemetry tracing (ADR-003.2): no-op unless
	// OTEL_EXPORTER_OTLP_ENDPOINT is set. Must run before anything that
	// creates tracers (router, postgres client, publisher).
	tracingShutdown, err := tracing.Init(context.Background(), "api-service")
	if err != nil {
		log.Fatal("failed to init tracing", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracingShutdown(ctx); err != nil {
			log.Error("failed to shutdown tracing", zap.Error(err))
		}
	}()

	db, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		log.Fatal("failed to init postgres", zap.Error(err))
	}
	defer db.Close()

	var redisClient *redisInfra.Client
	if cfg.Redis.Enabled {
		redisClient, err = redisInfra.NewClient(cfg.Redis)
		if err != nil {
			log.Fatal("failed to init redis", zap.Error(err))
		}
		defer redisClient.Close()
	}

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
	if err != nil {
		log.Fatal("failed to init rabbitmq", zap.Error(err))
	}
	defer rmqClient.Close()

	var minioClient *minioInfra.Client
	if cfg.MinIO.Endpoint != "" {
		minioCfg := minioInfra.Config{
			Endpoint:        cfg.MinIO.Endpoint,
			AccessKeyID:     cfg.MinIO.AccessKeyID,
			SecretAccessKey: cfg.MinIO.SecretAccessKey,
			UseSSL:          cfg.MinIO.UseSSL,
			BucketName:      cfg.MinIO.BucketName,
			MaxUploadSize:   cfg.MinIO.MaxUploadSize,
		}
		minioClient, err = minioInfra.NewClient(minioCfg, log)
		if err != nil {
			log.Fatal("failed to init minio", zap.Error(err))
		}
		log.Info("MinIO client initialized", zap.String("bucket", cfg.MinIO.BucketName))
	}

	// Background workers (outbox relay, results consumer, JWKS refresher)
	// share one lifecycle context, canceled during graceful shutdown.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	authClient := auth.NewClient(cfg.Auth, cfg.CircuitBreaker)
	// ADR-009.1: local JWKS verification is the default auth path; the
	// introspection client above stays as the strict-mode escape hatch.
	jwksVerifier := auth.NewJWKSVerifier(cfg.Auth, log)
	go jwksVerifier.Start(workerCtx)

	profileRepo := postgres.NewProfileRepository(db, log)

	var cache profile.Cache
	if redisClient != nil {
		cache = redisInfra.NewCache(redisClient, cfg.Cache)
	}
	profileService := profile.NewService(profileRepo, cache, redisInfra.ErrCacheMiss)

	// ADR-008.3: every task publish goes through the transactional outbox;
	// the relay is the only writer to the broker (confirm-mode).
	outboxStore := outbox.NewStore(db)
	taskService := task.NewService(outboxStore)

	publisher := rabbitmq.NewPublisher(rmqClient)
	go func() {
		if err := outbox.Relay(workerCtx, outboxStore, publisher, outbox.DefaultInterval); err != nil {
			log.Error("outbox relay exited", zap.Error(err))
		}
	}()

	var documentService *document.Service
	if minioClient != nil {
		documentRepo := postgres.NewDocumentRepository(db, outboxStore, log)
		documentService = document.NewService(documentRepo, minioClient, task.NewDocumentTaskBuilder(), log)
		log.Info("Document service initialized")
	}

	// task-results consumer (ADR-008.3): workers/graphrag publish
	// completion/failure; document status advances processing→completed/failed.
	var docUpdater task.DocumentStatusUpdater
	if documentService != nil {
		docUpdater = documentService
	}
	resultHandler := task.NewResultHandler(docUpdater, log)
	resultsConsumer := rabbitmq.NewResultsConsumer(rmqClient, resultHandler.Handle, log)
	go resultsConsumer.Start(workerCtx)

	healthHandler := handlers.NewHealthHandler(db, redisClient, rmqClient)
	router := api.NewRouter(cfg, authClient, jwksVerifier, profileService, taskService, documentService, healthHandler, log)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info("HTTP server started", zap.Int("port", cfg.Server.HTTPPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Prometheus metrics are served on their own port per the platform
	// contract, independent from the public API port above.
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsServer = api.NewMetricsServer(cfg)
		go func() {
			log.Info("Metrics server started", zap.Int("port", cfg.Metrics.Port))
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("metrics server error", zap.Error(err))
			}
		}()
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	log.Info("shutdown signal received, draining")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server", zap.Error(err))
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Error("failed to shutdown metrics server", zap.Error(err))
		}
	}

	// Stop background workers (outbox relay, results consumer, JWKS
	// refresher) after the HTTP surface is drained; unsent outbox rows are
	// simply picked up on next start — that is the whole point of the design.
	workerCancel()

	log.Info("server shutdown complete")
	time.Sleep(100 * time.Millisecond)
}
