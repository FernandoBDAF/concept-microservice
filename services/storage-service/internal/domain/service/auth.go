package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/infrastructure/repository"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// Auth service errors
var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrUserNotFound            = errors.New("user not found")
	ErrUserAlreadyExists       = errors.New("user already exists")
	ErrUserAccountLocked       = errors.New("user account is locked")
	ErrUserAccountInactive     = errors.New("user account is inactive")
	ErrUserNotVerified         = errors.New("user account is not verified")
	ErrInvalidRole             = errors.New("invalid role")
	ErrRoleNotFound            = errors.New("role not found")
	ErrCannotDeleteSystemRole  = errors.New("cannot delete system role")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
)

// AuthService handles business logic for authentication operations
type AuthService struct {
	userRepo  *repository.AuthRepository
	roleRepo  *repository.AuthRepository // Same repo, different operations
	auditRepo *repository.AuthRepository // Same repo, different operations
	log       *zap.Logger

	// Configuration
	passwordCost      int
	maxFailedAttempts int
	lockoutDuration   time.Duration
	sessionTimeout    time.Duration
}

// NewAuthService creates a new auth service
func NewAuthService(authRepo *repository.AuthRepository) *AuthService {
	return &AuthService{
		userRepo:          authRepo,
		roleRepo:          authRepo,
		auditRepo:         authRepo,
		log:               logger.Get().Named("auth_service"),
		passwordCost:      12, // bcrypt cost
		maxFailedAttempts: 5,
		lockoutDuration:   15 * time.Minute,
		sessionTimeout:    24 * time.Hour,
	}
}

// User Management Operations

// CreateUser creates a new user with hashed password
func (s *AuthService) CreateUser(ctx context.Context, req *models.AuthUserRequest) (*models.AuthUser, error) {
	startTime := time.Now()
	s.log.Info("Creating new user",
		logger.String("email", req.Email),
		logger.String("role", req.Role),
	)

	// Validate request
	if err := req.Validate(); err != nil {
		s.log.Error("Invalid user creation request",
			logger.ErrorField(err),
			logger.String("email", req.Email),
		)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		s.log.Error("Failed to check existing user",
			logger.ErrorField(err),
			logger.String("email", req.Email),
		)
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		s.log.Error("User already exists",
			logger.String("email", req.Email),
		)
		return nil, ErrUserAlreadyExists
	}

	// Validate role exists
	if req.Role != "" {
		_, err := s.roleRepo.GetRoleByName(ctx, req.Role)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				s.log.Error("Invalid role",
					logger.String("role", req.Role),
				)
				return nil, ErrInvalidRole
			}
			return nil, fmt.Errorf("failed to validate role: %w", err)
		}
	}

	// Generate salt and hash password
	salt, err := s.generateSalt()
	if err != nil {
		s.log.Error("Failed to generate salt",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	hashedPassword, err := s.hashPassword(req.Password, salt)
	if err != nil {
		s.log.Error("Failed to hash password",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user model
	user := &models.AuthUser{
		ID:             uuid.New().String(),
		Email:          req.Email,
		HashedPassword: hashedPassword,
		Salt:           salt,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Role:           req.Role,
		IsActive:       true,
		IsVerified:     false, // Users need to verify their email
		FailedAttempts: 0,
	}

	// Set IsActive from request if provided
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	// Create user in database
	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		s.log.Error("Failed to create user in database",
			logger.ErrorField(err),
			logger.String("email", req.Email),
		)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create audit log
	auditReq := &models.AuthAuditLogRequest{
		Action:    models.AuditActionUserCreated,
		Resource:  models.ResourceUser,
		IPAddress: "127.0.0.1", // TODO: Get from context
		UserAgent: "storage-service",
		Success:   true,
		Details: map[string]interface{}{
			"user_id":    user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"created_by": "system",
		},
	}

	if err := s.createAuditLog(ctx, auditReq); err != nil {
		s.log.Warn("Failed to create audit log for user creation",
			logger.ErrorField(err),
		)
		// Don't fail the operation for audit log failure
	}

	s.log.Info("Successfully created user",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
		logger.Duration("duration", time.Since(startTime)),
	)

	// Return sanitized user (without sensitive data)
	return user.SanitizeForAPI(), nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*models.AuthUser, error) {
	s.log.Debug("Getting user by ID",
		logger.String("user_id", id),
	)

	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Debug("User not found",
				logger.String("user_id", id),
			)
			return nil, ErrUserNotFound
		}
		s.log.Error("Failed to get user",
			logger.ErrorField(err),
			logger.String("user_id", id),
		)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	s.log.Debug("Successfully retrieved user",
		logger.String("user_id", id),
		logger.String("email", user.Email),
	)

	// Return sanitized user (without sensitive data)
	return user.SanitizeForAPI(), nil
}

// GetUserByEmail retrieves a user by email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*models.AuthUser, error) {
	s.log.Debug("Getting user by email",
		logger.String("email", email),
	)

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Debug("User not found",
				logger.String("email", email),
			)
			return nil, ErrUserNotFound
		}
		s.log.Error("Failed to get user by email",
			logger.ErrorField(err),
			logger.String("email", email),
		)
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	s.log.Debug("Successfully retrieved user by email",
		logger.String("user_id", user.ID),
		logger.String("email", email),
	)

	// Return sanitized user (without sensitive data)
	return user.SanitizeForAPI(), nil
}

