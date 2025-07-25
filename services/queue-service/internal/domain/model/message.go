package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
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

// NewMessage creates a new message with default values
func NewMessage(msgType string, payload json.RawMessage) *Message {
	return &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: uuid.New().String(),
		Payload:       payload,
		Metadata:      make(map[string]string),
		Priority:      0,
	}
}

// MarshalJSON implements the json.Marshaler interface
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(m),
		Timestamp: m.Timestamp.Format(time.RFC3339),
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := &struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, aux.Timestamp)
	if err != nil {
		return err
	}

	m.Timestamp = t
	return nil
}

// RoutingConfig represents routing configuration for different worker types
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

// DefaultRoutingMap provides default routing configuration for worker types
var DefaultRoutingMap = map[string]RoutingConfig{
	"profile.task": {
		Exchange:      "tasks-exchange",
		Queue:         "profile-processing",
		TTL:           24 * time.Hour,
		Prefetch:      1,
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		DeadLetterTTL: 7 * 24 * time.Hour, // 7 days in DLQ
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
		DeadLetterTTL: 24 * time.Hour, // 1 day in DLQ
		MaxRetries:    5,
		Description:   "Email sending tasks with short TTL and high prefetch for burst processing",
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
		DeadLetterTTL: 3 * 24 * time.Hour, // 3 days in DLQ
		MaxRetries:    2,
		Description:   "Image processing tasks with long TTL and low prefetch for resource-intensive operations",
	},
}

// WorkerConfig represents worker-specific configuration
type WorkerConfig struct {
	Prefetch      int
	TTL           time.Duration
	DeadLetterTTL time.Duration
	MaxRetries    int
}

// BuildRoutingMapFromConfig creates a routing configuration map from service configuration
func BuildRoutingMapFromConfig(profileConfig, emailConfig, imageConfig WorkerConfig) map[string]RoutingConfig {
	return map[string]RoutingConfig{
		"profile.task": {
			Exchange:      "tasks-exchange",
			Queue:         "profile-processing",
			TTL:           profileConfig.TTL,
			Prefetch:      profileConfig.Prefetch,
			Durable:       true,
			AutoDelete:    false,
			Exclusive:     false,
			NoWait:        false,
			DeadLetterTTL: profileConfig.DeadLetterTTL,
			MaxRetries:    profileConfig.MaxRetries,
			Description:   "Profile processing tasks with configurable TTL and prefetch",
		},
		"email.send": {
			Exchange:      "email-tasks",
			Queue:         "email-processing",
			TTL:           emailConfig.TTL,
			Prefetch:      emailConfig.Prefetch,
			Durable:       true,
			AutoDelete:    false,
			Exclusive:     false,
			NoWait:        false,
			DeadLetterTTL: emailConfig.DeadLetterTTL,
			MaxRetries:    emailConfig.MaxRetries,
			Description:   "Email sending tasks with configurable TTL and high prefetch for burst processing",
		},
		"image.process": {
			Exchange:      "image-tasks",
			Queue:         "image-processing",
			TTL:           imageConfig.TTL,
			Prefetch:      imageConfig.Prefetch,
			Durable:       true,
			AutoDelete:    false,
			Exclusive:     false,
			NoWait:        false,
			DeadLetterTTL: imageConfig.DeadLetterTTL,
			MaxRetries:    imageConfig.MaxRetries,
			Description:   "Image processing tasks with configurable TTL and low prefetch for resource-intensive operations",
		},
	}
}

// UpdateRoutingMap updates the global routing map with configuration
func UpdateRoutingMap(routingMap map[string]RoutingConfig) {
	for key, config := range routingMap {
		DefaultRoutingMap[key] = config
	}
}

// ProfileUpdate represents a profile update message payload
type ProfileUpdate struct {
	UserID    string                 `json:"user_id" validate:"required"`
	Changes   map[string]interface{} `json:"changes" validate:"required"`
	Timestamp time.Time              `json:"timestamp" validate:"required"`
}

// CacheInvalidation represents a cache invalidation message payload
type CacheInvalidation struct {
	Keys     []string `json:"keys" validate:"required"`
	Pattern  string   `json:"pattern"`
	AllCache bool     `json:"all_cache"`
}

// BackgroundJob represents a background job message payload
type BackgroundJob struct {
	JobType    string                 `json:"job_type" validate:"required"`
	Parameters map[string]interface{} `json:"parameters"`
	Priority   int32                  `json:"priority" validate:"min=0,max=9"`
}

type MessageStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}
