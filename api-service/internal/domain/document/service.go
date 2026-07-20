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

type MinIOClient interface {
	Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, objectName string) error
	BucketName() string
}

// TaskBuilder produces the serialized document.process envelope for the
// upload transaction (satisfied by task.DocumentTaskBuilder). The envelope
// is built up front so the document INSERT and the outbox INSERT commit in
// ONE transaction (ADR-008.3) — publishing is the relay's job.
type TaskBuilder interface {
	BuildDocumentTask(ctx context.Context, documentID, profileID, userID uuid.UUID, storagePath, bucket, fileType string) (taskID, routingKey string, envelope []byte, err error)
}

type Service struct {
	repo   Repository
	minio  MinIOClient
	tasks  TaskBuilder
	logger *zap.Logger
}

func NewService(repo Repository, minio MinIOClient, tasks TaskBuilder, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		minio:  minio,
		tasks:  tasks,
		logger: logger.Named("document_service"),
	}
}

func (s *Service) Upload(ctx context.Context, userID, profileID uuid.UUID, filename string, reader io.Reader, size int64, mimeType string) (*Document, string, error) {
	if err := ValidateFileType(filename); err != nil {
		return nil, "", err
	}
	if err := ValidateMimeType(mimeType); err != nil {
		return nil, "", err
	}
	if size <= 0 {
		return nil, "", ErrEmptyFile
	}

	docID := uuid.New()
	ext := filepath.Ext(filename)
	storagePath := fmt.Sprintf("%s/%s/%s%s",
		profileID.String(),
		time.Now().Format("2006/01/02"),
		docID.String(),
		ext,
	)

	if err := s.minio.Upload(ctx, storagePath, reader, size, mimeType); err != nil {
		s.logger.Error("Failed to upload file to MinIO",
			zap.Error(err),
			zap.String("path", storagePath),
		)
		return nil, "", fmt.Errorf("failed to upload file: %w", err)
	}

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

	// Build the envelope first, then commit document + outbox event in one
	// transaction (ADR-008.3). The old swallowed direct publish is gone: a
	// committed document ALWAYS has its task queued, and the relay gets it
	// to the broker (EXP-42).
	taskID, routingKey, envelope, err := s.tasks.BuildDocumentTask(ctx, doc.ID, doc.ProfileID, doc.UserID, doc.StoragePath, doc.StorageBucket, doc.FileType)
	if err != nil {
		_ = s.minio.Delete(ctx, storagePath)
		s.logger.Error("Failed to build document task envelope",
			zap.Error(err),
			zap.String("document_id", docID.String()),
		)
		return nil, "", fmt.Errorf("failed to build document task: %w", err)
	}

	if err := s.repo.CreateWithTask(ctx, doc, routingKey, envelope); err != nil {
		_ = s.minio.Delete(ctx, storagePath)
		s.logger.Error("Failed to create document record",
			zap.Error(err),
			zap.String("document_id", docID.String()),
		)
		return nil, "", fmt.Errorf("failed to create document record: %w", err)
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

func (s *Service) GetDownloadURL(ctx context.Context, id uuid.UUID) (string, error) {
	doc, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	url, err := s.minio.GetPresignedURL(ctx, doc.StoragePath, 15*time.Minute)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	return url, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	doc, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.minio.Delete(ctx, doc.StoragePath); err != nil {
		s.logger.Warn("Failed to delete file from MinIO, continuing with DB deletion",
			zap.Error(err),
			zap.String("document_id", id.String()),
		)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete document record: %w", err)
	}

	s.logger.Info("Document deleted",
		zap.String("document_id", id.String()),
		zap.String("profile_id", doc.ProfileID.String()),
	)

	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error {
	return s.repo.UpdateStatus(ctx, id, status, errorMsg)
}

// ApplyTaskResult advances the document lifecycle from a task.result
// message (ADR-008.3): processing → completed/failed. It is idempotent —
// duplicate results (the relay may re-publish by design, deduped only
// best-effort downstream) and results for already-terminal documents are
// skipped, and an unknown document is dropped rather than redelivered
// forever. Only transient errors (e.g. DB down) are returned.
func (s *Service) ApplyTaskResult(ctx context.Context, id uuid.UUID, status string, errorMsg *string) error {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		s.logger.Warn("task result for unknown document, dropping",
			zap.String("document_id", id.String()),
			zap.String("status", status),
		)
		return nil
	}
	if doc.Status == StatusCompleted || doc.Status == StatusFailed {
		s.logger.Debug("task result for already-terminal document, skipping",
			zap.String("document_id", id.String()),
			zap.String("current_status", string(doc.Status)),
			zap.String("result_status", status),
		)
		return nil
	}

	switch status {
	case "completed":
		return s.repo.UpdateProcessingCompleted(ctx, id)
	case "failed":
		return s.repo.UpdateStatus(ctx, id, StatusFailed, errorMsg)
	default:
		s.logger.Warn("task result with unknown status, dropping",
			zap.String("document_id", id.String()),
			zap.String("status", status),
		)
		return nil
	}
}
