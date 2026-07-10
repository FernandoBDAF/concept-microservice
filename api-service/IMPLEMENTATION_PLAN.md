# API Service - Implementation Plan

**Project:** api-service  
**Language:** Go  
**Status:** ✅ Base implementation complete, enhancements pending  
**Session Focus:** Add document upload capability and MinIO integration  
**Last Updated:** January 2026

---

## 1. Current State

### 1.1 Completed Features
- ✅ Profile CRUD API with PostgreSQL
- ✅ Redis cache-aside pattern
- ✅ RabbitMQ task publishing (email, image, profile)
- ✅ Auth middleware with circuit breaker to auth-service
- ✅ Health checks and Prometheus metrics
- ✅ Logging with Zap
- ✅ Graceful shutdown

### 1.2 Missing Features (This Plan)
- ❌ Document upload endpoint (`POST /api/v1/documents/upload`)
- ❌ MinIO client integration
- ❌ Document metadata storage in PostgreSQL
- ❌ `document.process` routing key for GraphRAG
- ❌ Document retrieval and status endpoints

---

## 2. Implementation Tasks

### Phase 1: MinIO Client Integration

**Location:** `internal/infrastructure/minio/`

#### Task 1.1: Add Dependencies

```bash
cd api-service
go get github.com/minio/minio-go/v7@v7.0.66
```

> **Note:** `github.com/gabriel-vasile/mimetype` is already an indirect dependency and will become direct when used.

**Files to modify:**
- `go.mod` - Add MinIO dependency

#### Task 1.2: Add Configuration

**File:** `internal/config/config.go`

Add to Config struct:

```go
type Config struct {
    Server         ServerConfig         `mapstructure:"server"`
    Postgres       PostgresConfig       `mapstructure:"postgres"`
    Redis          RedisConfig          `mapstructure:"redis"`
    RabbitMQ       RabbitMQConfig       `mapstructure:"rabbitmq"`
    Auth           AuthConfig           `mapstructure:"auth"`
    Logging        LoggingConfig        `mapstructure:"logging"`
    Metrics        MetricsConfig        `mapstructure:"metrics"`
    CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
    Cache          CacheConfig          `mapstructure:"cache"`
    MinIO          MinIOConfig          `mapstructure:"minio"`  // ADD THIS
}

// ADD THIS STRUCT
type MinIOConfig struct {
    Endpoint        string `mapstructure:"endpoint"`
    AccessKeyID     string `mapstructure:"access_key_id"`
    SecretAccessKey string `mapstructure:"secret_access_key"`
    UseSSL          bool   `mapstructure:"use_ssl"`
    BucketName      string `mapstructure:"bucket_name"`
    MaxUploadSize   int64  `mapstructure:"max_upload_size"`
}
```

Add environment variable bindings in `Load()`:

```go
viper.BindEnv("minio.endpoint", "MINIO_ENDPOINT")
viper.BindEnv("minio.access_key_id", "MINIO_ACCESS_KEY")
viper.BindEnv("minio.secret_access_key", "MINIO_SECRET_KEY")
viper.BindEnv("minio.use_ssl", "MINIO_USE_SSL")
viper.BindEnv("minio.bucket_name", "MINIO_BUCKET_NAME")
viper.BindEnv("minio.max_upload_size", "MINIO_MAX_UPLOAD_SIZE")
```

Add defaults in `setDefaults()`:

```go
viper.SetDefault("minio.endpoint", "minio:9000")
viper.SetDefault("minio.access_key_id", "")
viper.SetDefault("minio.secret_access_key", "")
viper.SetDefault("minio.use_ssl", false)
viper.SetDefault("minio.bucket_name", "documents-raw")
viper.SetDefault("minio.max_upload_size", 104857600) // 100MB
```

Add validation in `Validate()`:

```go
// MinIO validation (optional - only if enabled)
if c.MinIO.Endpoint != "" {
    if c.MinIO.AccessKeyID == "" {
        return fmt.Errorf("minio access key is required when endpoint is set")
    }
    if c.MinIO.SecretAccessKey == "" {
        return fmt.Errorf("minio secret key is required when endpoint is set")
    }
}
```

#### Task 1.3: Create MinIO Client

**File:** `internal/infrastructure/minio/client.go`

