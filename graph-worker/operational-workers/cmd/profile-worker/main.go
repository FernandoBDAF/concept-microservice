package main

import (
	"log"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
	"github.com/fernandobarroso/microservices/operational-workers/internal/processors/profile"
)

func main() {
	log.Println("Starting Profile Worker...")

	config := &base.WorkerConfig{
		WorkerType:    "profile",
		QueueName:     "profile-processing",
		ExchangeName:  "profile-tasks",
		RoutingKey:    "profile.task",
		PrefetchCount: 2,
		HTTPPort:      "8080",
	}

	processor := profile.NewProcessor()

	worker, err := base.NewBaseWorker(config, processor)
	if err != nil {
		log.Fatalf("Failed to create profile worker: %v", err)
	}

	if err := worker.Run(); err != nil {
		log.Fatalf("Profile worker failed: %v", err)
	}
}