// AuthenticateUser authenticates a user with email and password
func (s *AuthService) AuthenticateUser(ctx context.Context, email, password, ipAddress, userAgent string) (*models.AuthUser, error) {
	startTime := time.Now()
	s.log.Info("Authenticating user",
		logger.String("email", email),
		logger.String("ip_address", ipAddress),
	)

	// Get user by email (with sensitive data for verification)
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Record failed login attempt for non-existent user
			_ = s.recordLoginAttempt(ctx, "", ipAddress, userAgent, false, "user not found")
			s.log.Warn("Authentication failed - user not found",
				logger.String("email", email),
			)
			return nil, ErrInvalidCredentials
		}
		s.log.Error("Failed to get user for authentication",
			logger.ErrorField(err),
			logger.String("email", email),
		)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Check if user can login
	if !user.CanLogin() {
		reason := ""
		var returnErr error

		if !user.IsActive {
			reason = "account inactive"
			returnErr = ErrUserAccountInactive
		} else if !user.IsVerified {
			reason = "account not verified"
			returnErr = ErrUserNotVerified
		} else if user.IsLocked() {
			reason = "account locked"
			returnErr = ErrUserAccountLocked
		}

		// Record failed login attempt
		_ = s.recordLoginAttempt(ctx, user.ID, ipAddress, userAgent, false, reason)

		s.log.Warn("Authentication failed - user cannot login",
			logger.String("user_id", user.ID),
			logger.String("email", email),
			logger.String("reason", reason),
		)
		return nil, returnErr
	}

	// Verify password
	if !s.verifyPassword(password, user.HashedPassword, user.Salt) {
		// Increment failed attempts and potentially lock account
		_ = s.handleFailedLogin(ctx, user, ipAddress, userAgent)

		s.log.Warn("Authentication failed - invalid password",
			logger.String("user_id", user.ID),
			logger.String("email", email),
		)
		return nil, ErrInvalidCredentials
	}

	// Successful authentication - record login and reset failed attempts
	_ = s.recordLoginAttempt(ctx, user.ID, ipAddress, userAgent, true, "successful login")

	s.log.Info("Successfully authenticated user",
		logger.String("user_id", user.ID),
		logger.String("email", email),
		logger.Duration("duration", time.Since(startTime)),
	)

	// Return sanitized user (without sensitive data)
	return user.SanitizeForAPI(), nil
}

