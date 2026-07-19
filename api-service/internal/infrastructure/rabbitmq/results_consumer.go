package rabbitmq

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

// Broker names for the results path — data lives in
// deploy/rabbitmq/definitions.json (exchange task-results → queue
// task-results via rk task.result); never invented here (ADR-008.4).
const (
	ResultsQueue      = "task-results"
	ResultsRoutingKey = "task.result"
)

// ResultHandlerFunc processes one decoded task.result envelope. nil → ack;
// error → nack+requeue (transient failure, e.g. DB down).
type ResultHandlerFunc func(ctx context.Context, msg *task.Message) error

// ResultsConsumer consumes the task-results queue (ADR-008.3): workers and
// graphrag publish completion/failure there; api-service advances document
// status from it. It runs its own channel on the shared connection,
// passively verifies the queue (never declares), and resubscribes after
// connection loss.
type ResultsConsumer struct {
	client  *Client
	handler ResultHandlerFunc
	log     *zap.Logger
}

func NewResultsConsumer(client *Client, handler ResultHandlerFunc, log *zap.Logger) *ResultsConsumer {
	return &ResultsConsumer{
		client:  client,
		handler: handler,
		log:     log.Named("results_consumer"),
	}
}

// Start blocks until ctx is done, consuming task.result messages and
// re-establishing the subscription after failures. Run it as a goroutine.
func (rc *ResultsConsumer) Start(ctx context.Context) {
	retry := rc.client.config.ReconnectTimeout
	if retry <= 0 {
		retry = 5 * time.Second
	}

	for {
		if err := rc.consumeOnce(ctx); err != nil {
			rc.log.Warn("task-results consumer stopped, retrying",
				zap.Error(err), zap.Duration("retry_in", retry))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}

// consumeOnce opens a channel, verifies the queue passively, and drains
// deliveries until the channel dies or ctx is canceled.
func (rc *ResultsConsumer) consumeOnce(ctx context.Context) error {
	ch, err := rc.client.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// Passive only (ADR-008.4). Missing queue → pointed crash-grade log; the
	// consumer keeps retrying rather than killing an otherwise-serving API.
	if _, err := ch.QueueDeclarePassive(ResultsQueue, true, false, false, false, nil); err != nil {
		rc.log.Error("broker topology missing — is definitions.json loaded?",
			zap.String("queue", ResultsQueue), zap.Error(err))
		return err
	}

	if err := ch.Qos(rc.client.PrefetchCount(), 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(ResultsQueue, "api-service-results", false, false, false, false, nil)
	if err != nil {
		return err
	}

	rc.log.Info("consuming task results", zap.String("queue", ResultsQueue))

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return nil // channel closed; outer loop resubscribes
			}
			rc.handleDelivery(ctx, d)
		}
	}
}

func (rc *ResultsConsumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var msg task.Message
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		// task-results has no DLQ: poison must be logged and acked, never
		// requeued into a loop.
		rc.log.Warn("malformed task-results envelope, dropping", zap.Error(err))
		_ = d.Ack(false)
		return
	}

	if err := rc.handler(ctx, &msg); err != nil {
		rc.log.Warn("task result handling failed, requeueing",
			zap.Error(err), zap.String("envelope_id", msg.ID))
		_ = d.Nack(false, true)
		return
	}

	_ = d.Ack(false)
}
