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

	httpadapter "github.com/FBDAF/microservices/services/queue-service/internal/adapters/http"
	"github.com/FBDAF/microservices/services/queue-service/internal/adapters/rabbitmq"
	"github.com/FBDAF/microservices/services/queue-service/internal/config"
	"github.com/FBDAF/microservices/services/queue-service/internal/domain/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize RabbitMQ
	rmqConfig := &rabbitmq.Config{
		Hosts:            make([]string, len(cfg.RabbitMQ.Cluster.Nodes)),
		Username:         os.Getenv("RABBITMQ_USERNAME"),
		Password:         os.Getenv("RABBITMQ_PASSWORD"),
		VHost:            os.Getenv("RABBITMQ_VHOST"),
		PrefetchCount:    cfg.RabbitMQ.Options.PrefetchCount,
		ReconnectTimeout: cfg.RabbitMQ.Options.ReconnectInterval,
		MaxRetries:       cfg.RabbitMQ.Options.MaxRetries,
		MessageTTL:       cfg.RabbitMQ.Options.MessageTTL,
	}

	// Convert node configurations to host strings
	for i, node := range cfg.RabbitMQ.Cluster.Nodes {
		rmqConfig.Hosts[i] = fmt.Sprintf("%s:%d", node.Host, node.Port)
	}

	rmq, err := rabbitmq.New(rmqConfig)
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ: %v", err)
	}
	defer rmq.Close()

	// Initialize queue service
	queueService := service.NewQueueService(rmq, "default")

	// Start consuming messages
	go func() {
		if err := queueService.StartConsuming(); err != nil {
			log.Printf("Failed to start consuming messages: %v", err)
		}
	}()

	// Initialize HTTP handler
	handler := httpadapter.NewHandler(queueService)

	// Create Gin router
	router := gin.Default()

	// Register routes
	handler.RegisterRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Service.Port),
		Handler: router,
	}

	// Start HTTP server
	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.Service.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
