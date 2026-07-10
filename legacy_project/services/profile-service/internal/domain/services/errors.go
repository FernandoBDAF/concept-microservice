package services

import "errors"

var (
	// ErrProfileNotFound is returned when a profile is not found
	ErrProfileNotFound = errors.New("profile not found")

	// ErrInvalidProfile is returned when a profile is invalid
	ErrInvalidProfile = errors.New("invalid profile")

	// ErrDuplicateProfile is returned when a profile already exists
	ErrDuplicateProfile = errors.New("profile already exists")
)
