package base

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/idempotency"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/processors"
	commonQueue "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/tracing"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
	"github.com/prometheus/client_golang/prometheus"
)

// WorkerConfig holds configuration for any worker. Exchange/Queue/RoutingKey
// and the TTL/DLQ fields must match graph-worker/shared/contracts and the
// api-service publisher's declared topology exactly — see cmd/*/main.go for
// per-worker values and any documented deviations.
type WorkerConfig struct {
	WorkerType    string
	QueueName     string
	ExchangeName  string
	RoutingKey    string
	PrefetchCount int
	MessageTTL    time.Duration
	DeadLetterTTL time.Duration
	MaxRetries    int
	HTTPPort      string
}

// BaseWorker provides common worker functionality
type BaseWorker struct {
	config    *WorkerConfig
	processor processors.MessageProcessor
	consumer  *commonQueue.Consumer
	server    *HTTPServer
	metrics   *WorkerMetrics
	guard     idempotency.Guard
}

// WorkerMetrics provides common metrics for all workers
type WorkerMetrics struct {
	ConsumeLatency    prometheus.Histogram
	ConsumeErrors     prometheus.Counter
	MessageAge        prometheus.Histogram
	Duplicates        prometheus.Counter
	IdempotencyErrors prometheus.Counter
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(config *WorkerConfig, processor processors.MessageProcessor) (*BaseWorker, error) {
	// Initialize queue configuration
	queueConfig := commonQueue.NewConfig()
	queueConfig.WorkerType = config.WorkerType
	queueConfig.Queue = config.QueueName
	queueConfig.Exchange = config.ExchangeName
	queueConfig.RoutingKey = config.RoutingKey
	queueConfig.PrefetchCount = config.PrefetchCount
	queueConfig.MessageTTL = config.MessageTTL
	queueConfig.DeadLetterTTL = config.DeadLetterTTL
	queueConfig.MaxRetries = config.MaxRetries
	queueConfig.URL = resolveRabbitURL()

	// Create consumer (connects lazily in Start, with reconnect-on-failure).
	consumer, err := commonQueue.NewConsumer(queueConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create HTTP server for health checks
	server := NewHTTPServer(config.HTTPPort)

	// Initialize metrics. Registration is idempotent per WorkerType (see
	// utils.RegisterOrExisting) so constructing a worker more than once in
	// the same process doesn't panic on duplicate collector registration.
	metrics := &WorkerMetrics{
		ConsumeLatency: utils.RegisterOrExisting(prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_consume_latency_seconds", config.WorkerType),
				Help: fmt.Sprintf("Time taken to consume messages for %s worker", config.WorkerType),
			},
		)),
		ConsumeErrors: utils.RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_consume_errors_total", config.WorkerType),
				Help: fmt.Sprintf("Total number of consume errors for %s worker", config.WorkerType),
			},
		)),
		MessageAge: utils.RegisterOrExisting(prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_message_age_seconds", config.WorkerType),
				Help: fmt.Sprintf("Age of messages when consumed by %s worker", config.WorkerType),
			},
		)),
		Duplicates: utils.RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_duplicates_total", config.WorkerType),
				Help: fmt.Sprintf("Total duplicate deliveries deduped by the %s worker idempotency guard", config.WorkerType),
			},
		)),
		IdempotencyErrors: utils.RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_idempotency_errors_total", config.WorkerType),
				Help: fmt.Sprintf("Total idempotency guard errors (fail-open) for the %s worker", config.WorkerType),
			},
		)),
	}

	// Idempotency guard (ADR-008.2): shared Redis SETNX in production; an
	// in-memory guard (single-replica dev only) when REDIS_ADDR is unset.
	guard := resolveGuard(config.WorkerType)

	return &BaseWorker{
		config:    config,
		processor: processor,
		consumer:  consumer,
		server:    server,
		metrics:   metrics,
		guard:     guard,
	}, nil
}

// resolveGuard builds the idempotency guard from REDIS_ADDR (host:port, e.g.
// redis:6379). An empty value falls back to an in-memory guard with a loud
// warning — that only dedupes within a single process, so it is dev-only.
func resolveGuard(workerType string) idempotency.Guard {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		log.Printf("%s worker: idempotency guard using Redis at %s", workerType, addr)
		return idempotency.NewRedisGuardFromAddr(addr)
	}
	log.Printf("%s worker: REDIS_ADDR unset — using in-memory idempotency guard "+
		"(single-replica dev only; duplicates across replicas/restarts are NOT deduped)", workerType)
	return idempotency.NewInMemoryGuard()
}

