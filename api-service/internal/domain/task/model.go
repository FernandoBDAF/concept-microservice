package task

import (
	"encoding/json"
	"time"
)

// Message represents a message in the queue
type Message struct {
	ID            string            `json:"id" validate:"required"`
	Type          string            `json:"type" validate:"required"`
	Timestamp     time.Time         `json:"timestamp" validate:"required"`
	CorrelationID string            `json:"correlation_id"`
	Payload       json.RawMessage   `json:"payload" validate:"required"`
	Metadata      map[string]string `json:"metadata"`
	Priority      int32             `json:"priority" validate:"min=0,max=9"`
}

// RoutingConfig maps a routing key to the broker resources that already
// exist for it. Topology (exchanges, queues, bindings, TTL/retry args) is
// owned by deploy/rabbitmq/definitions.json (ADR-008.4); this map is only
// the publish/consume-side lookup — services verify passively and never
// declare.
type RoutingConfig struct {
	Exchange    string
	Queue       string
	Prefetch    int
	Description string
}

// DefaultRoutingMap holds the four contract task types (ADR-008.6). There
// is no fallback: unknown routing keys are a publisher bug and fail fast.
// Names mirror deploy/rabbitmq/ROUTING_KEYS.generated.md.
var DefaultRoutingMap = map[string]RoutingConfig{
	"profile.task": {
		Exchange:    "profile-tasks",
		Queue:       "profile-processing",
		Prefetch:    2,
		Description: "Profile processing tasks",
	},
	"email.send": {
		Exchange:    "email-tasks",
		Queue:       "email-processing",
		Prefetch:    5,
		Description: "Email sending tasks",
	},
	"image.process": {
		Exchange:    "image-tasks",
		Queue:       "image-processing",
		Prefetch:    1,
		Description: "Image processing tasks",
	},
	"document.process": {
		Exchange:    "document-tasks",
		Queue:       "document-processing",
		Prefetch:    1,
		Description: "Document processing tasks for GraphRAG",
	},
}

// IsContractTaskType reports whether s is one of the four contract task
// types / routing keys (ADR-008.6 whitelist).
func IsContractTaskType(s string) bool {
	_, ok := DefaultRoutingMap[s]
	return ok
}