```go
package minio

import (
    "context"
    "fmt"
    "io"
    "time"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    "go.uber.org/zap"
)

// Config holds MinIO configuration
type Config struct {
    Endpoint        string
    AccessKeyID     string
    SecretAccessKey string
    UseSSL          bool
    BucketName      string
    MaxUploadSize   int64
}

// Client wraps the MinIO client with application-specific methods
type Client struct {
    client     *minio.Client
    bucketName string
    maxSize    int64
    logger     *zap.Logger
}

// NewClient creates a new MinIO client and ensures the bucket exists
func NewClient(cfg Config, logger *zap.Logger) (*Client, error) {
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
        Secure: cfg.UseSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create minio client: %w", err)
    }

    // Ensure bucket exists
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    exists, err := client.BucketExists(ctx, cfg.BucketName)
    if err != nil {
        return nil, fmt.Errorf("failed to check bucket existence: %w", err)
    }

    if !exists {
        err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
        if err != nil {
            return nil, fmt.Errorf("failed to create bucket: %w", err)
        }
        logger.Info("Created MinIO bucket", zap.String("bucket", cfg.BucketName))
    }

    return &Client{
        client:     client,
        bucketName: cfg.BucketName,
        maxSize:    cfg.MaxUploadSize,
        logger:     logger.Named("minio"),
    }, nil
}

// Upload uploads a file to MinIO
func (c *Client) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
    if size > c.maxSize {
        return fmt.Errorf("file size %d exceeds maximum allowed size %d", size, c.maxSize)
    }

    _, err := c.client.PutObject(ctx, c.bucketName, objectName, reader, size, minio.PutObjectOptions{
        ContentType: contentType,
    })
    if err != nil {
        return fmt.Errorf("failed to upload object: %w", err)
    }

    c.logger.Debug("Uploaded object",
        zap.String("bucket", c.bucketName),
        zap.String("object", objectName),
        zap.Int64("size", size),
    )

    return nil
}

// Download returns a reader for the specified object
func (c *Client) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
    obj, err := c.client.GetObject(ctx, c.bucketName, objectName, minio.GetObjectOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get object: %w", err)
    }
    return obj, nil
}

// Delete removes an object from MinIO
func (c *Client) Delete(ctx context.Context, objectName string) error {
    err := c.client.RemoveObject(ctx, c.bucketName, objectName, minio.RemoveObjectOptions{})
    if err != nil {
        return fmt.Errorf("failed to delete object: %w", err)
    }

    c.logger.Debug("Deleted object",
        zap.String("bucket", c.bucketName),
        zap.String("object", objectName),
    )

    return nil
}

// GetPresignedURL returns a presigned URL for downloading the object
func (c *Client) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
    url, err := c.client.PresignedGetObject(ctx, c.bucketName, objectName, expiry, nil)
    if err != nil {
        return "", fmt.Errorf("failed to generate presigned URL: %w", err)
    }
    return url.String(), nil
}

// HealthCheck verifies MinIO connectivity
func (c *Client) HealthCheck(ctx context.Context) error {
    _, err := c.client.BucketExists(ctx, c.bucketName)
    return err
}

// BucketName returns the configured bucket name
func (c *Client) BucketName() string {
    return c.bucketName
}
```

---

### Phase 2: Document Domain Layer

**Location:** `internal/domain/document/`

#### Task 2.1: Create Document Model

**File:** `internal/domain/document/model.go`

```go
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

// DocumentStatus represents the processing status of a document
type DocumentStatus string

const (
    StatusPending    DocumentStatus = "pending"
    StatusProcessing DocumentStatus = "processing"
    StatusCompleted  DocumentStatus = "completed"
    StatusFailed     DocumentStatus = "failed"
)

// AllowedFileTypes defines the permitted file extensions
var AllowedFileTypes = map[string]bool{
    ".pdf":  true,
    ".txt":  true,
    ".md":   true,
    ".docx": true,
    ".doc":  true,
}

// AllowedMimeTypes defines the permitted MIME types
var AllowedMimeTypes = map[string]bool{
    "application/pdf":                                                              true,
    "text/plain":                                                                    true,
    "text/markdown":                                                                 true,
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document":      true,
    "application/msword":                                                            true,
}

// JSONMap is a map type that can be stored as JSONB in PostgreSQL
type JSONMap map[string]interface{}

// Scan implements the sql.Scanner interface for JSONMap
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

// Value implements the driver.Valuer interface for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
    if j == nil {
        return []byte("{}"), nil
    }
    return json.Marshal(j)
}

// Document represents a document stored in the system
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

// UploadRequest represents a document upload request
type UploadRequest struct {
    ProfileID uuid.UUID `json:"profile_id" binding:"required"`
    Metadata  JSONMap   `json:"metadata,omitempty"`
}

// DocumentResponse is the API response for document operations
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

// ToResponse converts Document to DocumentResponse
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

// Validation errors
var (
    ErrInvalidFileType    = errors.New("file type not allowed")
    ErrInvalidMimeType    = errors.New("mime type not allowed")
    ErrFileTooLarge       = errors.New("file size exceeds maximum allowed")
    ErrEmptyFile          = errors.New("file is empty")
    ErrInvalidProfileID   = errors.New("invalid profile ID")
    ErrDocumentNotFound   = errors.New("document not found")
)

// ValidateFileType checks if the file extension is allowed
func ValidateFileType(filename string) error {
    ext := strings.ToLower(filepath.Ext(filename))
    if !AllowedFileTypes[ext] {
        return fmt.Errorf("%w: %s", ErrInvalidFileType, ext)
    }
    return nil
}

// ValidateMimeType checks if the MIME type is allowed
func ValidateMimeType(mimeType string) error {
    if !AllowedMimeTypes[mimeType] {
        return fmt.Errorf("%w: %s", ErrInvalidMimeType, mimeType)
    }
    return nil
}

// GetFileType extracts the file extension without the dot
func GetFileType(filename string) string {
    ext := filepath.Ext(filename)
    if ext != "" {
        return ext[1:] // Remove leading dot
    }
    return ""
}
```

#### Task 2.2: Create Document Repository Interface

**File:** `internal/domain/document/repository.go`

