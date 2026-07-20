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
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/outbox"
)

type DocumentRepository struct {
	db     *sqlx.DB
	outbox *outbox.Store
	log    *zap.Logger
}

func NewDocumentRepository(db *sqlx.DB, outboxStore *outbox.Store, log *zap.Logger) *DocumentRepository {
	return &DocumentRepository{
		db:     db,
		outbox: outboxStore,
		log:    log.Named("document_repository"),
	}
}

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

// CreateWithTask is the transactional-outbox upload path (ADR-008.3): the
// document INSERT, its document.process outbox envelope, and the
// pending→processing transition commit atomically. Either the document
// exists WITH its queued task, or nothing was written — the EXP-42 crash
// window (row committed, publish lost) cannot occur.
func (r *DocumentRepository) CreateWithTask(ctx context.Context, doc *document.Document, routingKey string, envelope []byte) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting upload transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after commit

	insert := `
		INSERT INTO documents (
			id, profile_id, user_id, filename, original_filename, file_type,
			file_size, storage_path, storage_bucket, mime_type, status, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at`

	if err := tx.QueryRowxContext(ctx, insert,
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
	).Scan(&doc.CreatedAt, &doc.UpdatedAt); err != nil {
		return fmt.Errorf("error creating document: %w", err)
	}

	if err := r.outbox.Add(ctx, tx, routingKey, envelope); err != nil {
		return fmt.Errorf("error adding outbox envelope: %w", err)
	}

	// The task is now (transactionally) queued: pending → processing.
	if _, err := tx.ExecContext(ctx,
		`UPDATE documents SET status = $1 WHERE id = $2`,
		document.StatusProcessing, doc.ID,
	); err != nil {
		return fmt.Errorf("error advancing document to processing: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing upload transaction: %w", err)
	}

	doc.Status = document.StatusProcessing
	return nil
}

func (r *DocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*document.Document, error) {
	query := `
		SELECT id, profile_id, user_id, filename, original_filename, file_type,
			   file_size, storage_path, storage_bucket, mime_type, status,
			   processing_started_at, processing_completed_at, error_message,
			   metadata, created_at, updated_at
		FROM documents
		WHERE id = $1`

	var doc document.Document
	if err := r.db.GetContext(ctx, &doc, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting document: %w", err)
	}

	return &doc, nil
}

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
	if err := r.db.SelectContext(ctx, &docs, query, profileID, limit, offset); err != nil {
		return nil, fmt.Errorf("error listing documents: %w", err)
	}

	return docs, nil
}

func (r *DocumentRepository) CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM documents WHERE profile_id = $1`

	var count int
	if err := r.db.GetContext(ctx, &count, query, profileID); err != nil {
		return 0, fmt.Errorf("error counting documents: %w", err)
	}

	return count, nil
}

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
