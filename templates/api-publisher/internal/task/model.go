package task

import (
	"encoding/json"
	"errors"
	"time"
)

// Message is the frozen-shape task envelope published to the broker. It is
// stored whole in the outbox and published verbatim by the relay, so its
// JSON shape is a wire contract — see CONTRACTS.md. Consumers MUST tolerate
// unknown extra fields and MUST NOT require metadata.
type Message struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`
	Timestamp     time.Time         `json:"timestamp"`
	CorrelationID string            `json:"correlation_id"`
	Payload       json.RawMessage   `json:"payload"`
	Metadata      map[string]string `json:"metadata"`
	Priority      int32             `json:"priority"`
}

// RoutingConfig maps a routing key to the broker resources that ALREADY
// exist for it. Topology (exchanges, queues, bindings, DLQ/TTL args) is
// owned by your broker provisioning (e.g. a definitions.json loaded at
// boot); this map is only the publish-side lookup — the API verifies
// passively at connect and never declares.
type RoutingConfig struct {
	Exchange    string
	Queue       string
	Prefetch    int
	Description string
}

// ErrUnknownRoutingKey is returned for a routing key that is not in
// DefaultRoutingMap. There is no fallback: an unknown key is a publisher
// bug and fails fast. Handlers map it to HTTP 400.
var ErrUnknownRoutingKey = errors.New("unknown routing key: not one of the contract task types")

// IsContractTaskType reports whether s is one of the routing keys declared
// in DefaultRoutingMap (see tasktypes.go).
func IsContractTaskType(s string) bool {
	_, ok := DefaultRoutingMap[s]
	return ok
}
