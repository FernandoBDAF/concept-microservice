package models

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"time"

	"github.com/google/uuid"
)

// TaskRequest represents a request to submit a task
// ✅ UPDATED: Support new task types for multi-worker architecture
type TaskRequest struct {
	Type    string      `json:"type" binding:"required,oneof=profile_update email_notification image_processing"`
	Payload interface{} `json:"payload" binding:"required"`
}

// ✅ NEW: Validate method for TaskRequest
func (tr *TaskRequest) Validate() error {
	validTypes := map[string]bool{
		"profile_update":     true, // → Profile Worker
		"email_notification": true, // → Email Worker
		"image_processing":   true, // → Image Worker
	}

	if !validTypes[tr.Type] {
		return &TaskError{
			Code:    400,
			Message: "Invalid task type. Supported types: profile_update, email_notification, image_processing",
		}
	}

	if tr.Payload == nil {
		return &TaskError{
			Code:    400,
			Message: "Task payload is required",
		}
	}

	// ✅ NEW: Task-specific payload validation
	switch tr.Type {
	case "email_notification":
		return tr.validateEmailPayload()
	case "image_processing":
		return tr.validateImagePayload()
	case "profile_update":
		return tr.validateProfilePayload()
	}

	return nil
}

// ✅ NEW: Email payload validation
func (tr *TaskRequest) validateEmailPayload() error {
	payloadMap, ok := tr.Payload.(map[string]interface{})
	if !ok {
		return &TaskError{
			Code:    400,
			Message: "Email task payload must be an object",
		}
	}

	// Validate required email fields
	to, exists := payloadMap["to"].(string)
	if !exists || to == "" {
		return &TaskError{
			Code:    400,
			Message: "Email task requires 'to' field with valid email address",
		}
	}

	// Validate email format
	if _, err := mail.ParseAddress(to); err != nil {
		return &TaskError{
			Code:    400,
			Message: "Invalid email address format in 'to' field",
		}
	}

	template, exists := payloadMap["template"].(string)
	if !exists || template == "" {
		return &TaskError{
			Code:    400,
			Message: "Email task requires 'template' field",
		}
	}

	return nil
}

// ✅ NEW: Image payload validation
func (tr *TaskRequest) validateImagePayload() error {
	payloadMap, ok := tr.Payload.(map[string]interface{})
	if !ok {
		return &TaskError{
			Code:    400,
			Message: "Image task payload must be an object",
		}
	}

	imageURL, exists := payloadMap["image_url"].(string)
	if !exists || imageURL == "" {
		return &TaskError{
			Code:    400,
			Message: "Image task requires 'image_url' field",
		}
	}

	operation, exists := payloadMap["operation"].(string)
	if !exists || operation == "" {
		return &TaskError{
			Code:    400,
			Message: "Image task requires 'operation' field",
		}
	}

	validOps := map[string]bool{"resize": true, "convert": true, "optimize": true}
	if !validOps[operation] {
		return &TaskError{
			Code:    400,
			Message: "Invalid image operation. Supported: resize, convert, optimize",
		}
	}

	return nil
}

// ✅ NEW: Profile payload validation
func (tr *TaskRequest) validateProfilePayload() error {
	payloadMap, ok := tr.Payload.(map[string]interface{})
	if !ok {
		return &TaskError{
			Code:    400,
			Message: "Profile task payload must be an object",
		}
	}

	userID, exists := payloadMap["user_id"].(string)
	if !exists || userID == "" {
		return &TaskError{
			Code:    400,
			Message: "Profile task requires 'user_id' field",
		}
	}

	action, exists := payloadMap["action"].(string)
	if !exists || action == "" {
		return &TaskError{
			Code:    400,
			Message: "Profile task requires 'action' field",
		}
	}

	validActions := map[string]bool{"create": true, "update": true, "delete": true, "sync": true}
	if !validActions[action] {
		return &TaskError{
			Code:    400,
			Message: "Invalid profile action. Supported: create, update, delete, sync",
		}
	}

	return nil
}

// ✅ NEW: Task-specific request models for different worker types

