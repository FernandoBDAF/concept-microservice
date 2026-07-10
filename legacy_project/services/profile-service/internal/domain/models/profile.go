package models

import (
	"errors"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

// Profile represents a user profile
type Profile struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	ImageURLs []string  `json:"image_urls,omitempty"`
	Address   *Address  `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	GetFrom   string    `json:"get_from,omitempty"`
}

// Address represents a user's address
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
	ZipCode string `json:"zip_code"`
}

// ProfileRequest represents a request to create or update a profile
type ProfileRequest struct {
	FirstName string   `json:"first_name" binding:"required"`
	LastName  string   `json:"last_name" binding:"required"`
	Email     string   `json:"email" binding:"required,email"`
	Phone     string   `json:"phone,omitempty"`
	Bio       string   `json:"bio,omitempty"`
	ImageURLs []string `json:"image_urls,omitempty"`
	Address   *Address `json:"address,omitempty"`
}

// ProfileResponse represents a response containing profile data
type ProfileResponse struct {
	Profile *Profile `json:"profile,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// Validate validates the profile request
func (r *ProfileRequest) Validate() error {
	if strings.TrimSpace(r.FirstName) == "" {
		return errors.New("first name is required")
	}

	if strings.TrimSpace(r.LastName) == "" {
		return errors.New("last name is required")
	}

	if strings.TrimSpace(r.Email) == "" {
		return errors.New("email is required")
	}

	// Basic email format validation
	if !strings.Contains(r.Email, "@") || !strings.Contains(r.Email, ".") {
		return errors.New("invalid email format")
	}

	// Validate address if provided
	if r.Address != nil {
		if strings.TrimSpace(r.Address.Street) == "" {
			return errors.New("street is required when address is provided")
		}
		if strings.TrimSpace(r.Address.City) == "" {
			return errors.New("city is required when address is provided")
		}
		if strings.TrimSpace(r.Address.State) == "" {
			return errors.New("state is required when address is provided")
		}
		if strings.TrimSpace(r.Address.Country) == "" {
			return errors.New("country is required when address is provided")
		}
		if strings.TrimSpace(r.Address.ZipCode) == "" {
			return errors.New("zip code is required when address is provided")
		}
	}

	return nil
}