// UpdateUser updates user information
func (s *AuthService) UpdateUser(ctx context.Context, id string, req *models.AuthUserRequest) (*models.AuthUser, error) {
	s.log.Info("Updating user",
		logger.String("user_id", id),
		logger.String("email", req.Email),
	)

	// Validate request
	if err := req.Validate(); err != nil {
		s.log.Error("Invalid user update request",
			logger.ErrorField(err),
			logger.String("user_id", id),
		)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// Get existing user
	existingUser, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if email is being changed and if it's already in use
	if req.Email != existingUser.Email {
		otherUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if otherUser != nil {
			return nil, ErrUserAlreadyExists
		}
	}

	// Validate role if changed
	if req.Role != existingUser.Role {
		_, err := s.roleRepo.GetRoleByName(ctx, req.Role)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrInvalidRole
			}
			return nil, fmt.Errorf("failed to validate role: %w", err)
		}
	}

	// Update user fields
	existingUser.Email = req.Email
	existingUser.FirstName = req.FirstName
	existingUser.LastName = req.LastName
	existingUser.Role = req.Role

	if req.IsActive != nil {
		existingUser.IsActive = *req.IsActive
	}

	// Update password if provided
	if req.Password != "" {
		salt, err := s.generateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}

		hashedPassword, err := s.hashPassword(req.Password, salt)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}

		existingUser.HashedPassword = hashedPassword
		existingUser.Salt = salt
		existingUser.FailedAttempts = 0
		existingUser.LockedUntil = nil
	}

	// Update in database
	if err := s.userRepo.UpdateUser(ctx, existingUser); err != nil {
		s.log.Error("Failed to update user",
			logger.ErrorField(err),
			logger.String("user_id", id),
		)
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Create audit log
	auditReq := &models.AuthAuditLogRequest{
		UserID:    &id,
		Action:    models.AuditActionUserUpdated,
		Resource:  models.ResourceUser,
		IPAddress: "127.0.0.1", // TODO: Get from context
		UserAgent: "storage-service",
		Success:   true,
		Details: map[string]interface{}{
			"updated_fields": []string{"email", "first_name", "last_name", "role"},
			"new_email":      req.Email,
			"new_role":       req.Role,
		},
	}

	if err := s.createAuditLog(ctx, auditReq); err != nil {
		s.log.Warn("Failed to create audit log for user update",
			logger.ErrorField(err),
		)
	}

	s.log.Info("Successfully updated user",
		logger.String("user_id", id),
		logger.String("email", req.Email),
	)

	// Return sanitized user
	return existingUser.SanitizeForAPI(), nil
}

