package profile

import (
	"errors"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
)

var (
	ErrInvalidTaskType  = errors.New("invalid task type")
	ErrInvalidProfileID = errors.New("invalid profile id")
)

// TaskType represents the type of profile task to process
type TaskType string

const (
	TaskTypeSync     TaskType = "sync"
	TaskTypeValidate TaskType = "validate"
	TaskTypeEnrich   TaskType = "enrich"
)

// ProfilePayload contains profile task data
type ProfilePayload struct {
	TaskType  TaskType          `json:"task_type"`
	ProfileID string            `json:"profile_id"`
	UserID    string            `json:"user_id,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

// ProfileMessage represents a profile task message
type ProfileMessage struct {
	Type      string         `json:"type"`
	Payload   ProfilePayload `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`

	queue.Message
}

// NewProfileMessage creates a profile message from queue message
func NewProfileMessage(msg *queue.Message) (*ProfileMessage, error) {
	var profileMsg ProfileMessage
	if err := msg.UnmarshalPayload(&profileMsg); err != nil {
		return nil, err
	}

	profileMsg.Message = *msg
	return &profileMsg, nil
}

// Validate validates the profile message
func (m *ProfileMessage) Validate() error {
	if m.Type != "profile" {
		return errors.New("message type must be 'profile'")
	}

	if m.Payload.ProfileID == "" {
		return ErrInvalidProfileID
	}

	switch m.Payload.TaskType {
	case TaskTypeSync, TaskTypeValidate, TaskTypeEnrich:
		return nil
	default:
		return ErrInvalidTaskType
	}
}
