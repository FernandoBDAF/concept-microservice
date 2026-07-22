package task

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BuildEnvelope constructs the frozen-shape task envelope. It is the SINGLE
// construction point for every publish path, so any two callers that submit
// the same task type produce byte-shape-identical envelopes — the property
// that lets the outbox store an envelope and the relay publish it verbatim.
//
// Every envelope carries metadata.source and a trace_id. If you run
// distributed tracing, inject the active span's W3C traceparent into
// metadata here (and set trace_id to the real trace ID) so consumers can
// continue the trace from the envelope; this template keeps the minimal
// correlation-id fallback and leaves that seam to the adopter.
func BuildEnvelope(routingKey, msgType string, payload interface{}, metadata map[string]string) (*Message, error) {
	if !IsContractTaskType(routingKey) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRoutingKey, routingKey)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	correlationID := uuid.New().String()

	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["source"] = "api-publisher"
	if _, ok := metadata["trace_id"]; !ok {
		metadata["trace_id"] = correlationID
	}

	return &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Payload:       body,
		Metadata:      metadata,
		Priority:      0,
	}, nil
}