// DeleteUser deletes a user
func (s *AuthService) DeleteUser(ctx context.Context, id string) error {
	s.log.Info("Deleting user",
		logger.String("user_id", id),
	)

	// Check if user exists
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Delete user
	if err := s.userRepo.DeleteUser(ctx, id); err != nil {
		s.log.Error("Failed to delete user",
			logger.ErrorField(err),
			logger.String("user_id", id),
		)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Create audit log
	auditReq := &models.AuthAuditLogRequest{
		UserID:    &id,
		Action:    models.AuditActionUserDeleted,
		Resource:  models.ResourceUser,
		IPAddress: "127.0.0.1", // TODO: Get from context
		UserAgent: "storage-service",
		Success:   true,
		Details: map[string]interface{}{
			"deleted_user_email": user.Email,
			"deleted_user_role":  user.Role,
		},
	}

	if err := s.createAuditLog(ctx, auditReq); err != nil {
		s.log.Warn("Failed to create audit log for user deletion",
			logger.ErrorField(err),
		)
	}

	s.log.Info("Successfully deleted user",
		logger.String("user_id", id),
		logger.String("email", user.Email),
	)
	return nil
}

// ListUsers retrieves users with pagination
func (s *AuthService) ListUsers(ctx context.Context, page, pageSize int) ([]*models.AuthUser, error) {
	s.log.Debug("Listing users",
		logger.Int("page", page),
		logger.Int("page_size", pageSize),
	)

	users, err := s.userRepo.ListUsers(ctx, page, pageSize)
	if err != nil {
		s.log.Error("Failed to list users",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Sanitize all users
	var sanitizedUsers []*models.AuthUser
	for _, user := range users {
		sanitizedUsers = append(sanitizedUsers, user.SanitizeForAPI())
	}

	s.log.Debug("Successfully listed users",
		logger.Int("count", len(sanitizedUsers)),
	)
	return sanitizedUsers, nil
}

// Audit Log Operations

// CreateAuditLog creates an audit log entry
func (s *AuthService) CreateAuditLog(ctx context.Context, req *models.AuthAuditLogRequest) error {
	return s.createAuditLog(ctx, req)
}

// GetAuditLogs retrieves audit logs with filtering
func (s *AuthService) GetAuditLogs(ctx context.Context, userID *string, action string, success *bool, page, pageSize int) ([]*models.AuthAuditLog, error) {
	s.log.Debug("Getting audit logs",
		logger.Int("page", page),
		logger.Int("page_size", pageSize),
	)

	logs, err := s.auditRepo.GetAuditLogs(ctx, userID, action, success, page, pageSize)
	if err != nil {
		s.log.Error("Failed to get audit logs",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	s.log.Debug("Successfully retrieved audit logs",
		logger.Int("count", len(logs)),
	)
	return logs, nil
}

// Role Management Operations

// CreateRole creates a new role
func (s *AuthService) CreateRole(ctx context.Context, req *models.AuthRoleRequest) (*models.AuthRole, error) {
	s.log.Info("Creating role",
		logger.String("role_name", req.Name),
	)

	// Validate request
	if err := req.Validate(); err != nil {
		s.log.Error("Invalid role creation request",
			logger.ErrorField(err),
			logger.String("role_name", req.Name),
		)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// Create role model
	role := &models.AuthRole{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    false, // User-created roles are not system roles
	}

	// Create role in database
	if err := s.roleRepo.CreateRole(ctx, role); err != nil {
		s.log.Error("Failed to create role",
			logger.ErrorField(err),
			logger.String("role_name", req.Name),
		)
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// Create audit log
	auditReq := &models.AuthAuditLogRequest{
		Action:    models.AuditActionRoleCreated,
		Resource:  models.ResourceRole,
		IPAddress: "127.0.0.1", // TODO: Get from context
		UserAgent: "storage-service",
		Success:   true,
		Details: map[string]interface{}{
			"role_id":     role.ID,
			"role_name":   role.Name,
			"permissions": role.Permissions,
		},
	}

	if err := s.createAuditLog(ctx, auditReq); err != nil {
		s.log.Warn("Failed to create audit log for role creation",
			logger.ErrorField(err),
		)
	}

	s.log.Info("Successfully created role",
		logger.String("role_id", role.ID),
		logger.String("role_name", req.Name),
	)
	return role, nil
}

// GetRoleByID retrieves a role by ID
func (s *AuthService) GetRoleByID(ctx context.Context, id string) (*models.AuthRole, error) {
	s.log.Debug("Getting role by ID",
		logger.String("role_id", id),
	)

	role, err := s.roleRepo.GetRoleByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	s.log.Debug("Successfully retrieved role",
		logger.String("role_id", id),
		logger.String("role_name", role.Name),
	)
	return role, nil
}

// ListRoles retrieves all roles
func (s *AuthService) ListRoles(ctx context.Context) ([]*models.AuthRole, error) {
	s.log.Debug("Listing all roles")

	roles, err := s.roleRepo.ListRoles(ctx)
	if err != nil {
		s.log.Error("Failed to list roles",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	s.log.Debug("Successfully listed roles",
		logger.Int("count", len(roles)),
	)
	return roles, nil
}

// Helper Methods

// generateSalt generates a random salt for password hashing
func (s *AuthService) generateSalt() (string, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return hex.EncodeToString(salt), nil
}

// hashPassword hashes a password with salt using bcrypt
func (s *AuthService) hashPassword(password, salt string) (string, error) {
	// Combine password and salt
	saltedPassword := password + salt

	// Hash with bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(saltedPassword), s.passwordCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// verifyPassword verifies a password against its hash and salt
func (s *AuthService) verifyPassword(password, hashedPassword, salt string) bool {
	// Combine password and salt
	saltedPassword := password + salt

	// Compare with bcrypt
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(saltedPassword))
	return err == nil
}

// handleFailedLogin handles a failed login attempt
func (s *AuthService) handleFailedLogin(ctx context.Context, user *models.AuthUser, ipAddress, userAgent string) error {
	user.FailedAttempts++

	// Lock account if max attempts reached
	if user.FailedAttempts >= s.maxFailedAttempts {
		lockUntil := time.Now().Add(s.lockoutDuration)
		user.LockedUntil = &lockUntil

		// Create account locked audit log
		auditReq := &models.AuthAuditLogRequest{
			UserID:    &user.ID,
			Action:    models.AuditActionAccountLocked,
			Resource:  models.ResourceUser,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Success:   true,
			Details: map[string]interface{}{
				"failed_attempts": user.FailedAttempts,
				"locked_until":    lockUntil.Format(time.RFC3339),
			},
		}
		_ = s.createAuditLog(ctx, auditReq)
	}

	// Update user in database
	_ = s.userRepo.UpdateUser(ctx, user)

	// Record failed login attempt
	return s.recordLoginAttempt(ctx, user.ID, ipAddress, userAgent, false, "invalid password")
}

// recordLoginAttempt records a login attempt in audit logs
func (s *AuthService) recordLoginAttempt(ctx context.Context, userID, ipAddress, userAgent string, success bool, reason string) error {
	return s.auditRepo.RecordLoginAttempt(ctx, userID, ipAddress, userAgent, success, reason)
}

// createAuditLog creates an audit log entry
func (s *AuthService) createAuditLog(ctx context.Context, req *models.AuthAuditLogRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid audit log request: %w", err)
	}

	auditLog, err := req.ToAuditLog()
	if err != nil {
		return fmt.Errorf("failed to convert audit log request: %w", err)
	}

	return s.auditRepo.CreateAuditLog(ctx, auditLog)
}
