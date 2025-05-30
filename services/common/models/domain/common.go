package domain

import (
	"time"
)

// Address represents a physical address
type Address struct {
	Street     string `json:"street" validate:"required"`
	City       string `json:"city" validate:"required"`
	State      string `json:"state" validate:"required"`
	PostalCode string `json:"postal_code" validate:"required"`
	Country    string `json:"country" validate:"required"`
}

// Validate validates the Address
func (a *Address) Validate() error {
	if err := ValidateString(a.Street, ValidationRules{Required: true}); err != nil {
		return err
	}
	if err := ValidateString(a.City, ValidationRules{Required: true}); err != nil {
		return err
	}
	if err := ValidateString(a.State, ValidationRules{Required: true}); err != nil {
		return err
	}
	if err := ValidateString(a.PostalCode, ValidationRules{Required: true}); err != nil {
		return err
	}
	if err := ValidateString(a.Country, ValidationRules{Required: true}); err != nil {
		return err
	}
	return nil
}

// Contact represents contact information
type Contact struct {
	Email       string `json:"email" validate:"required,email"`
	Phone       string `json:"phone" validate:"required"`
	Alternative string `json:"alternative,omitempty"`
}

// Validate validates the Contact
func (c *Contact) Validate() error {
	if err := ValidateString(c.Email, ValidationRules{
		Required: true,
		Pattern:  EmailPattern,
	}); err != nil {
		return err
	}
	if err := ValidateString(c.Phone, ValidationRules{
		Required: true,
		Pattern:  PhonePattern,
	}); err != nil {
		return err
	}
	return nil
}

// TimeRange represents a time period
type TimeRange struct {
	Start CustomTime `json:"start" validate:"required"`
	End   CustomTime `json:"end" validate:"required"`
}

// Validate validates the TimeRange
func (tr *TimeRange) Validate() error {
	if tr.Start.After(tr.End.Time) {
		return &ValidationError{
			Field:   "time_range",
			Message: "start time must be before end time",
		}
	}
	return nil
}

// Money represents a monetary amount
type Money struct {
	Amount   float64 `json:"amount" validate:"required"`
	Currency string  `json:"currency" validate:"required,len=3"`
}

// Validate validates the Money
func (m *Money) Validate() error {
	if m.Amount < 0 {
		return &ValidationError{
			Field:   "amount",
			Message: "amount cannot be negative",
		}
	}
	return nil
}

// Pagination represents pagination parameters
type Pagination struct {
	Page     int `json:"page" validate:"required,min=1"`
	PageSize int `json:"page_size" validate:"required,min=1,max=100"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// NewPaginatedResponse creates a new PaginatedResponse
func NewPaginatedResponse(data interface{}, total int64, page, pageSize int) *PaginatedResponse {
	totalPages := (int(total) + pageSize - 1) / pageSize
	return &PaginatedResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// Status represents a status with a timestamp
type Status struct {
	Value     string     `json:"value" validate:"required"`
	Timestamp CustomTime `json:"timestamp" validate:"required"`
	Reason    string     `json:"reason,omitempty"`
}

// NewStatus creates a new Status
func NewStatus(value string, reason string) *Status {
	return &Status{
		Value:     value,
		Timestamp: CustomTime{Time: time.Now()},
		Reason:    reason,
	}
}

// Metadata represents generic metadata
type Metadata struct {
	Key   string      `json:"key" validate:"required"`
	Value interface{} `json:"value" validate:"required"`
}

// MetadataList represents a list of metadata
type MetadataList []Metadata

// Get retrieves a metadata value by key
func (ml MetadataList) Get(key string) (interface{}, bool) {
	for _, m := range ml {
		if m.Key == key {
			return m.Value, true
		}
	}
	return nil, false
}

// Set sets a metadata value
func (ml *MetadataList) Set(key string, value interface{}) {
	for i, m := range *ml {
		if m.Key == key {
			(*ml)[i].Value = value
			return
		}
	}
	*ml = append(*ml, Metadata{Key: key, Value: value})
}

// Delete removes a metadata entry
func (ml *MetadataList) Delete(key string) {
	for i, m := range *ml {
		if m.Key == key {
			*ml = append((*ml)[:i], (*ml)[i+1:]...)
			return
		}
	}
}
