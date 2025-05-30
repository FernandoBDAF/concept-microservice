package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	commonQueue "github.com/fernandobarroso/common/queue"
	serviceQueue "github.com/fernandobarroso/worker-service/internal/adapters/queue"
	"github.com/fernandobarroso/worker-service/internal/processors/profile"
	"github.com/fernandobarroso/worker-service/internal/server"
)

func main() {
	// Load configuration
	config := commonQueue.NewConfig()

	rabbitUser := os.Getenv("RABBITMQ_USER")
	rabbitPassword := os.Getenv("RABBITMQ_PASSWORD")
	rabbitHost := os.Getenv("RABBITMQ_HOST")
	rabbitPort := os.Getenv("RABBITMQ_PORT")

	config.URL = fmt.Sprintf("amqp://%s:%s@%s:%s/", rabbitUser, rabbitPassword, rabbitHost, rabbitPort)

	// Create processor
	processor := profile.NewProcessor()

	// Create consumer
	consumer, err := serviceQueue.NewConsumer(config, processor)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	// Create and start server
	srv := server.NewServer(consumer, "8080")

	// Create context that listens for the interrupt signal from the OS
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the server
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}