// ProfileTaskPayload represents payload for profile processing tasks
type ProfileTaskPayload struct {
	UserID string                 `json:"user_id" binding:"required"`
	Action string                 `json:"action" binding:"required,oneof=create update delete sync"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// EmailTaskPayload represents payload for email notification tasks
type EmailTaskPayload struct {
	To       string                 `json:"to" binding:"required,email"`
	Template string                 `json:"template" binding:"required"`
	Subject  string                 `json:"subject,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Priority int                    `json:"priority,omitempty"` // ✅ NEW: Email priority (1=high, 2=normal, 3=low)
}

// ✅ NEW: Validate method for EmailTaskPayload
func (etp *EmailTaskPayload) Validate() error {
	if etp.To == "" {
		return &TaskError{Code: 400, Message: "Email 'to' field is required"}
	}

	if _, err := mail.ParseAddress(etp.To); err != nil {
		return &TaskError{Code: 400, Message: "Invalid email address format"}
	}

	if etp.Template == "" {
		return &TaskError{Code: 400, Message: "Email 'template' field is required"}
	}

	// Validate priority if provided (0 means not set, which is valid)
	if etp.Priority != 0 && (etp.Priority < 1 || etp.Priority > 3) {
		return &TaskError{Code: 400, Message: "Email priority must be 1 (high), 2 (normal), or 3 (low)"}
	}

	return nil
}

// ImageTaskPayload represents payload for image processing tasks
type ImageTaskPayload struct {
	ImageURL     string                 `json:"image_url" binding:"required,url"`
	Operation    string                 `json:"operation" binding:"required,oneof=resize convert optimize"`
	Options      map[string]interface{} `json:"options,omitempty"`
	OutputFormat string                 `json:"output_format,omitempty"` // ✅ NEW: Target format
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

// ✅ NEW: Enhanced task responses for different types

// EmailTaskResponse represents response for email notification tasks
type EmailTaskResponse struct {
	TaskID      string                 `json:"task_id"`
	ProfileID   string                 `json:"profile_id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	EmailTo     string                 `json:"email_to"`
	Template    string                 `json:"template"`
	RoutingKey  string                 `json:"routing_key"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ImageTaskResponse represents response for image processing tasks
type ImageTaskResponse struct {
	TaskID      string                 `json:"task_id"`
	ProfileID   string                 `json:"profile_id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	ImageURL    string                 `json:"image_url"`
	Operation   string                 `json:"operation"`
	RoutingKey  string                 `json:"routing_key"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ✅ NEW: TaskError represents a task-related error
type TaskError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *TaskError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// ✅ NEW: Helper method to serialize task payload to json.RawMessage
func (t *Task) SerializePayload() (json.RawMessage, error) {
	payloadBytes, err := json.Marshal(t.Payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payloadBytes), nil
}

// ✅ NEW: Helper methods for creating specific task responses

// CreateEmailTaskResponse creates a standardized email task response
func CreateEmailTaskResponse(task *Task, routingKey string) *EmailTaskResponse {
	emailPayload := task.Payload.(map[string]interface{})
	return &EmailTaskResponse{
		TaskID:      task.ID.String(),
		ProfileID:   task.ProfileID,
		Type:        task.Type,
		Status:      task.Status,
		EmailTo:     emailPayload["to"].(string),
		Template:    emailPayload["template"].(string),
		RoutingKey:  routingKey,
		ScheduledAt: task.CreatedAt,
		CreatedAt:   task.CreatedAt,
		Metadata:    make(map[string]interface{}),
	}
}

// CreateImageTaskResponse creates a standardized image task response
func CreateImageTaskResponse(task *Task, routingKey string) *ImageTaskResponse {
	imagePayload := task.Payload.(map[string]interface{})
	return &ImageTaskResponse{
		TaskID:      task.ID.String(),
		ProfileID:   task.ProfileID,
		Type:        task.Type,
		Status:      task.Status,
		ImageURL:    imagePayload["image_url"].(string),
		Operation:   imagePayload["operation"].(string),
		RoutingKey:  routingKey,
		ScheduledAt: task.CreatedAt,
		CreatedAt:   task.CreatedAt,
		Metadata:    make(map[string]interface{}),
	}
}
