package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuthUser represents a user in the authentication system
type AuthUser struct {
	ID             string     `json:"id" db:"id"`
	Email          string     `json:"email" db:"email"`
	HashedPassword string     `json:"-" db:"hashed_password"` // Never expose in JSON
	Salt           string     `json:"-" db:"salt"`            // Never expose in JSON
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	Role           string     `json:"role" db:"role"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	IsVerified     bool       `json:"is_verified" db:"is_verified"`
	LastLoginAt    *time.Time `json:"last_login_at" db:"last_login_at"`
	FailedAttempts int        `json:"failed_attempts" db:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until" db:"locked_until"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// AuthUserRequest represents a request to create or update a user
type AuthUserRequest struct {
	Email     string `json:"email" validate:"required,email,max=255"`
	Password  string `json:"password,omitempty" validate:"omitempty,min=8,max=128"`
	FirstName string `json:"first_name" validate:"required,min=1,max=100"`
	LastName  string `json:"last_name" validate:"required,min=1,max=100"`
	Role      string `json:"role" validate:"required,oneof=admin user moderator"`
	IsActive  *bool  `json:"is_active,omitempty"`
}

// Validate validates the auth user request
func (r *AuthUserRequest) Validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return fmt.Errorf("email is required")
	}

	// Validate email format
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(r.Email) {
		return fmt.Errorf("invalid email format")
	}

	if len(r.Email) > 255 {
		return fmt.Errorf("email must be at most 255 characters")
	}

	if strings.TrimSpace(r.FirstName) == "" {
		return fmt.Errorf("first name is required")
	}

	if len(r.FirstName) > 100 {
		return fmt.Errorf("first name must be at most 100 characters")
	}

	if strings.TrimSpace(r.LastName) == "" {
		return fmt.Errorf("last name is required")
	}

	if len(r.LastName) > 100 {
		return fmt.Errorf("last name must be at most 100 characters")
	}

	// Validate role
	validRoles := map[string]bool{
		"admin":     true,
		"user":      true,
		"moderator": true,
	}
	if !validRoles[r.Role] {
		return fmt.Errorf("role must be one of: admin, user, moderator")
	}

	// Validate password if provided (for create or password update)
	if r.Password != "" {
		if len(r.Password) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}
		if len(r.Password) > 128 {
			return fmt.Errorf("password must be at most 128 characters")
		}
	}

	return nil
}

// IsLocked checks if the user account is currently locked
func (u *AuthUser) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// CanLogin checks if the user can log in (active, verified, not locked)
func (u *AuthUser) CanLogin() bool {
	return u.IsActive && u.IsVerified && !u.IsLocked()
}

// SanitizeForAPI removes sensitive data before sending to API response
func (u *AuthUser) SanitizeForAPI() *AuthUser {
	// Create a copy to avoid modifying the original
	sanitized := *u
	sanitized.HashedPassword = ""
	sanitized.Salt = ""
	return &sanitized
}

