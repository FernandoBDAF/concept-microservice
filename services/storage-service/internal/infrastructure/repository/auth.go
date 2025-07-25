package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// AuthRepository handles database operations for authentication data
type AuthRepository struct {
	db  *sqlx.DB
	log *zap.Logger
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *sqlx.DB) *AuthRepository {
	return &AuthRepository{
		db:  db,
		log: logger.Get().Named("auth_repository"),
	}
}

// User Operations

// CreateUser creates a new user in the database
func (r *AuthRepository) CreateUser(ctx context.Context, user *models.AuthUser) error {
	r.log.Info("Creating new auth user",
		logger.String("email", user.Email),
		logger.String("role", user.Role),
	)

	query := `
		INSERT INTO auth_users (id, email, hashed_password, salt, first_name, last_name, role, is_active, is_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowxContext(ctx, query,
		user.ID, user.Email, user.HashedPassword, user.Salt,
		user.FirstName, user.LastName, user.Role, user.IsActive, user.IsVerified,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			r.log.Error("User email already exists",
				logger.String("email", user.Email),
				logger.ErrorField(err),
			)
			return fmt.Errorf("user with email %s already exists", user.Email)
		}
		r.log.Error("Failed to create user",
			logger.String("email", user.Email),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to create user: %w", err)
	}

	r.log.Info("Successfully created auth user",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)
	return nil
}

// GetUserByID retrieves a user by ID
func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*models.AuthUser, error) {
	r.log.Debug("Getting user by ID",
		logger.String("user_id", id),
	)

	query := `
		SELECT id, email, hashed_password, salt, first_name, last_name, role, 
		       is_active, is_verified, last_login_at, failed_attempts, locked_until,
		       created_at, updated_at
		FROM auth_users 
		WHERE id = $1`

	var user models.AuthUser
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("User not found",
				logger.String("user_id", id),
			)
			return nil, ErrNotFound
		}
		r.log.Error("Failed to get user by ID",
			logger.String("user_id", id),
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	r.log.Debug("Successfully retrieved user",
		logger.String("user_id", id),
		logger.String("email", user.Email),
	)
	return &user, nil
}

// GetUserByEmail retrieves a user by email address
func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*models.AuthUser, error) {
	r.log.Debug("Getting user by email",
		logger.String("email", email),
	)

	query := `
		SELECT id, email, hashed_password, salt, first_name, last_name, role, 
		       is_active, is_verified, last_login_at, failed_attempts, locked_until,
		       created_at, updated_at
		FROM auth_users 
		WHERE email = $1`

	var user models.AuthUser
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("User not found",
				logger.String("email", email),
			)
			return nil, ErrNotFound
		}
		r.log.Error("Failed to get user by email",
			logger.String("email", email),
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	r.log.Debug("Successfully retrieved user by email",
		logger.String("user_id", user.ID),
		logger.String("email", email),
	)
	return &user, nil
}

// UpdateUser updates a user in the database
func (r *AuthRepository) UpdateUser(ctx context.Context, user *models.AuthUser) error {
	r.log.Info("Updating auth user",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)

	query := `
		UPDATE auth_users 
		SET email = $1, first_name = $2, last_name = $3, role = $4, 
		    is_active = $5, is_verified = $6, last_login_at = $7, 
		    failed_attempts = $8, locked_until = $9
		WHERE id = $10
		RETURNING updated_at`

	err := r.db.QueryRowxContext(ctx, query,
		user.Email, user.FirstName, user.LastName, user.Role,
		user.IsActive, user.IsVerified, user.LastLoginAt,
		user.FailedAttempts, user.LockedUntil, user.ID,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Error("User not found for update",
				logger.String("user_id", user.ID),
			)
			return ErrNotFound
		}
		r.log.Error("Failed to update user",
			logger.String("user_id", user.ID),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to update user: %w", err)
	}

	r.log.Info("Successfully updated auth user",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)
	return nil
}

// UpdateUserPassword updates a user's password and salt
func (r *AuthRepository) UpdateUserPassword(ctx context.Context, userID, hashedPassword, salt string) error {
	r.log.Info("Updating user password",
		logger.String("user_id", userID),
	)

	query := `
		UPDATE auth_users 
		SET hashed_password = $1, salt = $2, failed_attempts = 0, locked_until = NULL
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, hashedPassword, salt, userID)
	if err != nil {
		r.log.Error("Failed to update user password",
			logger.String("user_id", userID),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.log.Error("User not found for password update",
			logger.String("user_id", userID),
		)
		return ErrNotFound
	}

	r.log.Info("Successfully updated user password",
		logger.String("user_id", userID),
	)
	return nil
}

