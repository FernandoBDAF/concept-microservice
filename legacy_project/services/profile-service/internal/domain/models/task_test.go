package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestTaskRequest_Validate tests the enhanced task request validation logic
func TestTaskRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		taskRequest *TaskRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid profile_update task",
			taskRequest: &TaskRequest{
				Type: "profile_update",
				Payload: map[string]interface{}{
					"user_id": "123",
					"action":  "update",
				},
			},
			expectError: false,
		},
		{
			name: "Valid email_notification task",
			taskRequest: &TaskRequest{
				Type: "email_notification",
				Payload: map[string]interface{}{
					"to":       "user@example.com",
					"template": "welcome",
				},
			},
			expectError: false,
		},
		{
			name: "Valid image_processing task",
			taskRequest: &TaskRequest{
				Type: "image_processing",
				Payload: map[string]interface{}{
					"image_url": "https://example.com/image.jpg",
					"operation": "resize",
				},
			},
			expectError: false,
		},
		{
			name: "Invalid task type",
			taskRequest: &TaskRequest{
				Type:    "unsupported_task",
				Payload: map[string]interface{}{"data": "test"},
			},
			expectError: true,
			errorMsg:    "Invalid task type. Supported types: profile_update, email_notification, image_processing",
		},
		{
			name: "Empty task type",
			taskRequest: &TaskRequest{
				Type:    "",
				Payload: map[string]interface{}{"data": "test"},
			},
			expectError: true,
			errorMsg:    "Invalid task type. Supported types: profile_update, email_notification, image_processing",
		},
		{
			name: "Nil payload",
			taskRequest: &TaskRequest{
				Type:    "profile_update",
				Payload: nil,
			},
			expectError: true,
			errorMsg:    "Task payload is required",
		},
		// ✅ NEW: Email-specific validation tests
		{
			name: "Email task with invalid email format",
			taskRequest: &TaskRequest{
				Type: "email_notification",
				Payload: map[string]interface{}{
					"to":       "invalid-email",
					"template": "welcome",
				},
			},
			expectError: true,
			errorMsg:    "Invalid email address format",
		},
		{
			name: "Email task missing template",
			taskRequest: &TaskRequest{
				Type: "email_notification",
				Payload: map[string]interface{}{
					"to": "user@example.com",
				},
			},
			expectError: true,
			errorMsg:    "Email task requires 'template' field",
		},
		// ✅ NEW: Image-specific validation tests
		{
			name: "Image task with invalid operation",
			taskRequest: &TaskRequest{
				Type: "image_processing",
				Payload: map[string]interface{}{
					"image_url": "https://example.com/image.jpg",
					"operation": "invalid_op",
				},
			},
			expectError: true,
			errorMsg:    "Invalid image operation",
		},
		{
			name: "Image task missing image_url",
			taskRequest: &TaskRequest{
				Type: "image_processing",
				Payload: map[string]interface{}{
					"operation": "resize",
				},
			},
			expectError: true,
			errorMsg:    "Image task requires 'image_url' field",
		},
		// ✅ NEW: Profile-specific validation tests
		{
			name: "Profile task with invalid action",
			taskRequest: &TaskRequest{
				Type: "profile_update",
				Payload: map[string]interface{}{
					"user_id": "123",
					"action":  "invalid_action",
				},
			},
			expectError: true,
			errorMsg:    "Invalid profile action",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.taskRequest.Validate()

			if test.expectError {
				assert.Error(t, err, "Expected validation error for test case: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg,
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no validation error for test case: %s", test.name)
			}
		})
	}
}

// ✅ NEW: Enhanced EmailTaskPayload validation tests
func TestEmailTaskPayload_Validation(t *testing.T) {
	tests := []struct {
		name        string
		payload     *EmailTaskPayload
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid email payload",
			payload: &EmailTaskPayload{
				To:       "user@example.com",
				Template: "welcome",
				Subject:  "Welcome!",
				Priority: 2,
			},
			expectError: false,
		},
		{
			name: "Valid email payload with high priority",
			payload: &EmailTaskPayload{
				To:       "user@example.com",
				Template: "urgent",
				Priority: 1,
			},
			expectError: false,
		},
		{
			name: "Valid email payload with default priority",
			payload: &EmailTaskPayload{
				To:       "user@example.com",
				Template: "welcome",
				Priority: 0, // Default (not set) - should be valid
			},
			expectError: false,
		},
		{
			name: "Empty email address",
			payload: &EmailTaskPayload{
				To:       "",
				Template: "welcome",
			},
			expectError: true,
			errorMsg:    "Email 'to' field is required",
		},
		{
			name: "Invalid email format",
			payload: &EmailTaskPayload{
				To:       "invalid-email",
				Template: "welcome",
			},
			expectError: true,
			errorMsg:    "Invalid email address format",
		},
		{
			name: "Missing template",
			payload: &EmailTaskPayload{
				To:       "user@example.com",
				Template: "",
			},
			expectError: true,
			errorMsg:    "Email 'template' field is required",
		},
		{
			name: "Invalid priority too high",
			payload: &EmailTaskPayload{
				To:       "user@example.com",
				Template: "welcome",
				Priority: 4, // Invalid
			},
			expectError: true,
			errorMsg:    "Email priority must be 1 (high), 2 (normal), or 3 (low)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.payload.Validate()

			if test.expectError {
				assert.Error(t, err, "Expected validation error for test case: %s", test.name)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg,
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no validation error for test case: %s", test.name)
			}
		})
	}
}

