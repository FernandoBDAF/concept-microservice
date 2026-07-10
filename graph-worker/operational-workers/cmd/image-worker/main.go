package main

import (
	"log"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
	"github.com/fernandobarroso/microservices/operational-workers/internal/processors/image"
)

func main() {
	log.Println("Starting Image Worker...")

	// Image worker configuration as specified in tracker
	config := &base.WorkerConfig{
		WorkerType:    "image",
		QueueName:     "image-processing",
		ExchangeName:  "image-tasks",
		RoutingKey:    "image.process",
		PrefetchCount: 1, // Resource intensive - process one at a time
		HTTPPort:      "8080",
	}

	// Create the image processor
	processor := image.NewImageProcessor()

	// Create the base worker with image processor
	worker, err := base.NewBaseWorker(config, processor)
	if err != nil {
		log.Fatalf("Failed to create image worker: %v", err)
	}

	// Run the worker (includes signal handling)
	if err := worker.Run(); err != nil {
		log.Fatalf("Image worker failed: %v", err)
	}
}
