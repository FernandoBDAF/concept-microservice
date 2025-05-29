package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Profile represents a user profile in the system
type Profile struct {
	ID        uuid.UUID `json:"id" db:"id"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	Email     string    `json:"email" db:"email"`
	Phone     string    `json:"phone,omitempty" db:"phone"`
	Addresses []Address `json:"addresses,omitempty" db:"-"`
	Contacts  []Contact `json:"contacts,omitempty" db:"-"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Address represents a physical address associated with a profile
type Address struct {
	ID         uuid.UUID `json:"id" db:"id"`
	ProfileID  uuid.UUID `json:"profile_id" db:"profile_id"`
	Street     string    `json:"street" db:"street"`
	City       string    `json:"city" db:"city"`
	State      string    `json:"state" db:"state"`
	Country    string    `json:"country" db:"country"`
	PostalCode string    `json:"postal_code" db:"postal_code"`
	IsPrimary  bool      `json:"is_primary" db:"is_primary"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Contact represents additional contact information for a profile
type Contact struct {
	ID        uuid.UUID `json:"id" db:"id"`
	ProfileID uuid.UUID `json:"profile_id" db:"profile_id"`
	Type      string    `json:"type" db:"type"`
	Value     string    `json:"value" db:"value"`
	IsPrimary bool      `json:"is_primary" db:"is_primary"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ProfileRequest represents the data needed to create or update a profile
type ProfileRequest struct {
	FirstName string    `json:"first_name" validate:"required"`
	LastName  string    `json:"last_name" validate:"required"`
	Email     string    `json:"email" validate:"required,email"`
	Phone     string    `json:"phone,omitempty"`
	Addresses []Address `json:"addresses,omitempty"`
	Contacts  []Contact `json:"contacts,omitempty"`
}

// Validation errors
var (
	ErrInvalidFirstName = errors.New("first name is required and must be between 1 and 100 characters")
	ErrInvalidLastName  = errors.New("last name is required and must be between 1 and 100 characters")
	ErrInvalidEmail     = errors.New("email is required and must be a valid email address")
	ErrInvalidPhone     = errors.New("phone number must be a valid format")
	ErrInvalidAddress   = errors.New("address is invalid")
	ErrInvalidContact   = errors.New("contact is invalid")
)

// Validate performs validation on the ProfileRequest
func (p *ProfileRequest) Validate() error {
	// Validate first name
	if strings.TrimSpace(p.FirstName) == "" || len(p.FirstName) > 100 {
		return ErrInvalidFirstName
	}

	// Validate last name
	if strings.TrimSpace(p.LastName) == "" || len(p.LastName) > 100 {
		return ErrInvalidLastName
	}

	// Validate email
	if !isValidEmail(p.Email) {
		return ErrInvalidEmail
	}

	// Validate phone (if provided)
	if p.Phone != "" && !isValidPhone(p.Phone) {
		return ErrInvalidPhone
	}

	// Validate addresses
	for i, addr := range p.Addresses {
		if err := validateAddress(&addr); err != nil {
			return fmt.Errorf("address %d: %w", i+1, err)
		}
	}

	// Validate contacts
	for i, contact := range p.Contacts {
		if err := validateContact(&contact); err != nil {
			return fmt.Errorf("contact %d: %w", i+1, err)
		}
	}

	return nil
}

// Helper validation functions

func isValidEmail(email string) bool {
	// Basic email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}

	if !strings.Contains(parts[1], ".") {
		return false
	}

	return true
}

func isValidPhone(phone string) bool {
	// Basic phone validation
	// Remove all non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	// Check if we have a reasonable number of digits
	return len(digits) >= 10 && len(digits) <= 15
}

func validateAddress(addr *Address) error {
	if strings.TrimSpace(addr.Street) == "" {
		return fmt.Errorf("%w: street is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(addr.City) == "" {
		return fmt.Errorf("%w: city is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(addr.State) == "" {
		return fmt.Errorf("%w: state is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(addr.Country) == "" {
		return fmt.Errorf("%w: country is required", ErrInvalidAddress)
	}
	if strings.TrimSpace(addr.PostalCode) == "" {
		return fmt.Errorf("%w: postal code is required", ErrInvalidAddress)
	}
	return nil
}

func validateContact(contact *Contact) error {
	if strings.TrimSpace(contact.Type) == "" {
		return fmt.Errorf("%w: type is required", ErrInvalidContact)
	}
	if strings.TrimSpace(contact.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidContact)
	}
	return nil
}
