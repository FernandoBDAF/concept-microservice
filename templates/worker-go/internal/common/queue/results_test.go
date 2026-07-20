package queue

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestClassifyOutcome is the source-of-truth for the routing/result decision:
// success → completed, a retryable error with a tier left → retry (NO result),
// unretryable or exhausted → DLQ (failed result).
func TestClassifyOutcome(t *testing.T) {
	retryable := errors.New("transient dependency failure")
	unretryable := errors.Join(ErrUnretryable, errors.New("bad payload"))

	cases := []struct {
		name    string
		err     error
		attempt int
		want    outcome
	}{
		{"success", nil, 0, outcomeSuccess},
		{"unretryable first attempt", unretryable, 0, outcomeDLQ},
		{"retryable attempt 0", retryable, 0, outcomeRetry},
		{"retryable attempt 1", retryable, 1, outcomeRetry},
		{"retryable attempt 2", retryable, 2, outcomeRetry},
		{"retryable exhausted attempt 3", retryable, 3, outcomeDLQ},
		{"retryable exhausted attempt 4", retryable, 4, outcomeDLQ},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyOutcome(c.err, c.attempt); got != c.want {
				t.Errorf("classifyOutcome() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestOutcomeResultStatus(t *testing.T) {
	if s, emit := outcomeSuccess.resultStatus(); !emit || s != statusCompleted {
		t.Errorf("success.resultStatus() = (%q,%v), want (completed,true)", s, emit)
	}
	if _, emit := outcomeRetry.resultStatus(); emit {
		t.Error("retry.resultStatus() must NOT emit a result (scheduled retry)")
	}
	if s, emit := outcomeDLQ.resultStatus(); !emit || s != statusFailed {
		t.Errorf("DLQ.resultStatus() = (%q,%v), want (failed,true)", s, emit)
	}
}

func decodeResult(t *testing.T, body []byte) taskResultEnvelope {
	t.Helper()
	var env taskResultEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return env
}

func TestBuildTaskResult_Completed(t *testing.T) {
	msg := &Message{ID: "env-1", Type: "email.send", Payload: json.RawMessage(`{"email_type":"welcome","recipient":"a@b.c"}`)}

	body, err := buildTaskResult("email", msg, statusCompleted, "")
	if err != nil {
		t.Fatalf("buildTaskResult: %v", err)
	}
	env := decodeResult(t, body)

	if env.Type != "task.result" {
		t.Errorf("Type = %q, want task.result", env.Type)
	}
	if env.ID == "" || env.ID == msg.ID {
		t.Errorf("result id must be a fresh uuid distinct from the task id, got %q", env.ID)
	}
	if env.Payload.Status != statusCompleted {
		t.Errorf("status = %q, want completed", env.Payload.Status)
	}
	if env.Payload.EnvelopeID != "env-1" || env.Payload.TaskID != "env-1" {
		t.Errorf("envelope_id/task_id = %q/%q, want env-1/env-1", env.Payload.EnvelopeID, env.Payload.TaskID)
	}
	if env.Payload.TaskType != "email.send" {
		t.Errorf("task_type = %q, want email.send", env.Payload.TaskType)
	}
	if env.Payload.Error != "" {
		t.Errorf("completed result must carry no error, got %q", env.Payload.Error)
	}
	if env.Payload.DocumentID != "" {
		t.Errorf("email payload has no document_id, got %q", env.Payload.DocumentID)
	}
	if env.Metadata["source"] != "email-worker" {
		t.Errorf("source = %q, want email-worker", env.Metadata["source"])
	}
	if _, perr := time.Parse(time.RFC3339, env.Timestamp); perr != nil {
		t.Errorf("timestamp %q not RFC3339: %v", env.Timestamp, perr)
	}
	if !strings.HasSuffix(env.Timestamp, "Z") {
		t.Errorf("timestamp %q must be UTC (trailing Z)", env.Timestamp)
	}
}

func TestBuildTaskResult_FailedWithDocumentID(t *testing.T) {
	msg := &Message{ID: "env-2", Type: "document.process", Payload: json.RawMessage(`{"document_id":"doc-9","storage_path":"x"}`)}

	body, err := buildTaskResult("profile", msg, statusFailed, "boom: exhausted tiers")
	if err != nil {
		t.Fatalf("buildTaskResult: %v", err)
	}
	env := decodeResult(t, body)

	if env.Payload.Status != statusFailed {
		t.Errorf("status = %q, want failed", env.Payload.Status)
	}
	if env.Payload.Error != "boom: exhausted tiers" {
		t.Errorf("error = %q, want the failure reason", env.Payload.Error)
	}
	if env.Payload.DocumentID != "doc-9" {
		t.Errorf("document_id = %q, want doc-9 (document.process payload carries one)", env.Payload.DocumentID)
	}
	if env.Metadata["source"] != "profile-worker" {
		t.Errorf("source = %q, want profile-worker", env.Metadata["source"])
	}
}