// AuthAuditLog represents a security audit log entry
type AuthAuditLog struct {
	ID        string    `json:"id" db:"id"`
	UserID    *string   `json:"user_id" db:"user_id"` // Nullable for system events
	Action    string    `json:"action" db:"action"`
	Resource  string    `json:"resource" db:"resource"` // What was accessed/modified
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	Success   bool      `json:"success" db:"success"`
	Details   string    `json:"details" db:"details"` // JSON string with additional context
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// AuthAuditLogRequest represents a request to create an audit log entry
type AuthAuditLogRequest struct {
	UserID    *string                `json:"user_id,omitempty"`
	Action    string                 `json:"action" validate:"required,max=100"`
	Resource  string                 `json:"resource" validate:"required,max=255"`
	IPAddress string                 `json:"ip_address" validate:"required,ip"`
	UserAgent string                 `json:"user_agent" validate:"max=1000"`
	Success   bool                   `json:"success"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Validate validates the audit log request
func (r *AuthAuditLogRequest) Validate() error {
	if strings.TrimSpace(r.Action) == "" {
		return fmt.Errorf("action is required")
	}

	if len(r.Action) > 100 {
		return fmt.Errorf("action must be at most 100 characters")
	}

	if strings.TrimSpace(r.Resource) == "" {
		return fmt.Errorf("resource is required")
	}

	if len(r.Resource) > 255 {
		return fmt.Errorf("resource must be at most 255 characters")
	}

	if strings.TrimSpace(r.IPAddress) == "" {
		return fmt.Errorf("ip_address is required")
	}

	if len(r.UserAgent) > 1000 {
		return fmt.Errorf("user_agent must be at most 1000 characters")
	}

	return nil
}

// ToAuditLog converts the request to an AuthAuditLog model
func (r *AuthAuditLogRequest) ToAuditLog() (*AuthAuditLog, error) {
	var detailsJSON string
	if r.Details != nil {
		detailsBytes, err := json.Marshal(r.Details)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal details: %w", err)
		}
		detailsJSON = string(detailsBytes)
	}

	return &AuthAuditLog{
		ID:        uuid.New().String(),
		UserID:    r.UserID,
		Action:    r.Action,
		Resource:  r.Resource,
		IPAddress: r.IPAddress,
		UserAgent: r.UserAgent,
		Success:   r.Success,
		Details:   detailsJSON,
		CreatedAt: time.Now(),
	}, nil
}

// AuthRole represents a role in the role-based access control system
type AuthRole struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Permissions []string  `json:"permissions" db:"-"`       // Loaded separately
	IsSystem    bool      `json:"is_system" db:"is_system"` // System roles cannot be deleted
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// AuthRoleRequest represents a request to create or update a role
type AuthRoleRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=100"`
	Description string   `json:"description" validate:"max=500"`
	Permissions []string `json:"permissions" validate:"required,min=1"`
}

// Validate validates the role request
func (r *AuthRoleRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if len(r.Name) > 100 {
		return fmt.Errorf("name must be at most 100 characters")
	}

	if len(r.Description) > 500 {
		return fmt.Errorf("description must be at most 500 characters")
	}

	if len(r.Permissions) == 0 {
		return fmt.Errorf("at least one permission is required")
	}

	// Validate permissions format
	validPermissions := map[string]bool{
		"users:read":      true,
		"users:write":     true,
		"users:delete":    true,
		"profiles:read":   true,
		"profiles:write":  true,
		"profiles:delete": true,
		"audit:read":      true,
		"roles:read":      true,
		"roles:write":     true,
		"roles:delete":    true,
		"system:admin":    true,
	}

	for _, permission := range r.Permissions {
		if !validPermissions[permission] {
			return fmt.Errorf("invalid permission: %s", permission)
		}
	}

	return nil
}

// AuthPermission represents a single permission entry for a role
type AuthPermission struct {
	ID         string    `json:"id" db:"id"`
	RoleID     string    `json:"role_id" db:"role_id"`
	Permission string    `json:"permission" db:"permission"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// LoginAttemptRequest represents a login attempt for audit logging
type LoginAttemptRequest struct {
	UserID    string `json:"user_id" validate:"required"`
	IPAddress string `json:"ip_address" validate:"required,ip"`
	UserAgent string `json:"user_agent" validate:"max=1000"`
	Success   bool   `json:"success"`
	Reason    string `json:"reason,omitempty" validate:"max=255"` // Failure reason if applicable
}

// Validate validates the login attempt request
func (r *LoginAttemptRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}

	if strings.TrimSpace(r.IPAddress) == "" {
		return fmt.Errorf("ip_address is required")
	}

	if len(r.UserAgent) > 1000 {
		return fmt.Errorf("user_agent must be at most 1000 characters")
	}

	if len(r.Reason) > 255 {
		return fmt.Errorf("reason must be at most 255 characters")
	}

	return nil
}

// Constants for audit log actions
const (
	AuditActionLogin           = "login"
	AuditActionLoginFailed     = "login_failed"
	AuditActionLogout          = "logout"
	AuditActionPasswordChange  = "password_change"
	AuditActionAccountLocked   = "account_locked"
	AuditActionAccountUnlocked = "account_unlocked"
	AuditActionUserCreated     = "user_created"
	AuditActionUserUpdated     = "user_updated"
	AuditActionUserDeleted     = "user_deleted"
	AuditActionRoleCreated     = "role_created"
	AuditActionRoleUpdated     = "role_updated"
	AuditActionRoleDeleted     = "role_deleted"
)

// Constants for resources
const (
	ResourceUser    = "user"
	ResourceRole    = "role"
	ResourceProfile = "profile"
	ResourceSystem  = "system"
)

// Default system roles
var DefaultRoles = []AuthRole{
	{
		ID:          "admin",
		Name:        "Administrator",
		Description: "Full system access with all permissions",
		Permissions: []string{
			"users:read", "users:write", "users:delete",
			"profiles:read", "profiles:write", "profiles:delete",
			"audit:read", "roles:read", "roles:write", "roles:delete",
			"system:admin",
		},
		IsSystem:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		ID:          "user",
		Name:        "User",
		Description: "Standard user with basic profile access",
		Permissions: []string{
			"profiles:read", "profiles:write",
		},
		IsSystem:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		ID:          "moderator",
		Name:        "Moderator",
		Description: "Moderation access with user and profile management",
		Permissions: []string{
			"users:read", "users:write",
			"profiles:read", "profiles:write", "profiles:delete",
			"audit:read",
		},
		IsSystem:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
}
