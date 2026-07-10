package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"microservices/services/profile-storage/internal/api/rest"
	"microservices/services/profile-storage/internal/config"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/database"
	"microservices/services/profile-storage/internal/infrastructure/repository"
	"microservices/services/profile-storage/internal/messaging"
	"microservices/services/profile-storage/internal/pkg/logger"
)

func main() {
	// Load configuration
	cfg := config.New()

	// Initialize logger
	if err := logger.Init(logger.Config{
		Environment: cfg.LogEnvironment,
		Level:       cfg.LogLevel,
		ServiceName: cfg.ServiceName,
	}); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting storage service",
		logger.String("service", cfg.ServiceName),
		logger.String("version", "1.0.0"),
		logger.Bool("queue_enabled", cfg.QueueEnabled))

	// Create connection manager
	connManager := database.NewConnectionManager(cfg)

	// Connect to database with retry logic
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := connManager.Connect(ctx); err != nil {
		logger.Fatal("Failed to connect to database", logger.ErrorField(err))
	}
	defer connManager.Close()

	// Create repositories
	profileRepo := repository.NewProfileRepository(connManager.GetDB())

	// Create services
	profileService := service.NewProfileService(profileRepo)
	batchService := service.NewAdvancedBatchOperationsService(profileService, connManager.GetDB())

	// Initialize messaging components if queue is enabled
	// REVIEW: should we really connect to queue directly?
	var consumer *messaging.Consumer
	var messageProcessor *messaging.MessageProcessor
	if cfg.QueueEnabled {
		logger.Info("Queue processing enabled - initializing consumer")

		// Create message processor
		messageProcessor = messaging.NewMessageProcessor()

		// Create and register message handlers
		batchHandler := messaging.NewBatchMessageHandler(batchService)
		storageHandler := messaging.NewStorageHandler(profileService, batchService)

		// Register handlers with the processor
		if err := messageProcessor.RegisterHandler(batchHandler); err != nil {
			logger.Fatal("Failed to register batch handler", logger.ErrorField(err))
		}
		if err := messageProcessor.RegisterHandler(storageHandler); err != nil {
			logger.Fatal("Failed to register storage handler", logger.ErrorField(err))
		}

		// Create consumer configuration
		consumerConfig := &messaging.ConsumerConfig{
			ConnectionURL:   cfg.RabbitMQURL,
			QueueName:       cfg.RabbitMQQueue,
			ExchangeName:    cfg.RabbitMQExchange,
			RoutingKey:      cfg.RabbitMQRoutingKey,
			ConsumerTag:     cfg.RabbitMQConsumerTag,
			PrefetchCount:   cfg.RabbitMQPrefetch,
			ProcessTimeout:  cfg.RabbitMQProcessTimeout,
			ReconnectDelay:  cfg.RabbitMQReconnectDelay,
			DLQEnabled:      cfg.DLQEnabled,
			DLQExchangeName: cfg.DLQExchangeName,
			DLQQueueName:    cfg.DLQQueueName,
			MaxRetries:      cfg.DLQMaxRetries,
		}

		// Create consumer
		consumer = messaging.NewConsumer(consumerConfig, messageProcessor)

		// Start consumer in goroutine
		go func() {
			logger.Info("Starting queue consumer",
				logger.String("queue", cfg.RabbitMQQueue),
				logger.String("exchange", cfg.RabbitMQExchange),
				logger.String("routing_key", cfg.RabbitMQRoutingKey))

			if err := consumer.Start(context.Background()); err != nil {
				logger.Error("Failed to start queue consumer", logger.ErrorField(err))
				// Don't exit - HTTP server should continue running
			}
		}()

		logger.Info("Queue processing enabled and active")
	} else {
		logger.Info("Queue processing disabled - running in HTTP-only mode")
	}

	// Create REST handlers
	profileHandler := rest.NewProfileHandler(profileService)
	batchHandler := rest.NewBatchHandler(batchService)
	healthHandler := rest.NewHealthHandler(connManager.GetDB())

	// Create and configure REST server
	restServer := rest.NewServer(cfg)

	// Register all handlers with the REST server
	restServer.RegisterRoutes(profileHandler, batchHandler, healthHandler)

	logger.Info("Service initialization complete!",
		logger.String("batch_service", "ACTIVE"),
		logger.Bool("profile_service", profileService != nil),
		logger.Bool("batch_service_ready", batchService != nil),
		logger.Bool("rest_server_ready", restServer != nil),
		logger.String("batch_endpoints", "REGISTERED"))

	// Start REST server
	go func() {
		logger.Info("Starting REST server", logger.String("addr", fmt.Sprintf(":%s", cfg.ServerPort)))
		if err := restServer.Start(); err != nil {
			logger.Fatal("Failed to start REST server", logger.ErrorField(err))
		}
	}()

	// Create and start gRPC server
	// REVIEW: implement gRPC server
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
		if err != nil {
			logger.Fatal("Failed to listen on gRPC port", logger.ErrorField(err))
		}

		grpcServer := grpc.NewServer()

		// Register gRPC services
		// Note: Commented out until proto definitions are available
		// grpcProfileHandler := grpc_handler.NewServer(profileService)
		// pb.RegisterProfileServiceServer(grpcServer, grpcProfileHandler)

		// Register health service
		// grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())

		// Register reflection service (for development)
		reflection.Register(grpcServer)

		logger.Info("Starting gRPC server", logger.String("addr", fmt.Sprintf(":%s", cfg.GRPCPort)))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC", logger.ErrorField(err))
		}
	}()

	logger.Info("Storage service started successfully!")

	// Wait for interrupt signal to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down storage service...")

	// Graceful shutdown
	// Shutdown queue consumer first
	if consumer != nil {
		logger.Info("Shutting down queue consumer...")
		if err := consumer.Stop(); err != nil {
			logger.Error("Queue consumer shutdown error", logger.ErrorField(err))
		} else {
			logger.Info("Queue consumer stopped successfully")
		}
	}

	// TODO: Add proper REST server shutdown method
	logger.Info("REST server shutdown - manual implementation needed")

	logger.Info("Storage service shutdown complete")
}