```go
package document

import (
    "context"

    "github.com/google/uuid"
)

// Repository defines the interface for document persistence operations
type Repository interface {
    // Create inserts a new document record
    Create(ctx context.Context, doc *Document) error

    // GetByID retrieves a document by its ID
    GetByID(ctx context.Context, id uuid.UUID) (*Document, error)

    // GetByProfileID retrieves documents for a specific profile with pagination
    GetByProfileID(ctx context.Context, profileID uuid.UUID, limit, offset int) ([]*Document, error)

    // CountByProfileID returns the total number of documents for a profile
    CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error)

    // UpdateStatus updates the processing status of a document
    UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error

    // UpdateProcessingStarted marks a document as processing with start time
    UpdateProcessingStarted(ctx context.Context, id uuid.UUID) error

    // UpdateProcessingCompleted marks a document as completed with completion time
    UpdateProcessingCompleted(ctx context.Context, id uuid.UUID) error

    // Delete removes a document record
    Delete(ctx context.Context, id uuid.UUID) error
}
```

#### Task 2.3: Create Document Service

**File:** `internal/domain/document/service.go`

```go
package document

import (
    "context"
    "fmt"
    "io"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "go.uber.org/zap"
)

// MinIOClient defines the interface for object storage operations
type MinIOClient interface {
    Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
    GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
    Delete(ctx context.Context, objectName string) error
    BucketName() string
}

// TaskPublisher defines the interface for publishing tasks to the queue
type TaskPublisher interface {
    PublishDocumentTask(ctx context.Context, documentID, profileID, userID uuid.UUID, storagePath, bucket string) (string, error)
}

// Service handles document business logic
type Service struct {
    repo      Repository
    minio     MinIOClient
    publisher TaskPublisher
    logger    *zap.Logger
}

// NewService creates a new document service
func NewService(repo Repository, minio MinIOClient, publisher TaskPublisher, logger *zap.Logger) *Service {
    return &Service{
        repo:      repo,
        minio:     minio,
        publisher: publisher,
        logger:    logger.Named("document_service"),
    }
}

// Upload handles document upload: stores file in MinIO, creates DB record, publishes task
func (s *Service) Upload(ctx context.Context, userID, profileID uuid.UUID, filename string, reader io.Reader, size int64, mimeType string) (*Document, string, error) {
    // Validate file type
    if err := ValidateFileType(filename); err != nil {
        return nil, "", err
    }

    // Validate MIME type
    if err := ValidateMimeType(mimeType); err != nil {
        return nil, "", err
    }

    // Validate file size
    if size <= 0 {
        return nil, "", ErrEmptyFile
    }

    // Generate unique storage path
    docID := uuid.New()
    ext := filepath.Ext(filename)
    storagePath := fmt.Sprintf("%s/%s/%s%s", 
        profileID.String(), 
        time.Now().Format("2006/01/02"),
        docID.String(),
        ext,
    )

    // Upload to MinIO
    if err := s.minio.Upload(ctx, storagePath, reader, size, mimeType); err != nil {
        s.logger.Error("Failed to upload file to MinIO",
            zap.Error(err),
            zap.String("path", storagePath),
        )
        return nil, "", fmt.Errorf("failed to upload file: %w", err)
    }

    // Create document record
    doc := &Document{
        ID:               docID,
        ProfileID:        profileID,
        UserID:           userID,
        Filename:         docID.String() + ext,
        OriginalFilename: filename,
        FileType:         GetFileType(filename),
        FileSize:         size,
        StoragePath:      storagePath,
        StorageBucket:    s.minio.BucketName(),
        MimeType:         mimeType,
        Status:           StatusPending,
        Metadata:         make(JSONMap),
    }

    if err := s.repo.Create(ctx, doc); err != nil {
        // Try to clean up MinIO upload on DB failure
        _ = s.minio.Delete(ctx, storagePath)
        s.logger.Error("Failed to create document record",
            zap.Error(err),
            zap.String("document_id", docID.String()),
        )
        return nil, "", fmt.Errorf("failed to create document record: %w", err)
    }

    // Publish task to queue
    taskID, err := s.publisher.PublishDocumentTask(ctx, doc.ID, doc.ProfileID, doc.UserID, doc.StoragePath, doc.StorageBucket)
    if err != nil {
        s.logger.Warn("Failed to publish document task, document will need manual processing",
            zap.Error(err),
            zap.String("document_id", docID.String()),
        )
        // Don't fail the upload, just log the warning
        // Document can be reprocessed later
    }

    s.logger.Info("Document uploaded successfully",
        zap.String("document_id", docID.String()),
        zap.String("profile_id", profileID.String()),
        zap.String("filename", filename),
        zap.Int64("size", size),
        zap.String("task_id", taskID),
    )

    return doc, taskID, nil
}

// GetByID retrieves a document by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Document, error) {
    doc, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if doc == nil {
        return nil, ErrDocumentNotFound
    }
    return doc, nil
}

// GetByProfileID retrieves documents for a profile with pagination
func (s *Service) GetByProfileID(ctx context.Context, profileID uuid.UUID, page, pageSize int) ([]*Document, int, error) {
    if page < 1 {
        page = 1
    }
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }
    offset := (page - 1) * pageSize

    docs, err := s.repo.GetByProfileID(ctx, profileID, pageSize, offset)
    if err != nil {
        return nil, 0, err
    }

    total, err := s.repo.CountByProfileID(ctx, profileID)
    if err != nil {
        return nil, 0, err
    }

    return docs, total, nil
}

// GetDownloadURL returns a presigned URL for downloading the document
func (s *Service) GetDownloadURL(ctx context.Context, id uuid.UUID) (string, error) {
    doc, err := s.GetByID(ctx, id)
    if err != nil {
        return "", err
    }

    // Generate presigned URL valid for 15 minutes
    url, err := s.minio.GetPresignedURL(ctx, doc.StoragePath, 15*time.Minute)
    if err != nil {
        return "", fmt.Errorf("failed to generate download URL: %w", err)
    }

    return url, nil
}

// Delete removes a document from both storage and database
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
    doc, err := s.GetByID(ctx, id)
    if err != nil {
        return err
    }

    // Delete from MinIO first
    if err := s.minio.Delete(ctx, doc.StoragePath); err != nil {
        s.logger.Warn("Failed to delete file from MinIO, continuing with DB deletion",
            zap.Error(err),
            zap.String("document_id", id.String()),
        )
        // Continue with DB deletion even if MinIO delete fails
    }

    // Delete from database
    if err := s.repo.Delete(ctx, id); err != nil {
        return fmt.Errorf("failed to delete document record: %w", err)
    }

    s.logger.Info("Document deleted",
        zap.String("document_id", id.String()),
        zap.String("profile_id", doc.ProfileID.String()),
    )

    return nil
}

// UpdateStatus updates the processing status of a document
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error {
    return s.repo.UpdateStatus(ctx, id, status, errorMsg)
}
```

