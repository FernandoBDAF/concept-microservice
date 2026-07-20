# Message Format

## Envelope
All messages must use this standard envelope:

```json
{
  "id": "uuid",
  "type": "task.type",
  "timestamp": "2026-01-30T12:34:56Z",
  "payload": {},
  "metadata": {
    "source": "api-service",
    "trace_id": "..."
  }
}
```

- `id`: Unique message identifier (UUID)
- `type`: Task type or routing key (e.g., `document.process`)
- `timestamp`: ISO-8601 timestamp (UTC)
- `payload`: Task-specific body
- `metadata`: Optional tracing and context fields

## Document Processing Payload

```json
{
  "document_id": "doc-123",
  "storage_bucket": "documents",
  "storage_path": "uploads/2026/01/30/doc-123.pdf",
  "file_type": "pdf",
  "user_id": "user-456"
}
```

## Email Payload

```json
{
  "email_type": "welcome",
  "recipient": "user@example.com",
  "subject": "Welcome",
  "template_id": "welcome-template",
  "variables": {
    "first_name": "Ada"
  }
}
```

## Image Payload

```json
{
  "operation": "resize",
  "source_url": "s3://bucket/path/image.png",
  "target_path": "processed/image.png",
  "width": 512,
  "height": 512,
  "quality": 85,
  "format": "png"
}
```

## Profile Payload

```json
{
  "task_type": "sync",
  "profile_id": "profile-789",
  "user_id": "user-456",
  "data": {
    "source": "external-system"
  }
}
```

## Task Result (`task.result`)

Published by workers and graphrag-service after ack-worthy processing to
exchange `task-results` (routing key `task.result`); consumed by
api-service to advance document status (ADR-008.3). Standard envelope, with:

```json
{
  "id": "<new uuid>",
  "type": "task.result",
  "timestamp": "2026-01-30T12:35:10Z",
  "payload": {
    "task_id": "id of the processed task",
    "task_type": "document.process",
    "status": "completed",
    "error": "only present when status is failed",
    "envelope_id": "id of the originating task envelope",
    "document_id": "doc-123"
  },
  "metadata": {
    "source": "graphrag-service"
  }
}
```

- `id`: NEW UUID for the result message itself (not the task's id)
- `payload.status`: `"completed"` or `"failed"`
- `payload.error`: failure reason; only when `status` is `"failed"`
- `payload.envelope_id`: `id` of the originating task envelope (dedupe key —
  relay re-publishes are tolerated by design, consumers dedupe on it)
- `payload.document_id`: only for document-processing results; api-service
  uses it to advance the document `processing → completed/failed`
- `metadata.source`: producing service (e.g. `email-worker`,
  `graphrag-service`)
