package main

import (
	"log"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
	"github.com/fernandobarroso/microservices/operational-workers/internal/processors/email"
)

func main() {
	log.Println("Starting Email Worker...")

	// Email worker configuration as specified in tracker
	config := &base.WorkerConfig{
		WorkerType:    "email",
		QueueName:     "email-processing",
		ExchangeName:  "email-tasks",
		RoutingKey:    "email.send",
		PrefetchCount: 5, // Higher throughput for burst processing
		HTTPPort:      "8080",
	}

	// Create the email processor
	processor := email.NewEmailProcessor()

	// Create the base worker with email processor
	worker, err := base.NewBaseWorker(config, processor)
	if err != nil {
		log.Fatalf("Failed to create email worker: %v", err)
	}

	// Run the worker (includes signal handling)
	if err := worker.Run(); err != nil {
		log.Fatalf("Email worker failed: %v", err)
	}
}