---

### Phase 3: Database Migration

#### Task 3.1: Create Migration Files

**File:** `migrations/000002_create_documents.up.sql`

```sql
-- Create trigger function for auto-updating updated_at (if not exists)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    storage_bucket VARCHAR(100) NOT NULL,
    mime_type VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    processing_started_at TIMESTAMP WITH TIME ZONE,
    processing_completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_documents_profile_id ON documents(profile_id);
CREATE INDEX idx_documents_user_id ON documents(user_id);
CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_created_at ON documents(created_at DESC);

-- Composite index for filtering by profile and status
CREATE INDEX idx_documents_profile_status ON documents(profile_id, status);

-- Trigger for auto-updating updated_at
CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

**File:** `migrations/000002_create_documents.down.sql`

```sql
DROP TRIGGER IF EXISTS update_documents_updated_at ON documents;
DROP TABLE IF EXISTS documents;
-- Note: We don't drop the function as it may be used by other tables
```

---

### Phase 4: PostgreSQL Repository

**File:** `internal/infrastructure/postgres/document_repository.go`

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "go.uber.org/zap"

    "github.com/fernandobarroso/microservices/api-service/internal/domain/document"
)

// DocumentRepository handles database operations for documents
type DocumentRepository struct {
    db  *sqlx.DB
    log *zap.Logger
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *sqlx.DB, log *zap.Logger) *DocumentRepository {
    return &DocumentRepository{
        db:  db,
        log: log.Named("document_repository"),
    }
}

// Create inserts a new document record
func (r *DocumentRepository) Create(ctx context.Context, doc *document.Document) error {
    query := `
        INSERT INTO documents (
            id, profile_id, user_id, filename, original_filename, file_type,
            file_size, storage_path, storage_bucket, mime_type, status, metadata
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        RETURNING created_at, updated_at`

    err := r.db.QueryRowxContext(ctx, query,
        doc.ID,
        doc.ProfileID,
        doc.UserID,
        doc.Filename,
        doc.OriginalFilename,
        doc.FileType,
        doc.FileSize,
        doc.StoragePath,
        doc.StorageBucket,
        doc.MimeType,
        doc.Status,
        doc.Metadata,
    ).Scan(&doc.CreatedAt, &doc.UpdatedAt)

    if err != nil {
        return fmt.Errorf("error creating document: %w", err)
    }

    return nil
}

// GetByID retrieves a document by its ID
func (r *DocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*document.Document, error) {
    query := `
        SELECT id, profile_id, user_id, filename, original_filename, file_type,
               file_size, storage_path, storage_bucket, mime_type, status,
               processing_started_at, processing_completed_at, error_message,
               metadata, created_at, updated_at
        FROM documents
        WHERE id = $1`

    var doc document.Document
    err := r.db.GetContext(ctx, &doc, query, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, fmt.Errorf("error getting document: %w", err)
    }

    return &doc, nil
}

// GetByProfileID retrieves documents for a profile with pagination
func (r *DocumentRepository) GetByProfileID(ctx context.Context, profileID uuid.UUID, limit, offset int) ([]*document.Document, error) {
    query := `
        SELECT id, profile_id, user_id, filename, original_filename, file_type,
               file_size, storage_path, storage_bucket, mime_type, status,
               processing_started_at, processing_completed_at, error_message,
               metadata, created_at, updated_at
        FROM documents
        WHERE profile_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

    var docs []*document.Document
    err := r.db.SelectContext(ctx, &docs, query, profileID, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("error listing documents: %w", err)
    }

    return docs, nil
}

// CountByProfileID returns the total count of documents for a profile
func (r *DocumentRepository) CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error) {
    query := `SELECT COUNT(*) FROM documents WHERE profile_id = $1`

    var count int
    err := r.db.GetContext(ctx, &count, query, profileID)
    if err != nil {
        return 0, fmt.Errorf("error counting documents: %w", err)
    }

    return count, nil
}

