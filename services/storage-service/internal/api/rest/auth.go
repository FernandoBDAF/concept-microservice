package rest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// AuthHandler handles auth-related HTTP requests
type AuthHandler struct {
	authService *service.AuthService
	log         *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         logger.Get().Named("auth_handler"),
	}
}

// RegisterRoutes registers auth routes with the router
func (h *AuthHandler) RegisterRoutes(router *mux.Router) {
	// User management routes
	router.HandleFunc("/api/v1/auth/users", h.CreateUser).Methods("POST")
	router.HandleFunc("/api/v1/auth/users", h.ListUsers).Methods("GET")
	router.HandleFunc("/api/v1/auth/users/{id}", h.GetUser).Methods("GET")
	router.HandleFunc("/api/v1/auth/users/{id}", h.UpdateUser).Methods("PUT")
	router.HandleFunc("/api/v1/auth/users/{id}", h.DeleteUser).Methods("DELETE")
	router.HandleFunc("/api/v1/auth/users/email/{email}", h.GetUserByEmail).Methods("GET")

	// Authentication routes
	router.HandleFunc("/api/v1/auth/authenticate", h.AuthenticateUser).Methods("POST")
	router.HandleFunc("/api/v1/auth/users/{id}/login", h.RecordLoginAttempt).Methods("POST")

	// Audit log routes
	router.HandleFunc("/api/v1/auth/audit", h.CreateAuditLog).Methods("POST")
	router.HandleFunc("/api/v1/auth/audit", h.GetAuditLogs).Methods("GET")

	// Role management routes
	router.HandleFunc("/api/v1/auth/roles", h.CreateRole).Methods("POST")
	router.HandleFunc("/api/v1/auth/roles", h.ListRoles).Methods("GET")
	router.HandleFunc("/api/v1/auth/roles/{id}", h.GetRole).Methods("GET")
}

// User Management Endpoints

// CreateUser creates a new user
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Creating user via REST API")

	var req models.AuthUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.authService.CreateUser(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    user,
		"message": "User created successfully",
	})
}

// GetUser retrieves a user by ID
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	h.log.Debug("Getting user via REST API", logger.String("user_id", userID))

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    user,
	})
}

// GetUserByEmail retrieves a user by email
func (h *AuthHandler) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	email := vars["email"]

	h.log.Debug("Getting user by email via REST API", logger.String("email", email))

	user, err := h.authService.GetUserByEmail(r.Context(), email)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user by email")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    user,
	})
}

// UpdateUser updates a user
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	h.log.Info("Updating user via REST API", logger.String("user_id", userID))

	var req models.AuthUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.authService.UpdateUser(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    user,
		"message": "User updated successfully",
	})
}

// DeleteUser deletes a user
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	h.log.Info("Deleting user via REST API", logger.String("user_id", userID))

	err := h.authService.DeleteUser(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to delete user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

// ListUsers lists users with pagination
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Listing users via REST API")

	// Parse pagination parameters
	page, pageSize := h.parsePagination(r)

	users, err := h.authService.ListUsers(r.Context(), page, pageSize)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    users,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"count":     len(users),
		},
	})
}

// Authentication Endpoints

// AuthenticateUser authenticates a user with email and password
func (h *AuthHandler) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Authenticating user via REST API")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode authentication request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get IP address and user agent
	ipAddress := h.getClientIP(r)
	userAgent := r.UserAgent()

	user, err := h.authService.AuthenticateUser(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		h.handleServiceError(w, err, "Authentication failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    user,
		"message": "Authentication successful",
	})
}

// RecordLoginAttempt records a login attempt for audit purposes
func (h *AuthHandler) RecordLoginAttempt(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	h.log.Debug("Recording login attempt via REST API", logger.String("user_id", userID))

	var req models.LoginAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode login attempt request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Override userID from URL
	req.UserID = userID

	// Get IP address if not provided
	if req.IPAddress == "" {
		req.IPAddress = h.getClientIP(r)
	}

	// Get user agent if not provided
	if req.UserAgent == "" {
		req.UserAgent = r.UserAgent()
	}

	if err := req.Validate(); err != nil {
		h.log.Error("Invalid login attempt request", logger.ErrorField(err))
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// This would typically be called by auth-service after a login attempt
	// For now, we'll create a simple audit log entry
	auditReq := &models.AuthAuditLogRequest{
		UserID:    &req.UserID,
		Action:    models.AuditActionLogin,
		Resource:  models.ResourceUser,
		IPAddress: req.IPAddress,
		UserAgent: req.UserAgent,
		Success:   req.Success,
		Details: map[string]interface{}{
			"reason": req.Reason,
		},
	}

	if !req.Success {
		auditReq.Action = models.AuditActionLoginFailed
	}

	if err := h.authService.CreateAuditLog(r.Context(), auditReq); err != nil {
		h.handleServiceError(w, err, "Failed to record login attempt")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Login attempt recorded successfully",
	})
}