// DeleteUser deletes a user from the database
func (r *AuthRepository) DeleteUser(ctx context.Context, id string) error {
	r.log.Info("Deleting auth user",
		logger.String("user_id", id),
	)

	query := `DELETE FROM auth_users WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("Failed to delete user",
			logger.String("user_id", id),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.log.Error("User not found for deletion",
			logger.String("user_id", id),
		)
		return ErrNotFound
	}

	r.log.Info("Successfully deleted auth user",
		logger.String("user_id", id),
	)
	return nil
}

// ListUsers retrieves a list of users with pagination
func (r *AuthRepository) ListUsers(ctx context.Context, page, pageSize int) ([]*models.AuthUser, error) {
	r.log.Debug("Listing auth users",
		logger.Int("page", page),
		logger.Int("page_size", pageSize),
	)

	offset := (page - 1) * pageSize
	query := `
		SELECT id, email, hashed_password, salt, first_name, last_name, role, 
		       is_active, is_verified, last_login_at, failed_attempts, locked_until,
		       created_at, updated_at
		FROM auth_users 
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	var users []*models.AuthUser
	err := r.db.SelectContext(ctx, &users, query, pageSize, offset)
	if err != nil {
		r.log.Error("Failed to list users",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	r.log.Debug("Successfully listed users",
		logger.Int("count", len(users)),
	)
	return users, nil
}

// Audit Log Operations

// CreateAuditLog creates a new audit log entry
func (r *AuthRepository) CreateAuditLog(ctx context.Context, log *models.AuthAuditLog) error {
	r.log.Debug("Creating audit log entry",
		logger.String("action", log.Action),
		logger.String("resource", log.Resource),
	)

	query := `
		INSERT INTO auth_audit_logs (id, user_id, action, resource, ip_address, user_agent, success, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.UserID, log.Action, log.Resource,
		log.IPAddress, log.UserAgent, log.Success, log.Details, log.CreatedAt,
	)

	if err != nil {
		r.log.Error("Failed to create audit log",
			logger.String("action", log.Action),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	r.log.Debug("Successfully created audit log entry",
		logger.String("log_id", log.ID),
		logger.String("action", log.Action),
	)
	return nil
}

// GetAuditLogs retrieves audit logs with filtering and pagination
func (r *AuthRepository) GetAuditLogs(ctx context.Context, userID *string, action string, success *bool, page, pageSize int) ([]*models.AuthAuditLog, error) {
	r.log.Debug("Getting audit logs",
		logger.Int("page", page),
		logger.Int("page_size", pageSize),
	)

	var conditions []string
	var args []interface{}
	argIndex := 1

	baseQuery := `
		SELECT id, user_id, action, resource, ip_address, user_agent, success, details, created_at
		FROM auth_audit_logs`

	if userID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, *userID)
		argIndex++
	}

	if action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIndex))
		args = append(args, action)
		argIndex++
	}

	if success != nil {
		conditions = append(conditions, fmt.Sprintf("success = $%d", argIndex))
		args = append(args, *success)
		argIndex++
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, (page-1)*pageSize)

	var logs []*models.AuthAuditLog
	err := r.db.SelectContext(ctx, &logs, baseQuery, args...)
	if err != nil {
		r.log.Error("Failed to get audit logs",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	r.log.Debug("Successfully retrieved audit logs",
		logger.Int("count", len(logs)),
	)
	return logs, nil
}

// Role Operations

// CreateRole creates a new role
func (r *AuthRepository) CreateRole(ctx context.Context, role *models.AuthRole) error {
	r.log.Info("Creating auth role",
		logger.String("role_name", role.Name),
	)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create role
	query := `
		INSERT INTO auth_roles (id, name, description, is_system)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`

	err = tx.QueryRowxContext(ctx, query,
		role.ID, role.Name, role.Description, role.IsSystem,
	).Scan(&role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return fmt.Errorf("role with name %s already exists", role.Name)
		}
		return fmt.Errorf("failed to create role: %w", err)
	}

	// Create permissions
	if len(role.Permissions) > 0 {
		for _, permission := range role.Permissions {
			permQuery := `INSERT INTO auth_permissions (role_id, permission) VALUES ($1, $2)`
			_, err = tx.ExecContext(ctx, permQuery, role.ID, permission)
			if err != nil {
				return fmt.Errorf("failed to create permission %s: %w", permission, err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.log.Info("Successfully created auth role",
		logger.String("role_id", role.ID),
		logger.String("role_name", role.Name),
	)
	return nil
}

// GetRoleByID retrieves a role by ID with its permissions
func (r *AuthRepository) GetRoleByID(ctx context.Context, id string) (*models.AuthRole, error) {
	r.log.Debug("Getting role by ID",
		logger.String("role_id", id),
	)

	// Get role
	roleQuery := `
		SELECT id, name, description, is_system, created_at, updated_at
		FROM auth_roles 
		WHERE id = $1`

	var role models.AuthRole
	err := r.db.GetContext(ctx, &role, roleQuery, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Get permissions
	permQuery := `SELECT permission FROM auth_permissions WHERE role_id = $1`
	var permissions []string
	err = r.db.SelectContext(ctx, &permissions, permQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	role.Permissions = permissions

	r.log.Debug("Successfully retrieved role",
		logger.String("role_id", id),
		logger.String("role_name", role.Name),
	)
	return &role, nil
}

// GetRoleByName retrieves a role by name with its permissions
func (r *AuthRepository) GetRoleByName(ctx context.Context, name string) (*models.AuthRole, error) {
	r.log.Debug("Getting role by name",
		logger.String("role_name", name),
	)

	// Get role
	roleQuery := `
		SELECT id, name, description, is_system, created_at, updated_at
		FROM auth_roles 
		WHERE name = $1`

	var role models.AuthRole
	err := r.db.GetContext(ctx, &role, roleQuery, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}

	// Get permissions
	permQuery := `SELECT permission FROM auth_permissions WHERE role_id = $1`
	var permissions []string
	err = r.db.SelectContext(ctx, &permissions, permQuery, role.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	role.Permissions = permissions

	r.log.Debug("Successfully retrieved role by name",
		logger.String("role_id", role.ID),
		logger.String("role_name", name),
	)
	return &role, nil
}

// UpdateRole updates a role and its permissions
func (r *AuthRepository) UpdateRole(ctx context.Context, role *models.AuthRole) error {
	r.log.Info("Updating auth role",
		logger.String("role_id", role.ID),
		logger.String("role_name", role.Name),
	)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update role
	query := `
		UPDATE auth_roles 
		SET name = $1, description = $2
		WHERE id = $3 AND is_system = false
		RETURNING updated_at`

	err = tx.QueryRowxContext(ctx, query,
		role.Name, role.Description, role.ID,
	).Scan(&role.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("role not found or is system role")
		}
		return fmt.Errorf("failed to update role: %w", err)
	}

	// Delete existing permissions
	_, err = tx.ExecContext(ctx, `DELETE FROM auth_permissions WHERE role_id = $1`, role.ID)
	if err != nil {
		return fmt.Errorf("failed to delete existing permissions: %w", err)
	}

	// Create new permissions
	if len(role.Permissions) > 0 {
		for _, permission := range role.Permissions {
			permQuery := `INSERT INTO auth_permissions (role_id, permission) VALUES ($1, $2)`
			_, err = tx.ExecContext(ctx, permQuery, role.ID, permission)
			if err != nil {
				return fmt.Errorf("failed to create permission %s: %w", permission, err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.log.Info("Successfully updated auth role",
		logger.String("role_id", role.ID),
		logger.String("role_name", role.Name),
	)
	return nil
}

// DeleteRole deletes a role (only non-system roles)
func (r *AuthRepository) DeleteRole(ctx context.Context, id string) error {
	r.log.Info("Deleting auth role",
		logger.String("role_id", id),
	)

	query := `DELETE FROM auth_roles WHERE id = $1 AND is_system = false`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("Failed to delete role",
			logger.String("role_id", id),
			logger.ErrorField(err),
		)
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.log.Error("Role not found or is system role",
			logger.String("role_id", id),
		)
		return fmt.Errorf("role not found or is system role")
	}

	r.log.Info("Successfully deleted auth role",
		logger.String("role_id", id),
	)
	return nil
}

// ListRoles retrieves all roles with their permissions
func (r *AuthRepository) ListRoles(ctx context.Context) ([]*models.AuthRole, error) {
	r.log.Debug("Listing all auth roles")

	// Get all roles
	roleQuery := `
		SELECT id, name, description, is_system, created_at, updated_at
		FROM auth_roles 
		ORDER BY name`

	var roles []*models.AuthRole
	err := r.db.SelectContext(ctx, &roles, roleQuery)
	if err != nil {
		r.log.Error("Failed to list roles",
			logger.ErrorField(err),
		)
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	// Get permissions for each role
	for _, role := range roles {
		permQuery := `SELECT permission FROM auth_permissions WHERE role_id = $1`
		var permissions []string
		err = r.db.SelectContext(ctx, &permissions, permQuery, role.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for role %s: %w", role.ID, err)
		}
		role.Permissions = permissions
	}

	r.log.Debug("Successfully listed auth roles",
		logger.Int("count", len(roles)),
	)
	return roles, nil
}

// RecordLoginAttempt records a login attempt and updates user statistics
func (r *AuthRepository) RecordLoginAttempt(ctx context.Context, userID, ipAddress, userAgent string, success bool, reason string) error {
	r.log.Debug("Recording login attempt",
		logger.String("user_id", userID),
		logger.Bool("success", success),
	)

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create audit log
	auditLog := &models.AuthAuditLog{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		UserID:    &userID,
		Action:    models.AuditActionLogin,
		Resource:  models.ResourceUser,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		Details:   fmt.Sprintf(`{"reason": "%s"}`, reason),
		CreatedAt: time.Now(),
	}

	if !success {
		auditLog.Action = models.AuditActionLoginFailed
	}

	auditQuery := `
		INSERT INTO auth_audit_logs (id, user_id, action, resource, ip_address, user_agent, success, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.ExecContext(ctx, auditQuery,
		auditLog.ID, auditLog.UserID, auditLog.Action, auditLog.Resource,
		auditLog.IPAddress, auditLog.UserAgent, auditLog.Success, auditLog.Details, auditLog.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Update user login statistics
	if success {
		// Successful login - reset failed attempts and update last login
		userQuery := `
			UPDATE auth_users 
			SET last_login_at = $1, failed_attempts = 0, locked_until = NULL
			WHERE id = $2`
		_, err = tx.ExecContext(ctx, userQuery, time.Now(), userID)
	} else {
		// Failed login - increment failed attempts
		userQuery := `
			UPDATE auth_users 
			SET failed_attempts = failed_attempts + 1
			WHERE id = $1`
		_, err = tx.ExecContext(ctx, userQuery, userID)
	}

	if err != nil {
		return fmt.Errorf("failed to update user login statistics: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.log.Debug("Successfully recorded login attempt",
		logger.String("user_id", userID),
		logger.Bool("success", success),
	)
	return nil
}
