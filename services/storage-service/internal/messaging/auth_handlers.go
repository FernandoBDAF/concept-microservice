package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// AuthHandler handles authentication-related messages from the queue
type AuthHandler struct {
	authService *service.AuthService
	log         *zap.Logger
}

// NewAuthHandler creates a new auth message handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         logger.Get().Named("auth_message_handler"),
	}
}

// Handle processes auth-related messages based on routing key
func (h *AuthHandler) Handle(ctx context.Context, msg *Message) (*MessageResponse, error) {
	startTime := time.Now()
	h.log.Info("Processing auth message",
		logger.String("routing_key", msg.RoutingKey),
		logger.String("message_id", msg.ID),
		logger.String("message_type", msg.Type),
	)

	var response *MessageResponse
	var err error

	switch msg.RoutingKey {
	case "auth.user.create":
		response, err = h.handleUserCreate(ctx, msg, startTime)
	case "auth.user.update":
		response, err = h.handleUserUpdate(ctx, msg, startTime)
	case "auth.user.delete":
		response, err = h.handleUserDelete(ctx, msg, startTime)
	case "auth.user.authenticate":
		response, err = h.handleUserAuthenticate(ctx, msg, startTime)
	case "auth.audit.log":
		response, err = h.handleAuditLog(ctx, msg, startTime)
	case "auth.role.create":
		response, err = h.handleRoleCreate(ctx, msg, startTime)
	case "auth.role.list":
		response, err = h.handleRoleList(ctx, msg, startTime)
	default:
		h.log.Error("Unsupported routing key for auth handler",
			logger.String("routing_key", msg.RoutingKey),
			logger.String("message_id", msg.ID),
		)
		return h.createErrorResponse(msg, fmt.Errorf("unsupported auth routing key: %s", msg.RoutingKey)), nil
	}

	if err != nil {
		h.log.Error("Failed to process auth message",
			logger.String("routing_key", msg.RoutingKey),
			logger.String("message_id", msg.ID),
			logger.ErrorField(err),
			logger.Duration("duration", time.Since(startTime)),
		)
		return h.createErrorResponse(msg, err), nil
	}

	h.log.Info("Successfully processed auth message",
		logger.String("routing_key", msg.RoutingKey),
		logger.String("message_id", msg.ID),
		logger.Duration("duration", time.Since(startTime)),
	)

	return response, nil
}

// handleUserCreate processes auth.user.create messages
func (h *AuthHandler) handleUserCreate(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing user create message",
		logger.String("message_id", msg.ID),
	)

	var req models.AuthUserRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user create request: %w", err)
	}

	user, err := h.authService.CreateUser(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &MessageResponse{
		MessageID:      fmt.Sprintf("resp_%s", msg.ID),
		Success:        true,
		Result:         map[string]interface{}{"user": user},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleUserUpdate processes auth.user.update messages
func (h *AuthHandler) handleUserUpdate(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing user update message",
		logger.String("message_id", msg.ID),
	)

	var updateData struct {
		UserID string                 `json:"user_id"`
		Data   models.AuthUserRequest `json:"data"`
	}

	if err := json.Unmarshal(msg.Payload, &updateData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user update request: %w", err)
	}

	user, err := h.authService.UpdateUser(ctx, updateData.UserID, &updateData.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &MessageResponse{
		MessageID:      fmt.Sprintf("resp_%s", msg.ID),
		Success:        true,
		Result:         map[string]interface{}{"user": user},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleUserDelete processes auth.user.delete messages
func (h *AuthHandler) handleUserDelete(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing user delete message",
		logger.String("message_id", msg.ID),
	)

	var deleteData struct {
		UserID string `json:"user_id"`
	}

	if err := json.Unmarshal(msg.Payload, &deleteData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user delete request: %w", err)
	}

	err := h.authService.DeleteUser(ctx, deleteData.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	return &MessageResponse{
		MessageID:      fmt.Sprintf("resp_%s", msg.ID),
		Success:        true,
		Result:         map[string]interface{}{"message": "User deleted successfully"},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleUserAuthenticate processes auth.user.authenticate messages
func (h *AuthHandler) handleUserAuthenticate(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing user authenticate message",
		logger.String("message_id", msg.ID),
	)

	var authData struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		IPAddress string `json:"ip_address"`
		UserAgent string `json:"user_agent"`
	}

	if err := json.Unmarshal(msg.Payload, &authData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal authenticate request: %w", err)
	}

	user, err := h.authService.AuthenticateUser(ctx, authData.Email, authData.Password,
		authData.IPAddress, authData.UserAgent)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"user":    user,
			"success": true,
			"message": "Authentication successful",
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleAuditLog processes auth.audit.log messages
func (h *AuthHandler) handleAuditLog(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing audit log message",
		logger.String("message_id", msg.ID),
	)

	var req models.AuthAuditLogRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit log request: %w", err)
	}

	err := h.authService.CreateAuditLog(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return &MessageResponse{
		MessageID:      fmt.Sprintf("resp_%s", msg.ID),
		Success:        true,
		Result:         map[string]interface{}{"message": "Audit log created successfully"},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleRoleCreate processes auth.role.create messages
func (h *AuthHandler) handleRoleCreate(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing role create message",
		logger.String("message_id", msg.ID),
	)

	var req models.AuthRoleRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal role create request: %w", err)
	}

	role, err := h.authService.CreateRole(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return &MessageResponse{
		MessageID:      fmt.Sprintf("resp_%s", msg.ID),
		Success:        true,
		Result:         map[string]interface{}{"role": role},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleRoleList processes auth.role.list messages
func (h *AuthHandler) handleRoleList(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing role list message",
		logger.String("message_id", msg.ID),
	)

	roles, err := h.authService.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"roles": roles,
			"count": len(roles),
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// createErrorResponse creates a standardized error response
func (h *AuthHandler) createErrorResponse(msg *Message, err error) *MessageResponse {
	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   false,
		Error:     err.Error(),
		Result: map[string]interface{}{
			"error":   err.Error(),
			"success": false,
			"type":    "auth_error",
		},
		ProcessedAt: time.Now(),
	}
}
