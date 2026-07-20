// loadgen (ADR-004.4) — AMQP load generator for queue-side experiments.
//
// Envelope-correct (graph-worker/shared/contracts/MESSAGE_FORMAT.md) so
// consumers treat generated load exactly like api-service traffic. It never
// declares topology — the broker owns it (definitions.json, ADR-008.4) — it
// only resolves the rk→exchange mapping consumers use.
//
// Target CLI (wired as an experiment step type by the runner):
//
//	loadgen -url amqp://guest:guest@rabbitmq:5672/ \
//	        -routing-key email.send -rate 200 -duration 30s -confirm
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type opts struct {
	URL        string
	RoutingKey string
	Rate       int           // messages per second
	Duration   time.Duration // total run time
	Confirm    bool          // publisher confirms on/off (throughput vs safety demo)
	Payload    string        // optional JSON payload override
}

// envelope mirrors the publisher contract (id/type/timestamp/payload/metadata).
type envelope struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Payload   json.RawMessage   `json:"payload"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// rkExchange resolves a routing key to its work exchange, mirroring
// deploy/rabbitmq/definitions.json (the same rk→exchange map the consumers
// use). loadgen publishes to the work exchange; the broker routes to the work
// queue. Unknown keys are a caller error — there is no default-tasks fallback
// (ADR-008.6).
var rkExchange = map[string]string{
	"email.send":       "email-tasks",
	"image.process":    "image-tasks",
	"profile.task":     "profile-tasks",
	"document.process": "document-tasks",
}

// defaultPayloads produce a valid payload per routing key so workers
// process (not reject) generated load. Keep in sync with the processors'
// Validate rules.
var defaultPayloads = map[string]string{
	"email.send":       `{"email_type":"notification","recipient":"loadgen@lab.local","subject":"loadgen"}`,
	"image.process":    `{"operation":"resize","source_url":"minio://documents-raw/loadgen.png","target_path":"/tmp/out.png","width":64,"height":64}`,
	"profile.task":     `{"task_type":"sync","profile_id":"00000000-0000-0000-0000-000000000001"}`,
	"document.process": `{"document_id":"00000000-0000-0000-0000-000000000002","storage_path":"loadgen/x.txt","storage_bucket":"documents-raw"}`,
}

func main() {
	var o opts
	flag.StringVar(&o.URL, "url", "amqp://guest:guest@rabbitmq:5672/", "AMQP URL")
	flag.StringVar(&o.RoutingKey, "routing-key", "email.send", "routing key (also envelope type)")
	flag.IntVar(&o.Rate, "rate", 100, "messages per second")
	flag.DurationVar(&o.Duration, "duration", 30*time.Second, "run duration")
	flag.BoolVar(&o.Confirm, "confirm", true, "wait for publisher confirms")
	flag.StringVar(&o.Payload, "payload", "", "JSON payload override (default: per-routing-key valid payload)")
	flag.Parse()

	if _, ok := rkExchange[o.RoutingKey]; !ok {
		fmt.Fprintf(os.Stderr, "unknown routing key %q (no exchange mapping; the broker owns topology, ADR-008.4)\n", o.RoutingKey)
		os.Exit(2)
	}

	if err := run(o); err != nil {
		fmt.Fprintf(os.Stderr, "loadgen: %v\n", err)
		os.Exit(1)
	}
}

func run(o opts) error {
	if o.Rate <= 0 {
		return fmt.Errorf("rate must be > 0, got %d", o.Rate)
	}
	exchange := rkExchange[o.RoutingKey]

	payload := o.Payload
	if payload == "" {
		payload = defaultPayloads[o.RoutingKey]
	}
	if !json.Valid([]byte(payload)) {
		return fmt.Errorf("payload is not valid JSON: %s", payload)
	}
	rawPayload := json.RawMessage(payload)

	conn, err := amqp.Dial(o.URL)
	if err != nil {
		return fmt.Errorf("dial %s: %w", o.URL, err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}

	var (
		sent      atomic.Int64
		confirmed atomic.Int64
		nacked    atomic.Int64
		returned  atomic.Int64
		errs      atomic.Int64
		wg        sync.WaitGroup
	)

	// Unroutable messages (mandatory publish, confirm mode) return here.
	returns := ch.NotifyReturn(make(chan amqp.Return, 128))
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range returns {
			returned.Add(1)
		}
	}()

	// Confirms are collected asynchronously (registered once per channel) so
	// the publish loop keeps to the target rate; -confirm=false skips this and
	// publishes fire-and-forget, making the throughput/safety delta observable.
	var confirms chan amqp.Confirmation
	if o.Confirm {
		if err := ch.Confirm(false); err != nil {
			_ = ch.Close()
			return fmt.Errorf("enable publisher confirms: %w", err)
		}
		confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1024))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range confirms {
				if c.Ack {
					confirmed.Add(1)
				} else {
					nacked.Add(1)
				}
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	interval := time.Second / time.Duration(o.Rate)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.After(o.Duration)
	mandatory := o.Confirm // when we care about delivery, catch unroutable

	start := time.Now()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-deadline:
			break loop
		case <-ticker.C:
			body, err := buildEnvelope(o.RoutingKey, rawPayload)
			if err != nil {
				errs.Add(1)
				continue
			}
			pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = ch.PublishWithContext(pubCtx, exchange, o.RoutingKey, mandatory, false, amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
				Body:         body,
			})
			cancel()
			if err != nil {
				errs.Add(1)
				continue
			}
			sent.Add(1)
		}
	}
	elapsed := time.Since(start)

	// Let outstanding confirms/returns land before tearing down the collectors.
	if o.Confirm {
		waitForConfirms(&sent, &confirmed, &nacked, 5*time.Second)
	}
	_ = ch.Close()
	_ = conn.Close()
	wg.Wait()

	sentN := sent.Load()
	effRate := 0.0
	if elapsed.Seconds() > 0 {
		effRate = float64(sentN) / elapsed.Seconds()
	}
	fmt.Printf("loadgen summary: rk=%s exchange=%s confirm=%v\n", o.RoutingKey, exchange, o.Confirm)
	fmt.Printf("  sent=%d confirmed=%d nacked=%d unroutable=%d errors=%d\n",
		sentN, confirmed.Load(), nacked.Load(), returned.Load(), errs.Load())
	fmt.Printf("  duration=%s effective_rate=%.1f msg/s (target %d)\n",
		elapsed.Round(time.Millisecond), effRate, o.Rate)

	if errs.Load() > 0 {
		return fmt.Errorf("%d publish error(s)", errs.Load())
	}
	if o.Confirm && (nacked.Load() > 0 || returned.Load() > 0) {
		return fmt.Errorf("delivery failures: %d nacked, %d unroutable", nacked.Load(), returned.Load())
	}
	return nil
}

func buildEnvelope(routingKey string, payload json.RawMessage) ([]byte, error) {
	return json.Marshal(envelope{
		ID:        uuid.NewString(),
		Type:      routingKey,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
		Metadata:  map[string]string{"source": "loadgen"},
	})
}

// waitForConfirms blocks until every sent message has been confirmed (ack or
// nack) or the timeout elapses, so the summary reflects the real outcome.
func waitForConfirms(sent, confirmed, nacked *atomic.Int64, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if confirmed.Load()+nacked.Load() >= sent.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
