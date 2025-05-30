package dto

import (
	"time"

	"github.com/FBDAF/microservices/services/common/models/domain"
)

// BaseResponse represents a base response DTO
type BaseResponse struct {
	RequestID string              `json:"request_id" validate:"required"`
	Timestamp time.Time           `json:"timestamp" validate:"required"`
	Metadata  domain.MetadataList `json:"metadata,omitempty"`
	Version   domain.Version      `json:"version" validate:"required"`
	Status    domain.Status       `json:"status" validate:"required"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	BaseResponse
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// FilteredResponse represents a response with filters
type FilteredResponse struct {
	BaseResponse
	Data    interface{}            `json:"data"`
	Filters map[string]interface{} `json:"filters,omitempty"`
	Sort    []string               `json:"sort,omitempty"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	BaseResponse
	Data       interface{}            `json:"data"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	TotalPages int                    `json:"total_pages"`
	Query      string                 `json:"query"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	Sort       []string               `json:"sort,omitempty"`
}

// CreateResponse represents a create response
type CreateResponse struct {
	BaseResponse
	ID   string      `json:"id" validate:"required"`
	Data interface{} `json:"data" validate:"required"`
}

// UpdateResponse represents an update response
type UpdateResponse struct {
	BaseResponse
	ID   string      `json:"id" validate:"required"`
	Data interface{} `json:"data" validate:"required"`
}

// DeleteResponse represents a delete response
type DeleteResponse struct {
	BaseResponse
	ID string `json:"id" validate:"required"`
}

// GetResponse represents a get response
type GetResponse struct {
	BaseResponse
	Data interface{} `json:"data" validate:"required"`
}

// ListResponse represents a list response
type ListResponse struct {
	BaseResponse
	Data       interface{}            `json:"data"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	TotalPages int                    `json:"total_pages"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	Sort       []string               `json:"sort,omitempty"`
}

// BatchResponse represents a batch response
type BatchResponse struct {
	BaseResponse
	Results []BatchResult `json:"results" validate:"required"`
}

// BatchResult represents a single result in a batch response
type BatchResult struct {
	Type   string        `json:"type" validate:"required"`
	ID     string        `json:"id,omitempty"`
	Data   interface{}   `json:"data,omitempty"`
	Status domain.Status `json:"status" validate:"required"`
	Error  string        `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	BaseResponse
	Error   string      `json:"error" validate:"required"`
	Code    string      `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// NewErrorResponse creates a new ErrorResponse
func NewErrorResponse(requestID string, version domain.Version, error string, code string, details interface{}) *ErrorResponse {
	return &ErrorResponse{
		BaseResponse: BaseResponse{
			RequestID: requestID,
			Timestamp: time.Now(),
			Version:   version,
			Status:    *domain.NewStatus("error", error),
		},
		Error:   error,
		Code:    code,
		Details: details,
	}
}
