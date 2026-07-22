package main

import (
	"log"

	"example.com/worker/internal/common/base"
	"example.com/worker/internal/common/utils"
	"example.com/worker/internal/processors/example"
)

func main() {
	log.Println("Starting example worker...")

	// Topology for the example.task pipeline. These names MUST match the
	// broker-owned definitions.json (ADR-008.4) — generate a matching fragment
	// with:
	//
	//   python3 scripts/generate-definitions.py --pipeline example:example.task
	//
	// which yields exchange "example-tasks", work queue "example-processing",
	// routing key "example.task", the 5s/30s/2m retry wait-queues, the DLX/DLQ,
	// and the shared task-results loop. TTL/MaxRetries are informational here
	// (the broker owns the real queue args) — see internal/common/queue/config.go.
	config := &base.WorkerConfig{
		WorkerType:    "example",
		QueueName:     "example-processing",
		ExchangeName:  "example-tasks",
		RoutingKey:    "example.task",
		PrefetchCount: 5,
		MaxRetries:    3, // == len(queue.RetryTiers)
		HTTPPort:      utils.GetEnvOrDefault("HEALTH_PORT", "8080"),
	}

	processor := example.NewProcessor()

	worker, err := base.NewBaseWorker(config, processor)
	if err != nil {
		log.Fatalf("Failed to create example worker: %v", err)
	}

	if err := worker.Run(); err != nil {
		log.Fatalf("example worker failed: %v", err)
	}
}
