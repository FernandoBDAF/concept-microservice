package repository

import "errors"

// Common repository errors
var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
)