// UpdateStatus updates the processing status and optionally the error message
func (r *DocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status document.DocumentStatus, errorMsg *string) error {
    query := `
        UPDATE documents
        SET status = $1, error_message = $2
        WHERE id = $3`

    result, err := r.db.ExecContext(ctx, query, status, errorMsg, id)
    if err != nil {
        return fmt.Errorf("error updating document status: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected: %w", err)
    }

    if rows == 0 {
        return document.ErrDocumentNotFound
    }

    return nil
}

// UpdateProcessingStarted sets the processing_started_at timestamp and status
func (r *DocumentRepository) UpdateProcessingStarted(ctx context.Context, id uuid.UUID) error {
    query := `
        UPDATE documents
        SET status = $1, processing_started_at = $2
        WHERE id = $3`

    result, err := r.db.ExecContext(ctx, query, document.StatusProcessing, time.Now(), id)
    if err != nil {
        return fmt.Errorf("error updating processing start: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected: %w", err)
    }

    if rows == 0 {
        return document.ErrDocumentNotFound
    }

    return nil
}

// UpdateProcessingCompleted sets the processing_completed_at timestamp and status
func (r *DocumentRepository) UpdateProcessingCompleted(ctx context.Context, id uuid.UUID) error {
    query := `
        UPDATE documents
        SET status = $1, processing_completed_at = $2
        WHERE id = $3`

    result, err := r.db.ExecContext(ctx, query, document.StatusCompleted, time.Now(), id)
    if err != nil {
        return fmt.Errorf("error updating processing completion: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected: %w", err)
    }

    if rows == 0 {
        return document.ErrDocumentNotFound
    }

    return nil
}

// Delete removes a document record
func (r *DocumentRepository) Delete(ctx context.Context, id uuid.UUID) error {
    query := `DELETE FROM documents WHERE id = $1`

    result, err := r.db.ExecContext(ctx, query, id)
    if err != nil {
        return fmt.Errorf("error deleting document: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected: %w", err)
    }

    if rows == 0 {
        return document.ErrDocumentNotFound
    }

    return nil
}
```

---

### Phase 5: Task Publisher Extension

**File:** `internal/domain/task/publisher.go` (new file)

```go
package task

import (
    "context"
    "encoding/json"

    "github.com/google/uuid"
)

// DocumentTaskPublisher wraps the Service to provide document-specific publishing
type DocumentTaskPublisher struct {
    service *Service
}

// NewDocumentTaskPublisher creates a new document task publisher
func NewDocumentTaskPublisher(service *Service) *DocumentTaskPublisher {
    return &DocumentTaskPublisher{service: service}
}

// PublishDocumentTask publishes a document processing task to the queue
func (p *DocumentTaskPublisher) PublishDocumentTask(ctx context.Context, documentID, profileID, userID uuid.UUID, storagePath, bucket string) (string, error) {
    payload := map[string]interface{}{
        "document_id":  documentID.String(),
        "profile_id":   profileID.String(),
        "user_id":      userID.String(),
        "storage_path": storagePath,
        "bucket":       bucket,
    }

    metadata := map[string]string{
        "source":      "api-service",
        "document_id": documentID.String(),
        "user_id":     userID.String(),
    }

    return p.service.Submit(ctx, "document.process", "document.process", payload, metadata)
}
```

**File:** `internal/domain/task/model.go` (update)

Add to `DefaultRoutingMap`:

```go
"document.process": {
    Exchange:      "document-tasks",
    Queue:         "document-processing",
    TTL:           12 * time.Hour,
    Prefetch:      1,
    Durable:       true,
    AutoDelete:    false,
    Exclusive:     false,
    NoWait:        false,
    DeadLetterTTL: 7 * 24 * time.Hour,
    MaxRetries:    3,
    Description:   "Document processing for GraphRAG with long TTL",
},
```

---

### Phase 6: Document API Handler

**File:** `internal/api/handlers/document.go`

```go
package handlers

import (
    "errors"
    "net/http"
    "strconv"

    "github.com/gabriel-vasile/mimetype"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/fernandobarroso/microservices/api-service/internal/domain/document"
)

// DocumentHandler handles document-related HTTP requests
type DocumentHandler struct {
    service *document.Service
    logger  *zap.Logger
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(service *document.Service, logger *zap.Logger) *DocumentHandler {
    return &DocumentHandler{
        service: service,
        logger:  logger.Named("document_handler"),
    }
}

// Upload handles multipart/form-data file upload
// POST /api/v1/documents/upload
func (h *DocumentHandler) Upload(c *gin.Context) {
    // Get user ID from context (set by auth middleware)
    userIDVal, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
        return
    }
    userID, err := uuid.Parse(toString(userIDVal))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }

    // Get profile ID from form
    profileIDStr := c.PostForm("profile_id")
    if profileIDStr == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "profile_id is required"})
        return
    }
    profileID, err := uuid.Parse(profileIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id"})
        return
    }

    // Get file from form
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
        return
    }
    defer file.Close()

    // Detect MIME type
    mtype, err := mimetype.DetectReader(file)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "failed to detect file type"})
        return
    }

    // Reset file reader after MIME detection
    if _, err := file.Seek(0, 0); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process file"})
        return
    }

    // Upload document
    doc, taskID, err := h.service.Upload(
        c.Request.Context(),
        userID,
        profileID,
        header.Filename,
        file,
        header.Size,
        mtype.String(),
    )
    if err != nil {
        h.handleError(c, err)
        return
    }

    c.JSON(http.StatusAccepted, gin.H{
        "document_id": doc.ID,
        "task_id":     taskID,
        "filename":    doc.OriginalFilename,
        "status":      doc.Status,
        "message":     "Document uploaded successfully, processing queued",
    })
}

