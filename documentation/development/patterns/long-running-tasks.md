# Long-Running Tasks Pattern

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/long-running-tasks.md*

## Overview

This document outlines the patterns and best practices for submitting and tracking long-running tasks via the API Service's task submission endpoints.

## Architecture Context

In the consolidated architecture, the API Service **publishes tasks** directly to RabbitMQ. Tasks are consumed and processed by external workers (not part of this service).

```
API Service → RabbitMQ → Workers (external)
```

## Task Categories

### 1. Image Generation Tasks

#### Characteristics

- Long processing time (30s - 5min)
- High resource usage
- External service dependency
- Result storage required

#### Submission Pattern

```go
type ImageTask struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    ProfileID string                 `json:"profile_id"`
    ImageType string                 `json:"image_type"`
    Params    map[string]interface{} `json:"params"`
    CreatedAt time.Time              `json:"created_at"`
}

func (s *ProfileService) SubmitImageTask(ctx context.Context, profileID string, params ImageTaskParams) (string, error) {
    task := &ImageTask{
        ID:        uuid.New().String(),
        Type:      "image.generation",
        ProfileID: profileID,
        ImageType: params.ImageType,
        Params:    params.Params,
        CreatedAt: time.Now(),
    }

    if err := s.publisher.PublishTask(ctx, task); err != nil {
        return "", fmt.Errorf("failed to submit image task: %w", err)
    }

    return task.ID, nil
}
```

### 2. Email Processing Tasks

#### Characteristics

- Moderate processing time (1-10s)
- External service dependency
- Retry requirements
- Status tracking

#### Submission Pattern

```go
type EmailTask struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    ProfileID string    `json:"profile_id"`
    EmailType string    `json:"email_type"`
    Recipient string    `json:"recipient"`
    Subject   string    `json:"subject"`
    Body      string    `json:"body"`
    CreatedAt time.Time `json:"created_at"`
}

func (s *ProfileService) SubmitEmailTask(ctx context.Context, profileID string, params EmailTaskParams) (string, error) {
    task := &EmailTask{
        ID:        uuid.New().String(),
        Type:      "email.send",
        ProfileID: profileID,
        EmailType: params.EmailType,
        Recipient: params.Recipient,
        Subject:   params.Subject,
        Body:      params.Body,
        CreatedAt: time.Now(),
    }

    if err := s.publisher.PublishTask(ctx, task); err != nil {
        return "", fmt.Errorf("failed to submit email task: %w", err)
    }

    return task.ID, nil
}
```

### 3. Profile Export Tasks

#### Submission Pattern

```go
type ExportTask struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    ProfileID string    `json:"profile_id"`
    Format    string    `json:"format"` // "json", "csv", "pdf"
    CreatedAt time.Time `json:"created_at"`
}

func (s *ProfileService) SubmitExportTask(ctx context.Context, profileID string, format string) (string, error) {
    task := &ExportTask{
        ID:        uuid.New().String(),
        Type:      "profile.export",
        ProfileID: profileID,
        Format:    format,
        CreatedAt: time.Now(),
    }

    if err := s.publisher.PublishTask(ctx, task); err != nil {
        return "", fmt.Errorf("failed to submit export task: %w", err)
    }

    return task.ID, nil
}
```

## Task State Management

### Task States

```go
type TaskStatus string

const (
    TaskStatusPending   TaskStatus = "PENDING"
    TaskStatusRunning   TaskStatus = "RUNNING"
    TaskStatusComplete  TaskStatus = "COMPLETE"
    TaskStatusFailed    TaskStatus = "FAILED"
    TaskStatusCancelled TaskStatus = "CANCELLED"
)

type TaskState struct {
    ID        string     `json:"id"`
    Status    TaskStatus `json:"status"`
    Progress  float64    `json:"progress"`
    StartTime *time.Time `json:"start_time,omitempty"`
    EndTime   *time.Time `json:"end_time,omitempty"`
    Error     string     `json:"error,omitempty"`
    ResultURL string     `json:"result_url,omitempty"`
}
```

### Task Status API

```go
// Handler for task status check
func (h *TaskHandler) GetTaskStatus(c *gin.Context) {
    taskID := c.Param("id")

    // Check cache for task status
    state, err := h.cache.GetTaskState(c.Request.Context(), taskID)
    if err == nil {
        c.JSON(http.StatusOK, state)
        return
    }

    // Check database for persisted state
    state, err = h.repository.GetTaskState(c.Request.Context(), taskID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
        return
    }

    c.JSON(http.StatusOK, state)
}
```

## API Endpoints

### Task Submission Endpoints

```go
// POST /api/v1/profiles/:id/tasks/email
// Submit email task for a profile

// POST /api/v1/profiles/:id/tasks/image
// Submit image generation task for a profile

// POST /api/v1/profiles/:id/tasks/export
// Submit export task for a profile

// GET /api/v1/tasks/:id
// Get task status
```

### Request/Response Examples

```json
// POST /api/v1/profiles/123/tasks/email
// Request:
{
    "email_type": "verification",
    "recipient": "user@example.com"
}

// Response:
{
    "task_id": "task-456",
    "status": "PENDING"
}
```

```json
// GET /api/v1/tasks/task-456
// Response:
{
    "id": "task-456",
    "status": "COMPLETE",
    "progress": 1.0,
    "start_time": "2026-01-29T10:00:00Z",
    "end_time": "2026-01-29T10:00:05Z"
}
```

## Best Practices

### 1. Task Submission

- Validate inputs before publishing
- Generate unique task IDs
- Include correlation data (profile_id)
- Use appropriate task types

### 2. Task Tracking

- Cache task states for quick lookup
- Persist final states to database
- Implement status polling endpoints
- Consider webhooks for completion

### 3. Error Handling

- Return task ID even on publish success
- Handle publish failures gracefully
- Log all task submissions

## Cross-References

- [Queuing Patterns](queuing-patterns.md)
- [Monitoring Patterns](monitoring-patterns.md)
- [API Design Best Practices](../best-practices/api-design-best-practices.md)

## Notes

- API Service only publishes tasks
- Workers process tasks asynchronously
- Task status is updated via cache/database by workers
