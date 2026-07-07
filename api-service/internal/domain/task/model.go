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

type RoutingConfig struct {
	Exchange      string
	Queue         string
	TTL           time.Duration
	Prefetch      int
	Durable       bool
	AutoDelete    bool
	Exclusive     bool
	NoWait        bool
	DeadLetterTTL time.Duration
	MaxRetries    int
	Description   string
}

var DefaultRoutingMap = map[string]RoutingConfig{
	"profile.task": {
		Exchange:      "profile-tasks",
		Queue:         "profile-processing",
		TTL:           1 * time.Hour,
		Prefetch:      2,
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		DeadLetterTTL: 24 * time.Hour,
		MaxRetries:    3,
		Description:   "Profile processing tasks with standard TTL and moderate prefetch",
	},
	"email.send": {
		Exchange:      "email-tasks",
		Queue:         "email-processing",
		TTL:           1 * time.Hour,
		Prefetch:      5,
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		DeadLetterTTL: 24 * time.Hour,
		MaxRetries:    5,
		Description:   "Email sending tasks with short TTL and high prefetch",
	},
	"image.process": {
		Exchange:      "image-tasks",
		Queue:         "image-processing",
		TTL:           6 * time.Hour,
		Prefetch:      1,
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		DeadLetterTTL: 3 * 24 * time.Hour,
		MaxRetries:    2,
		Description:   "Image processing tasks with long TTL and low prefetch",
	},
	"document.process": {
		Exchange:      "document-tasks",
		Queue:         "document-processing",
		TTL:           12 * time.Hour,
		Prefetch:      1,
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		DeadLetterTTL: 7 * 24 * time.Hour,
		MaxRetries:    3,
		Description:   "Document processing tasks for GraphRAG",
	},
}