// GetByID returns document details
// GET /api/v1/documents/:id
func (h *DocumentHandler) GetByID(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
        return
    }

    doc, err := h.service.GetByID(c.Request.Context(), id)
    if err != nil {
        h.handleError(c, err)
        return
    }

    c.JSON(http.StatusOK, doc.ToResponse())
}

// GetStatus returns processing status
// GET /api/v1/documents/:id/status
func (h *DocumentHandler) GetStatus(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
        return
    }

    doc, err := h.service.GetByID(c.Request.Context(), id)
    if err != nil {
        h.handleError(c, err)
        return
    }

    response := gin.H{
        "id":     doc.ID,
        "status": doc.Status,
    }

    if doc.ProcessingStartedAt != nil {
        response["processing_started_at"] = doc.ProcessingStartedAt
    }
    if doc.ProcessingCompletedAt != nil {
        response["processing_completed_at"] = doc.ProcessingCompletedAt
    }
    if doc.ErrorMessage != nil {
        response["error_message"] = doc.ErrorMessage
    }

    c.JSON(http.StatusOK, response)
}

// Download returns presigned URL for download
// GET /api/v1/documents/:id/download
func (h *DocumentHandler) Download(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
        return
    }

    url, err := h.service.GetDownloadURL(c.Request.Context(), id)
    if err != nil {
        h.handleError(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "download_url": url,
        "expires_in":   "15 minutes",
    })
}

// Delete removes document
// DELETE /api/v1/documents/:id
func (h *DocumentHandler) Delete(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
        return
    }

    if err := h.service.Delete(c.Request.Context(), id); err != nil {
        h.handleError(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Document deleted successfully",
    })
}

// ListByProfile returns documents for a profile
// GET /api/v1/profiles/:id/documents
func (h *DocumentHandler) ListByProfile(c *gin.Context) {
    profileID, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
        return
    }

    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

    docs, total, err := h.service.GetByProfileID(c.Request.Context(), profileID, page, pageSize)
    if err != nil {
        h.handleError(c, err)
        return
    }

    // Convert to response format
    responses := make([]*document.DocumentResponse, len(docs))
    for i, doc := range docs {
        responses[i] = doc.ToResponse()
    }

    c.JSON(http.StatusOK, gin.H{
        "documents":  responses,
        "total":      total,
        "page":       page,
        "page_size":  pageSize,
        "total_pages": (total + pageSize - 1) / pageSize,
    })
}

// handleError maps domain errors to HTTP responses
func (h *DocumentHandler) handleError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, document.ErrDocumentNotFound):
        c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
    case errors.Is(err, document.ErrInvalidFileType):
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    case errors.Is(err, document.ErrInvalidMimeType):
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    case errors.Is(err, document.ErrFileTooLarge):
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    case errors.Is(err, document.ErrEmptyFile):
        c.JSON(http.StatusBadRequest, gin.H{"error": "file cannot be empty"})
    default:
        h.logger.Error("Unexpected error", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
    }
}

func toString(value interface{}) string {
    switch v := value.(type) {
    case string:
        return v
    default:
        return ""
    }
}
```

---

### Phase 7: Update Router

**File:** `internal/api/router.go` (update)

```go
package api

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.uber.org/zap"

    "github.com/fernandobarroso/microservices/api-service/internal/api/handlers"
    "github.com/fernandobarroso/microservices/api-service/internal/api/middleware"
    "github.com/fernandobarroso/microservices/api-service/internal/config"
    "github.com/fernandobarroso/microservices/api-service/internal/domain/document"  // ADD
    "github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
    "github.com/fernandobarroso/microservices/api-service/internal/domain/task"
    "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/auth"
)

type Router struct {
    engine *gin.Engine
}

func NewRouter(
    cfg *config.Config,
    authClient *auth.Client,
    profileService *profile.Service,
    taskService *task.Service,
    documentService *document.Service,  // ADD THIS PARAMETER
    healthHandler *handlers.HealthHandler,
    logger *zap.Logger,
) *Router {
    engine := gin.New()
    engine.Use(gin.Recovery())
    engine.Use(middleware.LoggingMiddleware(logger))
    engine.Use(middleware.MetricsMiddleware())

    // Health and metrics
    engine.GET("/health", healthHandler.Liveness)
    engine.GET("/ready", healthHandler.Readiness)
    if cfg.Metrics.Enabled {
        engine.GET(cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
    }

    v1 := engine.Group("/api/v1")
    v1.Use(middleware.AuthMiddleware(authClient, logger))
    {
        profileHandler := handlers.NewProfileHandler(profileService)
        taskHandler := handlers.NewTaskHandler(taskService)
        documentHandler := handlers.NewDocumentHandler(documentService, logger)  // ADD

        // Profile routes
        profiles := v1.Group("/profiles")
        {
            profiles.GET("", profileHandler.GetProfiles)
            profiles.GET("/:id", profileHandler.GetProfile)
            profiles.POST("", profileHandler.CreateProfile)
            profiles.PUT("/:id", profileHandler.UpdateProfile)
            profiles.DELETE("/:id", profileHandler.DeleteProfile)

            // Profile tasks
            profiles.POST("/:id/tasks", taskHandler.SubmitTask)
            profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)
            profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)
            profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask)

            // Profile documents (ADD)
            profiles.GET("/:id/documents", documentHandler.ListByProfile)
        }

        // Document routes (ADD)
        documents := v1.Group("/documents")
        {
            documents.POST("/upload", documentHandler.Upload)
            documents.GET("/:id", documentHandler.GetByID)
            documents.GET("/:id/status", documentHandler.GetStatus)
            documents.GET("/:id/download", documentHandler.Download)
            documents.DELETE("/:id", documentHandler.Delete)
        }
    }

    engine.NoRoute(func(c *gin.Context) {
        c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
    })

    return &Router{engine: engine}
}