// Audit Log Endpoints

// CreateAuditLog creates an audit log entry
func (h *AuthHandler) CreateAuditLog(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Creating audit log via REST API")

	var req models.AuthAuditLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode audit log request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set IP address if not provided
	if req.IPAddress == "" {
		req.IPAddress = h.getClientIP(r)
	}

	// Set user agent if not provided
	if req.UserAgent == "" {
		req.UserAgent = r.UserAgent()
	}

	if err := h.authService.CreateAuditLog(r.Context(), &req); err != nil {
		h.handleServiceError(w, err, "Failed to create audit log")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Audit log created successfully",
	})
}

// GetAuditLogs retrieves audit logs with filtering
func (h *AuthHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Getting audit logs via REST API")

	// Parse query parameters
	query := r.URL.Query()
	var userID *string
	if uid := query.Get("user_id"); uid != "" {
		userID = &uid
	}

	action := query.Get("action")

	var success *bool
	if s := query.Get("success"); s != "" {
		if successBool, err := strconv.ParseBool(s); err == nil {
			success = &successBool
		}
	}

	page, pageSize := h.parsePagination(r)

	logs, err := h.authService.GetAuditLogs(r.Context(), userID, action, success, page, pageSize)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get audit logs")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    logs,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"count":     len(logs),
		},
		"filters": map[string]interface{}{
			"user_id": userID,
			"action":  action,
			"success": success,
		},
	})
}

// Role Management Endpoints

// CreateRole creates a new role
func (h *AuthHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Creating role via REST API")

	var req models.AuthRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode role request", logger.ErrorField(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	role, err := h.authService.CreateRole(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create role")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    role,
		"message": "Role created successfully",
	})
}

// GetRole retrieves a role by ID
func (h *AuthHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	h.log.Debug("Getting role via REST API", logger.String("role_id", roleID))

	role, err := h.authService.GetRoleByID(r.Context(), roleID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get role")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    role,
	})
}

// ListRoles lists all roles
func (h *AuthHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Listing roles via REST API")

	roles, err := h.authService.ListRoles(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to list roles")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    roles,
		"count":   len(roles),
	})
}

// Helper Methods

// parsePagination parses pagination parameters from request
func (h *AuthHandler) parsePagination(r *http.Request) (page, pageSize int) {
	query := r.URL.Query()

	page = 1
	if p := query.Get("page"); p != "" {
		if pageInt, err := strconv.Atoi(p); err == nil && pageInt > 0 {
			page = pageInt
		}
	}

	pageSize = 20 // Default page size
	if ps := query.Get("page_size"); ps != "" {
		if pageSizeInt, err := strconv.Atoi(ps); err == nil && pageSizeInt > 0 && pageSizeInt <= 100 {
			pageSize = pageSizeInt
		}
	}

	return page, pageSize
}

// getClientIP extracts the client IP address from the request
func (h *AuthHandler) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, get the first one
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if remoteAddr := r.RemoteAddr; remoteAddr != "" {
		// RemoteAddr includes port, remove it
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			return host
		}
		return remoteAddr
	}

	return "127.0.0.1" // Default fallback
}

// handleServiceError handles service errors and converts them to appropriate HTTP responses
func (h *AuthHandler) handleServiceError(w http.ResponseWriter, err error, message string) {
	h.log.Error(message, logger.ErrorField(err))

	var statusCode int
	var errorMessage string

	switch {
	case err == service.ErrInvalidRequest:
		statusCode = http.StatusBadRequest
		errorMessage = "Invalid request"
	case err == service.ErrUserNotFound:
		statusCode = http.StatusNotFound
		errorMessage = "User not found"
	case err == service.ErrUserAlreadyExists:
		statusCode = http.StatusConflict
		errorMessage = "User already exists"
	case err == service.ErrUserAccountLocked:
		statusCode = http.StatusLocked
		errorMessage = "User account is locked"
	case err == service.ErrUserAccountInactive:
		statusCode = http.StatusForbidden
		errorMessage = "User account is inactive"
	case err == service.ErrUserNotVerified:
		statusCode = http.StatusForbidden
		errorMessage = "User account is not verified"
	case err == service.ErrInvalidCredentials:
		statusCode = http.StatusUnauthorized
		errorMessage = "Invalid credentials"
	case err == service.ErrInvalidRole:
		statusCode = http.StatusBadRequest
		errorMessage = "Invalid role"
	case err == service.ErrRoleNotFound:
		statusCode = http.StatusNotFound
		errorMessage = "Role not found"
	case err == service.ErrCannotDeleteSystemRole:
		statusCode = http.StatusForbidden
		errorMessage = "Cannot delete system role"
	case err == service.ErrInsufficientPermissions:
		statusCode = http.StatusForbidden
		errorMessage = "Insufficient permissions"
	default:
		statusCode = http.StatusInternalServerError
		errorMessage = "Internal server error"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   errorMessage,
		"message": message,
	})
}
