package domain

import (
	"time"
)

// Model interface defines the basic methods that all models should implement
type Model interface {
	GetID() string
	SetID(string)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
	GetUpdatedAt() time.Time
	SetUpdatedAt(time.Time)
	GetDeletedAt() *time.Time
	SetDeletedAt(*time.Time)
}

// BaseModel provides a basic implementation of the Model interface
type BaseModel struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// GetID returns the model's ID
func (m *BaseModel) GetID() string {
	return m.ID
}

// SetID sets the model's ID
func (m *BaseModel) SetID(id string) {
	m.ID = id
}

// GetCreatedAt returns the creation timestamp
func (m *BaseModel) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// SetCreatedAt sets the creation timestamp
func (m *BaseModel) SetCreatedAt(t time.Time) {
	m.CreatedAt = t
}

// GetUpdatedAt returns the last update timestamp
func (m *BaseModel) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// SetUpdatedAt sets the last update timestamp
func (m *BaseModel) SetUpdatedAt(t time.Time) {
	m.UpdatedAt = t
}

// GetDeletedAt returns the deletion timestamp
func (m *BaseModel) GetDeletedAt() *time.Time {
	return m.DeletedAt
}

// SetDeletedAt sets the deletion timestamp
func (m *BaseModel) SetDeletedAt(t *time.Time) {
	m.DeletedAt = t
}

// NewBaseModel creates a new BaseModel with current timestamps
func NewBaseModel() *BaseModel {
	now := time.Now()
	return &BaseModel{
		CreatedAt: now,
		UpdatedAt: now,
	}
}