func (r *Router) Run(addr string) error {
    return r.engine.Run(addr)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    r.engine.ServeHTTP(w, req)
}
```

---

### Phase 8: Update Main

**File:** `cmd/server/main.go` (update)

Add the following changes:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "go.uber.org/zap"

    "github.com/fernandobarroso/microservices/api-service/internal/api"
    "github.com/fernandobarroso/microservices/api-service/internal/api/handlers"
    "github.com/fernandobarroso/microservices/api-service/internal/config"
    "github.com/fernandobarroso/microservices/api-service/internal/domain/document"  // ADD
    "github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
    "github.com/fernandobarroso/microservices/api-service/internal/domain/task"
    "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/auth"
    minioInfra "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/minio"  // ADD
    "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/postgres"
    "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/rabbitmq"
    redisInfra "github.com/fernandobarroso/microservices/api-service/internal/infrastructure/redis"
    "github.com/fernandobarroso/microservices/api-service/internal/pkg/logger"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        fmt.Printf("failed to load config: %v\n", err)
        os.Exit(1)
    }

    log, err := logger.New(cfg.Logging)
    if err != nil {
        fmt.Printf("failed to init logger: %v\n", err)
        os.Exit(1)
    }
    zap.ReplaceGlobals(log)

    db, err := postgres.NewClient(cfg.Postgres)
    if err != nil {
        log.Fatal("failed to init postgres", zap.Error(err))
    }
    defer db.Close()

    var redisClient *redisInfra.Client
    if cfg.Redis.Enabled {
        redisClient, err = redisInfra.NewClient(cfg.Redis)
        if err != nil {
            log.Fatal("failed to init redis", zap.Error(err))
        }
        defer redisClient.Close()
    }

    rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
    if err != nil {
        log.Fatal("failed to init rabbitmq", zap.Error(err))
    }
    defer rmqClient.Close()

    // Initialize MinIO client (ADD THIS BLOCK)
    var minioClient *minioInfra.Client
    if cfg.MinIO.Endpoint != "" {
        minioCfg := minioInfra.Config{
            Endpoint:        cfg.MinIO.Endpoint,
            AccessKeyID:     cfg.MinIO.AccessKeyID,
            SecretAccessKey: cfg.MinIO.SecretAccessKey,
            UseSSL:          cfg.MinIO.UseSSL,
            BucketName:      cfg.MinIO.BucketName,
            MaxUploadSize:   cfg.MinIO.MaxUploadSize,
        }
        minioClient, err = minioInfra.NewClient(minioCfg, log)
        if err != nil {
            log.Fatal("failed to init minio", zap.Error(err))
        }
        log.Info("MinIO client initialized", zap.String("bucket", cfg.MinIO.BucketName))
    }

    authClient := auth.NewClient(cfg.Auth, cfg.CircuitBreaker)
    profileRepo := postgres.NewProfileRepository(db, log)

    var cache profile.Cache
    if redisClient != nil {
        cache = redisInfra.NewCache(redisClient, cfg.Cache)
    }
    profileService := profile.NewService(profileRepo, cache, redisInfra.ErrCacheMiss)

    publisher := rabbitmq.NewPublisher(rmqClient)
    taskService := task.NewService(publisher)

    // Initialize document service (ADD THIS BLOCK)
    var documentService *document.Service
    if minioClient != nil {
        documentRepo := postgres.NewDocumentRepository(db, log)
        documentPublisher := task.NewDocumentTaskPublisher(taskService)
        documentService = document.NewService(documentRepo, minioClient, documentPublisher, log)
        log.Info("Document service initialized")
    }

    healthHandler := handlers.NewHealthHandler(db, redisClient, rmqClient)
    router := api.NewRouter(cfg, authClient, profileService, taskService, documentService, healthHandler, log)

    server := &http.Server{
        Addr:              fmt.Sprintf(":%d", cfg.Server.HTTPPort),
        Handler:           router,
        ReadHeaderTimeout: cfg.Server.ReadTimeout,
        ReadTimeout:       cfg.Server.ReadTimeout,
        WriteTimeout:      cfg.Server.WriteTimeout,
    }

    go func() {
        log.Info("HTTP server started", zap.Int("port", cfg.Server.HTTPPort))
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal("failed to start server", zap.Error(err))
        }
    }()

    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
    <-shutdown

    ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Error("failed to shutdown server", zap.Error(err))
    }

    log.Info("server shutdown complete")
    time.Sleep(100 * time.Millisecond)
}
```

---

