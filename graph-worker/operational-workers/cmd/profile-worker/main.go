package main

import (
	"log"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
	"github.com/fernandobarroso/microservices/operational-workers/internal/processors/profile"
)

func main() {
	log.Println("Starting Profile Worker...")

	// Declare args must stay byte-identical to the publisher's
	// (api-service/internal/domain/task/model.go DefaultRoutingMap["profile.task"]),
	// or RabbitMQ rejects the redeclare with PRECONDITION_FAILED.
	config := &base.WorkerConfig{
		WorkerType:    "profile",
		QueueName:     "profile-processing",
		ExchangeName:  "profile-tasks",
		RoutingKey:    "profile.task",
		PrefetchCount: 2,
		MessageTTL:    1 * time.Hour,
		DeadLetterTTL: 24 * time.Hour,
		MaxRetries:    3,
		HTTPPort:      utils.GetEnvOrDefault("HEALTH_PORT", "8080"),
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
