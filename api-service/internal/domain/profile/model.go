package profile

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

type Contact struct {
	ID        uuid.UUID `json:"id" db:"id"`
	ProfileID uuid.UUID `json:"profile_id" db:"profile_id"`
	Type      string    `json:"type" db:"type"`
	Value     string    `json:"value" db:"value"`
	IsPrimary bool      `json:"is_primary" db:"is_primary"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type ProfileRequest struct {
	FirstName string           `json:"first_name" validate:"required"`
	LastName  string           `json:"last_name" validate:"required"`
	Email     string           `json:"email" validate:"required,email"`
	Phone     string           `json:"phone,omitempty"`
	Addresses []AddressRequest `json:"addresses,omitempty"`
	Contacts  []ContactRequest `json:"contacts,omitempty"`
}

type AddressRequest struct {
	Street     string `json:"street" validate:"required"`
	City       string `json:"city" validate:"required"`
	State      string `json:"state" validate:"required"`
	Country    string `json:"country" validate:"required"`
	PostalCode string `json:"postal_code" validate:"required"`
	IsPrimary  bool   `json:"is_primary"`
}

type ContactRequest struct {
	Type      string `json:"type" validate:"required"`
	Value     string `json:"value" validate:"required"`
	IsPrimary bool   `json:"is_primary"`
}

var (
	ErrInvalidFirstName = errors.New("first name is required and must be between 1 and 100 characters")
	ErrInvalidLastName  = errors.New("last name is required and must be between 1 and 100 characters")
	ErrInvalidEmail     = errors.New("email is required and must be a valid email address")
	ErrInvalidPhone     = errors.New("phone number must be a valid format")
	ErrInvalidAddress   = errors.New("address is invalid")
	ErrInvalidContact   = errors.New("contact is invalid")
)

func (p *ProfileRequest) Validate() error {
	if strings.TrimSpace(p.FirstName) == "" || len(p.FirstName) > 100 {
		return ErrInvalidFirstName
	}
	if strings.TrimSpace(p.LastName) == "" || len(p.LastName) > 100 {
		return ErrInvalidLastName
	}
	if !isValidEmail(p.Email) {
		return ErrInvalidEmail
	}
	if p.Phone != "" && !isValidPhone(p.Phone) {
		return ErrInvalidPhone
	}
	for i, addr := range p.Addresses {
		if err := validateAddressRequest(&addr); err != nil {
			return fmt.Errorf("address %d: %w", i+1, err)
		}
	}
	for i, contact := range p.Contacts {
		if err := validateContactRequest(&contact); err != nil {
			return fmt.Errorf("contact %d: %w", i+1, err)
		}
	}
	return nil
}

func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

func isValidPhone(phone string) bool {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	return len(digits) >= 10 && len(digits) <= 15
}

func validateAddressRequest(addr *AddressRequest) error {
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

func validateContactRequest(contact *ContactRequest) error {
	if strings.TrimSpace(contact.Type) == "" {
		return fmt.Errorf("%w: type is required", ErrInvalidContact)
	}
	if strings.TrimSpace(contact.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidContact)
	}
	return nil
}

