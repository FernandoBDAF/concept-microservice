package models

import (
	"time"

	"github.com/google/uuid"
)

// TaskRequest represents a request to submit a task
type TaskRequest struct {
	Type    string      `json:"type" binding:"required"`
	Payload interface{} `json:"payload" binding:"required"`
}

// Task represents a task in the system
type Task struct {
	ID        uuid.UUID   `json:"id"`
	ProfileID string      `json:"profile_id"`
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// TaskResponse represents a response containing task information
type TaskResponse struct {
	Task  *Task  `json:"task,omitempty"`
	Error string `json:"error,omitempty"`
}