## 3. New Endpoints Summary

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/documents/upload` | POST | Upload document (multipart/form-data) |
| `/api/v1/documents/:id` | GET | Get document details |
| `/api/v1/documents/:id/status` | GET | Get processing status |
| `/api/v1/documents/:id/download` | GET | Get presigned download URL |
| `/api/v1/documents/:id` | DELETE | Delete document |
| `/api/v1/profiles/:id/documents` | GET | List profile documents (paginated) |

---

## 4. File Checklist

### New Files to Create

| File | Description |
|------|-------------|
| `internal/infrastructure/minio/client.go` | MinIO client wrapper |
| `internal/domain/document/model.go` | Document models and validation |
| `internal/domain/document/repository.go` | Repository interface |
| `internal/domain/document/service.go` | Document business logic |
| `internal/infrastructure/postgres/document_repository.go` | PostgreSQL implementation |
| `internal/api/handlers/document.go` | HTTP handlers |
| `internal/domain/task/publisher.go` | Document task publisher |
| `migrations/000002_create_documents.up.sql` | Create documents table |
| `migrations/000002_create_documents.down.sql` | Drop documents table |

### Files to Modify

| File | Changes |
|------|---------|
| `go.mod` | Add `github.com/minio/minio-go/v7` |
| `internal/config/config.go` | Add MinIOConfig struct, defaults, env bindings, validation |
| `internal/domain/task/model.go` | Add `document.process` routing key |
| `internal/api/router.go` | Add documentService parameter, add document routes |
| `cmd/server/main.go` | Initialize MinIO client, document repo/service, update router call |

---

## 5. Testing Checklist

### Unit Tests
- [ ] MinIO client upload/download/delete
- [ ] Document service upload flow
- [ ] Document repository CRUD operations
- [ ] File type and MIME type validation
- [ ] JSONMap Scan/Value methods

### Integration Tests
- [ ] Upload document → MinIO stores file
- [ ] Upload document → PostgreSQL stores metadata
- [ ] Upload document → RabbitMQ receives message
- [ ] Download returns valid presigned URL
- [ ] Delete removes from both MinIO and PostgreSQL
- [ ] List documents with pagination

### Manual Tests

```bash
# Upload a document
curl -X POST http://localhost:8080/api/v1/documents/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test.pdf" \
  -F "profile_id=<uuid>"

# Expected response:
# {
#   "document_id": "...",
#   "task_id": "...",
#   "filename": "test.pdf",
#   "status": "pending",
#   "message": "Document uploaded successfully, processing queued"
# }

# Get document details
curl http://localhost:8080/api/v1/documents/<id> \
  -H "Authorization: Bearer $TOKEN"

# Check status
curl http://localhost:8080/api/v1/documents/<id>/status \
  -H "Authorization: Bearer $TOKEN"

# Get download URL
curl http://localhost:8080/api/v1/documents/<id>/download \
  -H "Authorization: Bearer $TOKEN"

# List documents for a profile
curl "http://localhost:8080/api/v1/profiles/<profile_id>/documents?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN"

# Delete document
curl -X DELETE http://localhost:8080/api/v1/documents/<id> \
  -H "Authorization: Bearer $TOKEN"
```

---

## 6. Environment Variables

### New Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MINIO_ENDPOINT` | minio:9000 | MinIO server endpoint |
| `MINIO_ACCESS_KEY` | (required) | MinIO access key |
| `MINIO_SECRET_KEY` | (required) | MinIO secret key |
| `MINIO_USE_SSL` | false | Use HTTPS for MinIO |
| `MINIO_BUCKET_NAME` | documents-raw | Default bucket name |
| `MINIO_MAX_UPLOAD_SIZE` | 104857600 | Max upload size in bytes (100MB) |

### Example `.env` Addition

```bash
# MinIO Configuration
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin123
MINIO_USE_SSL=false
MINIO_BUCKET_NAME=documents-raw
MINIO_MAX_UPLOAD_SIZE=104857600
```

---

## 7. Dependencies on Other Components

| Component | Dependency | Notes |
|-----------|------------|-------|
| MinIO | Required | Must be deployed first, creates bucket automatically |
| PostgreSQL | Required | Already available, run migration |
| RabbitMQ | Required | Already available, exchange created on first publish |
| graphrag-service | Downstream | Will consume `document.process` messages |

---

## 8. Implementation Order

Execute in this order to minimize blockers:

1. **Phase 1**: Add MinIO dependency and configuration
2. **Phase 2**: Create domain layer (model, repository interface, service)
3. **Phase 3**: Run database migration
4. **Phase 4**: Create PostgreSQL repository
5. **Phase 5**: Create task publisher extension
6. **Phase 6**: Create API handlers
7. **Phase 7**: Update router (add new parameter and routes)
8. **Phase 8**: Update main.go (wire everything together)
9. **Testing**: Run unit and integration tests

---

## 9. Success Criteria

- [ ] Document upload stores file in MinIO with correct path structure
- [ ] Document metadata stored in PostgreSQL with all fields populated
- [ ] Message published to RabbitMQ with `document.process` routing key
- [ ] Download returns working presigned URL (valid for 15 minutes)
- [ ] Status endpoint returns correct processing state
- [ ] List endpoint returns paginated results with total count
- [ ] Delete removes document from both MinIO and PostgreSQL
- [ ] All existing profile tests still pass
- [ ] No performance regression on existing endpoints
- [ ] Error handling returns appropriate HTTP status codes

---

*Document Version: 2.0*  
*Created: January 2026*  
*Updated: January 2026 (corrections applied)*  
*Estimated Effort: 2-3 days*
