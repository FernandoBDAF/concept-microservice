package document

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DocumentStatus string

const (
	StatusPending    DocumentStatus = "pending"
	StatusProcessing DocumentStatus = "processing"
	StatusCompleted  DocumentStatus = "completed"
	StatusFailed     DocumentStatus = "failed"
)

var AllowedFileTypes = map[string]bool{
	".pdf":  true,
	".txt":  true,
	".md":   true,
	".docx": true,
	".doc":  true,
}

var AllowedMimeTypes = map[string]bool{
	"application/pdf":                                                         true,
	"text/plain":                                                               true,
	"text/markdown":                                                            true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/msword":                                                       true,
}

type JSONMap map[string]interface{}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}

type Document struct {
	ID                    uuid.UUID      `json:"id" db:"id"`
	ProfileID             uuid.UUID      `json:"profile_id" db:"profile_id"`
	UserID                uuid.UUID      `json:"user_id" db:"user_id"`
	Filename              string         `json:"filename" db:"filename"`
	OriginalFilename      string         `json:"original_filename" db:"original_filename"`
	FileType              string         `json:"file_type" db:"file_type"`
	FileSize              int64          `json:"file_size" db:"file_size"`
	StoragePath           string         `json:"storage_path" db:"storage_path"`
	StorageBucket         string         `json:"storage_bucket" db:"storage_bucket"`
	MimeType              string         `json:"mime_type" db:"mime_type"`
	Status                DocumentStatus `json:"status" db:"status"`
	ProcessingStartedAt   *time.Time     `json:"processing_started_at,omitempty" db:"processing_started_at"`
	ProcessingCompletedAt *time.Time     `json:"processing_completed_at,omitempty" db:"processing_completed_at"`
	ErrorMessage          *string        `json:"error_message,omitempty" db:"error_message"`
	Metadata              JSONMap        `json:"metadata" db:"metadata"`
	CreatedAt             time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at" db:"updated_at"`
}

type UploadRequest struct {
	ProfileID uuid.UUID `json:"profile_id" binding:"required"`
	Metadata  JSONMap   `json:"metadata,omitempty"`
}

type DocumentResponse struct {
	ID               uuid.UUID      `json:"id"`
	ProfileID        uuid.UUID      `json:"profile_id"`
	Filename         string         `json:"filename"`
	OriginalFilename string         `json:"original_filename"`
	FileType         string         `json:"file_type"`
	FileSize         int64          `json:"file_size"`
	MimeType         string         `json:"mime_type"`
	Status           DocumentStatus `json:"status"`
	ErrorMessage     *string        `json:"error_message,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

func (d *Document) ToResponse() *DocumentResponse {
	return &DocumentResponse{
		ID:               d.ID,
		ProfileID:        d.ProfileID,
		Filename:         d.Filename,
		OriginalFilename: d.OriginalFilename,
		FileType:         d.FileType,
		FileSize:         d.FileSize,
		MimeType:         d.MimeType,
		Status:           d.Status,
		ErrorMessage:     d.ErrorMessage,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

var (
	ErrInvalidFileType  = errors.New("file type not allowed")
	ErrInvalidMimeType  = errors.New("mime type not allowed")
	ErrFileTooLarge     = errors.New("file size exceeds maximum allowed")
	ErrEmptyFile        = errors.New("file is empty")
	ErrInvalidProfileID = errors.New("invalid profile ID")
	ErrDocumentNotFound = errors.New("document not found")
)

func ValidateFileType(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if !AllowedFileTypes[ext] {
		return fmt.Errorf("%w: %s", ErrInvalidFileType, ext)
	}
	return nil
}

func ValidateMimeType(mimeType string) error {
	if !AllowedMimeTypes[mimeType] {
		return fmt.Errorf("%w: %s", ErrInvalidMimeType, mimeType)
	}
	return nil
}

func GetFileType(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return ext[1:]
	}
	return ""
}