// TestTaskPayloadModels tests the task-specific payload models
func TestProfileTaskPayload_Validation(t *testing.T) {
	// Test ProfileTaskPayload structure
	payload := &ProfileTaskPayload{
		UserID: "user-123",
		Action: "update",
		Data: map[string]interface{}{
			"name": "John Doe",
		},
	}

	assert.Equal(t, "user-123", payload.UserID)
	assert.Equal(t, "update", payload.Action)
	assert.NotNil(t, payload.Data)
}

func TestEmailTaskPayload_Structure(t *testing.T) {
	// Test EmailTaskPayload structure
	payload := &EmailTaskPayload{
		To:       "user@example.com",
		Template: "welcome",
		Subject:  "Welcome!",
		Priority: 2,
		Data: map[string]interface{}{
			"username": "johndoe",
		},
	}

	assert.Equal(t, "user@example.com", payload.To)
	assert.Equal(t, "welcome", payload.Template)
	assert.Equal(t, "Welcome!", payload.Subject)
	assert.Equal(t, 2, payload.Priority)
	assert.NotNil(t, payload.Data)
}

func TestImageTaskPayload_Structure(t *testing.T) {
	// Test ImageTaskPayload structure
	payload := &ImageTaskPayload{
		ImageURL:     "https://example.com/image.jpg",
		Operation:    "resize",
		OutputFormat: "webp",
		Options: map[string]interface{}{
			"width":  800,
			"height": 600,
		},
	}

	assert.Equal(t, "https://example.com/image.jpg", payload.ImageURL)
	assert.Equal(t, "resize", payload.Operation)
	assert.Equal(t, "webp", payload.OutputFormat)
	assert.NotNil(t, payload.Options)
}

// ✅ NEW: Test task response creation helpers
func TestCreateEmailTaskResponse(t *testing.T) {
	task := &Task{
		ID:        uuid.New(),
		ProfileID: "profile-123",
		Type:      "email_notification",
		Status:    "pending",
		Payload: map[string]interface{}{
			"to":       "user@example.com",
			"template": "welcome",
		},
		CreatedAt: time.Now(),
	}

	routingKey := "email.send"
	response := CreateEmailTaskResponse(task, routingKey)

	assert.Equal(t, task.ID.String(), response.TaskID)
	assert.Equal(t, task.ProfileID, response.ProfileID)
	assert.Equal(t, task.Type, response.Type)
	assert.Equal(t, task.Status, response.Status)
	assert.Equal(t, "user@example.com", response.EmailTo)
	assert.Equal(t, "welcome", response.Template)
	assert.Equal(t, routingKey, response.RoutingKey)
	assert.NotNil(t, response.Metadata)
}

func TestCreateImageTaskResponse(t *testing.T) {
	task := &Task{
		ID:        uuid.New(),
		ProfileID: "profile-123",
		Type:      "image_processing",
		Status:    "pending",
		Payload: map[string]interface{}{
			"image_url": "https://example.com/image.jpg",
			"operation": "resize",
		},
		CreatedAt: time.Now(),
	}

	routingKey := "image.process"
	response := CreateImageTaskResponse(task, routingKey)

	assert.Equal(t, task.ID.String(), response.TaskID)
	assert.Equal(t, task.ProfileID, response.ProfileID)
	assert.Equal(t, task.Type, response.Type)
	assert.Equal(t, task.Status, response.Status)
	assert.Equal(t, "https://example.com/image.jpg", response.ImageURL)
	assert.Equal(t, "resize", response.Operation)
	assert.Equal(t, routingKey, response.RoutingKey)
	assert.NotNil(t, response.Metadata)
}

// TestTask_SerializePayload tests the payload serialization helper
func TestTask_SerializePayload(t *testing.T) {
	task := &Task{
		Type: "profile_update",
		Payload: map[string]interface{}{
			"user_id": "123",
			"action":  "update",
		},
	}

	rawMessage, err := task.SerializePayload()
	assert.NoError(t, err, "SerializePayload should not return error")
	assert.NotNil(t, rawMessage, "SerializePayload should return valid json.RawMessage")
	assert.Contains(t, string(rawMessage), "user_id", "Serialized payload should contain user_id")
	assert.Contains(t, string(rawMessage), "123", "Serialized payload should contain user_id value")
}
 