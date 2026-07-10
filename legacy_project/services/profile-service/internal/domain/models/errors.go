package models

import "errors"

var (
	// ErrProfileNotFound is returned when a profile cannot be found
	ErrProfileNotFound = errors.New("profile not found")

	// ErrInvalidProfile is returned when a profile is invalid
	ErrInvalidProfile = errors.New("invalid profile")

	// ErrDuplicateProfile is returned when attempting to create a duplicate profile
	ErrDuplicateProfile = errors.New("profile already exists")

	// User-related errors
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidUser  = errors.New("invalid user data")
	ErrUserInactive = errors.New("user is inactive")
	ErrUnauthorized = errors.New("unauthorized access")
)
