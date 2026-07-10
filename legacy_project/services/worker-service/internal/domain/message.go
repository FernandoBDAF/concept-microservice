package domain

import (
	"errors"
	"time"

	"github.com/fernandobarroso/common/queue"
)

var (
	ErrInvalidProfileID = errors.New("invalid profile_id")
	ErrInvalidAction    = errors.New("invalid action")
	ErrInvalidData      = errors.New("invalid data")
)

type ProfileMessage struct {
	queue.Message
	ProfileID string    `json:"profile_id"`
	Action    string    `json:"action"`
	Data      any       `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *ProfileMessage) Validate() error {
	if m.ProfileID == "" {
		return ErrInvalidProfileID
	}
	if m.Action == "" {
		return ErrInvalidAction
	}
	if m.Data == nil {
		return ErrInvalidData
	}
	return nil
}
