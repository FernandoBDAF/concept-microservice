package base

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	commonQueue "github.com/fernandobarroso/common/queue"
	"github.com/fernandobarroso/workers/common/processors"
	"github.com/prometheus/client_golang/prometheus"
)

// WorkerConfig holds configuration for any worker
type WorkerConfig struct {
	WorkerType    string
	QueueName     string
	ExchangeName  string
	RoutingKey    string
	PrefetchCount int
	HTTPPort      string
}

// BaseWorker provides common worker functionality
type BaseWorker struct {
	config    *WorkerConfig
	processor processors.MessageProcessor
	consumer  *commonQueue.Consumer
	server    *HTTPServer
	metrics   *WorkerMetrics
}

// WorkerMetrics provides common metrics for all workers
type WorkerMetrics struct {
	ConsumeLatency prometheus.Histogram
	ConsumeErrors  prometheus.Counter
	MessageAge     prometheus.Histogram
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(config *WorkerConfig, processor processors.MessageProcessor) (*BaseWorker, error) {
	// Initialize queue configuration
	queueConfig := commonQueue.NewConfig()
	queueConfig.Queue = config.QueueName
	queueConfig.Exchange = config.ExchangeName
	queueConfig.RoutingKey = config.RoutingKey
	queueConfig.PrefetchCount = config.PrefetchCount

	// Build RabbitMQ URL from environment
	rabbitUser := os.Getenv("RABBITMQ_USER")
	rabbitPassword := os.Getenv("RABBITMQ_PASSWORD")
	rabbitHost := os.Getenv("RABBITMQ_HOST")
	rabbitPort := os.Getenv("RABBITMQ_PORT")
	rabbitVhost := os.Getenv("RABBITMQ_VHOST")

	// Use defaults if not provided
	if rabbitUser == "" {
		rabbitUser = "guest"
	}
	if rabbitPassword == "" {
		rabbitPassword = "guest"
	}
	if rabbitHost == "" {
		rabbitHost = "localhost"
	}
	if rabbitPort == "" {
		rabbitPort = "5672"
	}
	if rabbitVhost == "" {
		rabbitVhost = "/"
	}

	queueConfig.URL = fmt.Sprintf("amqp://%s:%s@%s:%s/%s",
		rabbitUser, rabbitPassword, rabbitHost, rabbitPort, rabbitVhost)

	// Create consumer
	consumer, err := commonQueue.NewConsumer(queueConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create HTTP server for health checks
	server := NewHTTPServer(config.HTTPPort)

	// Initialize metrics
	metrics := &WorkerMetrics{
		ConsumeLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_consume_latency_seconds", config.WorkerType),
				Help: fmt.Sprintf("Time taken to consume messages for %s worker", config.WorkerType),
			},
		),
		ConsumeErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_consume_errors_total", config.WorkerType),
				Help: fmt.Sprintf("Total number of consume errors for %s worker", config.WorkerType),
			},
		),
		MessageAge: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_message_age_seconds", config.WorkerType),
				Help: fmt.Sprintf("Age of messages when consumed by %s worker", config.WorkerType),
			},
		),
	}

	// Register metrics
	prometheus.MustRegister(metrics.ConsumeLatency, metrics.ConsumeErrors, metrics.MessageAge)

	return &BaseWorker{
		config:    config,
		processor: processor,
		consumer:  consumer,
		server:    server,
		metrics:   metrics,
	}, nil
}

// Start starts the worker
func (w *BaseWorker) Start(ctx context.Context) error {
	log.Printf("Starting %s worker", w.config.WorkerType)

	// Start HTTP server
	go func() {
		w.server.SetReady(true)
		if err := w.server.Start(ctx); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Message handler with metrics
	handler := func(msg *commonQueue.Message) error {
		timer := prometheus.NewTimer(w.metrics.ConsumeLatency)
		defer timer.ObserveDuration()

		// Validate message first
		if err := w.processor.Validate(msg); err != nil {
			w.metrics.ConsumeErrors.Inc()
			return w.processor.HandleError(ctx, msg, err)
		}

		// Process message
		err := w.processor.Process(ctx, msg)
		if err != nil {
			w.metrics.ConsumeErrors.Inc()
			return w.processor.HandleError(ctx, msg, err)
		}

		return nil
	}

	// Start consumer
	return w.consumer.Start(ctx, handler)
}

// Shutdown gracefully shuts down the worker
func (w *BaseWorker) Shutdown(ctx context.Context) error {
	log.Printf("Shutting down %s worker", w.config.WorkerType)

	w.server.SetReady(false)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := w.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	// Close consumer
	return w.consumer.Close()
}

// Run runs the worker with signal handling
func (w *BaseWorker) Run() error {
	// Create context that listens for interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start worker
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Printf("Worker start error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the worker
	if err := w.Shutdown(shutdownCtx); err != nil {
		log.Printf("Worker shutdown error: %v", err)
		return err
	}

	log.Printf("%s worker shut down successfully", w.config.WorkerType)
	return nil
}
