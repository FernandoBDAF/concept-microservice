package dto

import (
	"time"

	"github.com/FBDAF/microservices/services/common/models/domain"
)

// BaseRequest represents a base request DTO
type BaseRequest struct {
	RequestID string              `json:"request_id" validate:"required"`
	Timestamp time.Time           `json:"timestamp" validate:"required"`
	Metadata  domain.MetadataList `json:"metadata,omitempty"`
	Version   domain.Version      `json:"version" validate:"required"`
}

// PaginatedRequest represents a paginated request
type PaginatedRequest struct {
	BaseRequest
	Page     int `json:"page" validate:"required,min=1"`
	PageSize int `json:"page_size" validate:"required,min=1,max=100"`
}

// FilteredRequest represents a request with filters
type FilteredRequest struct {
	BaseRequest
	Filters map[string]interface{} `json:"filters,omitempty"`
	Sort    []string               `json:"sort,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	BaseRequest
	Query    string                 `json:"query" validate:"required"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	Sort     []string               `json:"sort,omitempty"`
	Page     int                    `json:"page" validate:"required,min=1"`
	PageSize int                    `json:"page_size" validate:"required,min=1,max=100"`
}

// CreateRequest represents a create request
type CreateRequest struct {
	BaseRequest
	Data interface{} `json:"data" validate:"required"`
}

// UpdateRequest represents an update request
type UpdateRequest struct {
	BaseRequest
	ID   string      `json:"id" validate:"required"`
	Data interface{} `json:"data" validate:"required"`
}

// DeleteRequest represents a delete request
type DeleteRequest struct {
	BaseRequest
	ID string `json:"id" validate:"required"`
}

// GetRequest represents a get request
type GetRequest struct {
	BaseRequest
	ID string `json:"id" validate:"required"`
}

// ListRequest represents a list request
type ListRequest struct {
	BaseRequest
	Filters  map[string]interface{} `json:"filters,omitempty"`
	Sort     []string               `json:"sort,omitempty"`
	Page     int                    `json:"page" validate:"required,min=1"`
	PageSize int                    `json:"page_size" validate:"required,min=1,max=100"`
}

// BatchRequest represents a batch request
type BatchRequest struct {
	BaseRequest
	Operations []BatchOperation `json:"operations" validate:"required,min=1"`
}

// BatchOperation represents a single operation in a batch request
type BatchOperation struct {
	Type string      `json:"type" validate:"required"`
	ID   string      `json:"id,omitempty"`
	Data interface{} `json:"data,omitempty"`
}
