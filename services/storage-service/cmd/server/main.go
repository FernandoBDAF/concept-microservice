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
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	pb "microservices/services/storage-service/api/proto/profile"
	grpcapi "microservices/services/storage-service/internal/api/grpc"
	"microservices/services/storage-service/internal/api/rest"
	"microservices/services/storage-service/internal/config"
	"microservices/services/storage-service/internal/domain/service"
	"microservices/services/storage-service/internal/infrastructure/database"
	"microservices/services/storage-service/internal/infrastructure/repository"
	"microservices/services/storage-service/internal/pkg/logger"
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

	// Create connection manager
	connManager := database.NewConnectionManager(cfg)

	// Connect to database with retry logic
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := connManager.Connect(ctx); err != nil {
		logger.Fatal("Failed to connect to database", logger.ErrorField(err))
	}
	defer connManager.Close()

	// Create repository and service
	profileRepo := repository.NewProfileRepository(connManager.GetDB())
	profileService := service.NewProfileService(profileRepo)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register services
	pb.RegisterProfileServiceServer(grpcServer, grpcapi.NewServer(profileService))
	grpc_health_v1.RegisterHealthServer(grpcServer, grpcapi.NewHealthServer(connManager.GetDB().DB))

	// Register reflection service
	reflection.Register(grpcServer)

	// Start server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Fatal("Failed to listen", logger.ErrorField(err))
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting gRPC server", logger.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve", logger.ErrorField(err))
		}
	}()

	// Create REST server
	restServer := rest.NewServer(cfg)

	// Initialize handlers
	profileHandler := rest.NewProfileHandler(profileService)
	healthHandler := rest.NewHealthHandler(connManager.GetDB())
	metricsHandler := rest.NewMetricsHandler(connManager)
	handler := rest.NewHandler(profileService)

	// Register routes
	restServer.RegisterRoutes(profileHandler, healthHandler, metricsHandler, handler)

	// Start REST server in a goroutine
	go func() {
		logger.Info("Starting REST server", logger.String("port", cfg.ServerPort))
		if err := restServer.Start(); err != nil {
			logger.Fatal("Failed to start REST server", logger.ErrorField(err))
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	logger.Info("Shutting down server...")
	grpcServer.GracefulStop()
}
