package rabbitmq

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes pre-serialized envelopes to RabbitMQ. It exists for
// the outbox relay (ADR-008.3): envelopes are stored whole in the outbox
// table, so the relay publishes the stored bytes with confirms and needs no
// domain knowledge. It satisfies outbox.Publisher.
type Publisher struct {
	client *Client
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

// envelopeHeaderFields is the minimal slice of the stored envelope needed
// to restore broker-level metadata at relay time. Pre-outbox, the direct
// publish path mirrored envelope metadata into AMQP headers and workers
// continue traces FROM those headers (operational-workers
// queue/trace.go extractTraceContext reads W3C traceparent/tracestate and
// baggage); the relay must preserve that contract or v3 end-to-end tracing
// breaks at the broker hop.
type envelopeHeaderFields struct {
	ID       string `json:"id"`
	Metadata struct {
		Traceparent string `json:"traceparent"`
		Tracestate  string `json:"tracestate"`
		Baggage     string `json:"baggage"`
	} `json:"metadata"`
}

// headersFromEnvelope derives the AMQP headers and MessageId the old direct
// publish path set, from the stored envelope JSON. Best-effort by design:
// a malformed or header-less envelope publishes with nil headers and an
// empty message id rather than failing — header restoration is parity, not
// a gate (consumers also carry the same fields inside the envelope body).
func headersFromEnvelope(body []byte) (amqp.Table, string) {
	var env envelopeHeaderFields
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, ""
	}

	var headers amqp.Table
	set := func(key, value string) {
		if value == "" {
			return
		}
		if headers == nil {
			headers = amqp.Table{}
		}
		headers[key] = value
	}
	set("traceparent", env.Metadata.Traceparent)
	set("tracestate", env.Metadata.Tracestate)
	set("baggage", env.Metadata.Baggage)

	return headers, env.ID
}

// PublishRaw publishes body on routingKey in confirm mode, restoring the
// trace-propagation headers and MessageId from the envelope. No topology is
// declared (ADR-008.4) — the broker owns it via definitions.json.
func (p *Publisher) PublishRaw(ctx context.Context, routingKey string, body []byte) error {
	headers, messageID := headersFromEnvelope(body)
	return p.client.Publish(ctx, routingKey, body, headers, messageID)
}
