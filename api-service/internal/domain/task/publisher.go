package task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// DocumentTaskRoutingKey is the contract routing key for GraphRAG document
// processing (deploy/rabbitmq/ROUTING_KEYS.generated.md).
const DocumentTaskRoutingKey = "document.process"

// DocumentTaskBuilder builds document.process envelopes for the upload
// path. Unlike the task endpoints (which enqueue via Service.Submit), the
// upload path needs the serialized envelope so the document INSERT and the
// outbox INSERT commit in ONE transaction (ADR-008.3) — so this builds and
// returns the envelope instead of publishing.
type DocumentTaskBuilder struct{}

func NewDocumentTaskBuilder() *DocumentTaskBuilder {
	return &DocumentTaskBuilder{}
}

// BuildDocumentTask returns (taskID, routingKey, serialized envelope). The
// envelope is built by the same BuildEnvelope helper Service.Submit uses,
// so both publish paths produce identical envelope shapes.
func (p *DocumentTaskBuilder) BuildDocumentTask(ctx context.Context, documentID, profileID, userID uuid.UUID, storagePath, bucket, fileType string) (string, string, []byte, error) {
	// Field names are the document-processing contract
	// (graph-worker/shared/contracts/MESSAGE_FORMAT.md); graphrag-service
	// requires document_id, storage_path, storage_bucket.
	payload := map[string]interface{}{
		"document_id":    documentID.String(),
		"profile_id":     profileID.String(),
		"user_id":        userID.String(),
		"storage_path":   storagePath,
		"storage_bucket": bucket,
		"file_type":      fileType,
	}

	metadata := map[string]string{
		"source":      "api-service",
		"document_id": documentID.String(),
		"user_id":     userID.String(),
	}

	msg, err := BuildEnvelope(ctx, DocumentTaskRoutingKey, DocumentTaskRoutingKey, payload, metadata)
	if err != nil {
		return "", "", nil, err
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}

	return msg.ID, DocumentTaskRoutingKey, body, nil
}
