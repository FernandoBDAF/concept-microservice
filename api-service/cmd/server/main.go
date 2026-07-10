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
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/postgres"
	redisInfra "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/redis"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/rabbitmq"
	"github.com/fernandobarroso/microservices/api-service/internal/pkg/logger"
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

	authClient := auth.NewClient(cfg.Auth, cfg.CircuitBreaker)
	profileRepo := postgres.NewProfileRepository(db, log)

	var cache profile.Cache
	if redisClient != nil {
		cache = redisInfra.NewCache(redisClient, cfg.Cache)
	}
	profileService := profile.NewService(profileRepo, cache, redisInfra.ErrCacheMiss)

	publisher := rabbitmq.NewPublisher(rmqClient)
	taskService := task.NewService(publisher)

	var documentService *document.Service
	if minioClient != nil {
		documentRepo := postgres.NewDocumentRepository(db, log)
		documentPublisher := task.NewDocumentTaskPublisher(taskService)
		documentService = document.NewService(documentRepo, minioClient, documentPublisher, log)
		log.Info("Document service initialized")
	}

	healthHandler := handlers.NewHealthHandler(db, redisClient, rmqClient)
	router := api.NewRouter(cfg, authClient, profileService, taskService, documentService, healthHandler, log)

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

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server", zap.Error(err))
	}

	log.Info("server shutdown complete")
	time.Sleep(100 * time.Millisecond)
}

