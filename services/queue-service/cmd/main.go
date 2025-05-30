package main

import (
	"context"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Initialize logger
	logger := log.New(os.Stdout, "[QUEUE-SERVICE] ", log.LstdFlags)

	// Load configuration
	cfg := config.NewConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize RabbitMQ
	rmqConfig := &rabbitmq.Config{
		Hosts:            []string{"localhost:5672"}, // TODO: Get from config
		Username:         "guest",                    // TODO: Get from config
		Password:         "guest",                    // TODO: Get from config
		VHost:            "/",
		PrefetchCount:    cfg.RabbitMQ.Options.PrefetchCount,
		ReconnectTimeout: cfg.RabbitMQ.Options.ReconnectInterval,
		MaxRetries:       cfg.RabbitMQ.Options.MaxRetries,
	}

	rmq, err := rabbitmq.New(rmqConfig)
	if err != nil {
		logger.Fatalf("Failed to initialize RabbitMQ: %v", err)
	}
	defer rmq.Close()

	// Initialize queue service
	queueService := service.NewQueueService()

	// Initialize HTTP handler
	handler := httpadapter.NewHandler(queueService)

	// Create Gin router
	router := gin.Default()

	// Register routes
	handler.RegisterRoutes(router)

	// Add health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	// Add metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Printf("Starting server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server exiting")
}
