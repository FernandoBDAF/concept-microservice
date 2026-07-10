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
