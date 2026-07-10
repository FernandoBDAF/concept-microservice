package task

import (
	"context"

	"github.com/google/uuid"
)

type DocumentTaskPublisher struct {
	service *Service
}

func NewDocumentTaskPublisher(service *Service) *DocumentTaskPublisher {
	return &DocumentTaskPublisher{service: service}
}

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
