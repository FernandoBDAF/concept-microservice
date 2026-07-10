package models

import (
	"errors"
	"strings"
	"time"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Role      string `json:"role,omitempty"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email     *string `json:"email,omitempty" validate:"omitempty,email"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Role      *string `json:"role,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
}

// UserResponse represents a response containing user data
type UserResponse struct {
	User  *User  `json:"user,omitempty"`
	Error string `json:"error,omitempty"`
}

// Validate validates the create user request
func (r *CreateUserRequest) Validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(r.Email, "@") || !strings.Contains(r.Email, ".") {
		return errors.New("invalid email format")
	}
	if strings.TrimSpace(r.Password) == "" {
		return errors.New("password is required")
	}
	if len(r.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if strings.TrimSpace(r.FirstName) == "" {
		return errors.New("first name is required")
	}
	if strings.TrimSpace(r.LastName) == "" {
		return errors.New("last name is required")
	}
	return nil
}

// Validate validates the update user request
func (r *UpdateUserRequest) Validate() error {
	if r.Email != nil {
		if !strings.Contains(*r.Email, "@") || !strings.Contains(*r.Email, ".") {
			return errors.New("invalid email format")
		}
	}
	return nil
}
