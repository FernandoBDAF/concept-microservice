// Command api is a minimal HTTP API that publishes work reliably: it accepts
// task submissions, writes them to a transactional outbox in Postgres, and a
// background relay publishes them to RabbitMQ with broker confirms. The
// crash window between "DB committed" and "message published" is closed by
// construction (see internal/outbox).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"example.com/api-publisher/internal/auth"
	"example.com/api-publisher/internal/config"
	"example.com/api-publisher/internal/httpapi"
	"example.com/api-publisher/internal/outbox"
	"example.com/api-publisher/internal/postgres"
	"example.com/api-publisher/internal/rabbitmq"
	"example.com/api-publisher/internal/task"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()
	zap.ReplaceGlobals(log)

	db, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		log.Fatal("failed to init postgres", zap.Error(err))
	}
	defer db.Close()

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
	if err != nil {
		log.Fatal("failed to init rabbitmq", zap.Error(err))
	}
	defer rmqClient.Close()

	// Background workers (outbox relay, JWKS refresher) share one lifecycle
	// context, canceled during graceful shutdown.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	// Local JWKS verification is the default (only) auth path — no per-request
	// hop to the auth service.
	jwksVerifier := auth.NewJWKSVerifier(cfg.Auth, log)
	go jwksVerifier.Start(workerCtx)

	// Every task publish goes through the transactional outbox; the relay is
	// the only writer to the broker (confirm-mode).
	outboxStore := outbox.NewStore(db)
	taskService := task.NewService(outboxStore)
	publisher := rabbitmq.NewPublisher(rmqClient)
	go func() {
		if err := outbox.Relay(workerCtx, outboxStore, publisher, outbox.DefaultInterval); err != nil {
			log.Error("outbox relay exited", zap.Error(err))
		}
	}()

	server := httpapi.NewServer(cfg.Server, cfg.Auth, jwksVerifier, taskService, db, log)
	go func() {
		log.Info("HTTP server started", zap.Int("port", cfg.Server.HTTPPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	log.Info("shutdown signal received, draining")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server", zap.Error(err))
	}

	// Stop background workers after the HTTP surface is drained. Unsent outbox
	// rows are simply picked up on the next start — that is the whole point.
	workerCancel()

	log.Info("server shutdown complete")
	time.Sleep(100 * time.Millisecond)
}