// resolveRabbitURL implements the CONTRACTS.md env var contract: RABBITMQ_URL
// is authoritative (default amqp://guest:guest@rabbitmq:5672/); the
// individual RABBITMQ_HOST/PORT/USER/PASSWORD/VHOST vars remain supported
// for finer-grained overrides when RABBITMQ_URL is not set.
func resolveRabbitURL() string {
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		return url
	}

	user := utils.GetEnvOrDefault("RABBITMQ_USER", "guest")
	password := utils.GetEnvOrDefault("RABBITMQ_PASSWORD", "guest")
	host := utils.GetEnvOrDefault("RABBITMQ_HOST", "rabbitmq")
	port := utils.GetEnvOrDefault("RABBITMQ_PORT", "5672")
	// Default vhost is the empty string so the URL ends in a single "/",
	// matching the contract default exactly (amqp://guest:guest@rabbitmq:5672/).
	vhost := utils.GetEnvOrDefault("RABBITMQ_VHOST", "")

	return fmt.Sprintf("amqp://%s:%s@%s:%s/%s", user, password, host, port, vhost)
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

	// Message handler with metrics. processCtx comes from the consumer: it
	// carries the consumer span (extracted from the delivery's trace
	// headers) but is detached from the shutdown-signal ctx, so an
	// in-flight message finishes instead of being aborted mid-way when
	// SIGTERM/SIGINT arrives; the consumer stops pulling *new* deliveries
	// as soon as ctx is done.
	handler := func(processCtx context.Context, msg *commonQueue.Message) error {
		timer := prometheus.NewTimer(w.metrics.ConsumeLatency)
		defer timer.ObserveDuration()

		// Validate message first. A validation failure is unretryable (no
		// number of retries fixes a malformed message), so wrap it with
		// ErrUnretryable — the consumer then routes it straight to the DLQ
		// instead of cycling it through the retry tiers (ADR-008.1).
		if err := w.processor.Validate(msg); err != nil {
			w.metrics.ConsumeErrors.Inc()
			return w.processor.HandleError(processCtx, msg, errors.Join(commonQueue.ErrUnretryable, err))
		}

		// Idempotency guard (ADR-008.2). First delivery of an
		// (envelope, attempt) wins; a genuine duplicate (same attempt
		// redelivered) is acked without reprocessing. The key is scoped by
		// attempt so an intentional retry (which increments the attempt) is
		// still reprocessed. Guard errors FAIL OPEN — process anyway — because
		// dedupe is a safety net, not a lock.
		if msg.ID != "" {
			key := idempotency.KeyForAttempt(w.config.QueueName, msg.ID, msg.Attempt)
			switch ok, gerr := w.guard.Begin(processCtx, key, idempotency.DefaultTTL); {
			case gerr != nil:
				w.metrics.IdempotencyErrors.Inc()
				log.Printf("%s worker: idempotency guard error for %s (fail-open, processing anyway): %v",
					w.config.WorkerType, key, gerr)
			case !ok:
				w.metrics.Duplicates.Inc()
				log.Printf("%s worker: duplicate delivery id=%s attempt=%d — acking without reprocessing",
					w.config.WorkerType, msg.ID, msg.Attempt)
				return nil // ack, skip processing
			}
		}

		// Process message
		if err := w.processor.Process(processCtx, msg); err != nil {
			w.metrics.ConsumeErrors.Inc()
			return w.processor.HandleError(processCtx, msg, err)
		}

		return nil
	}

	// Start consumer (reconnects internally on connection/channel loss)
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

	// Close consumer: waits for the in-flight delivery to finish before
	// closing the channel/connection (see Consumer.Close).
	return w.consumer.Close()
}

// Run runs the worker with signal handling
func (w *BaseWorker) Run() error {
	// OpenTelemetry tracing (ADR-003.2): no-op unless
	// OTEL_EXPORTER_OTLP_ENDPOINT is set. Service name is
	// "<worker-type>-worker" (e.g. email-worker), overridable via
	// OTEL_SERVICE_NAME.
	tracingShutdown, err := tracing.Init(context.Background(), w.config.WorkerType+"-worker")
	if err != nil {
		return fmt.Errorf("failed to init tracing: %w", err)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracingShutdown(flushCtx); err != nil {
			log.Printf("Tracing shutdown error: %v", err)
		}
	}()

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
