package task

import "context"

// ============================================================================
// EDIT THIS FILE to define your task types. It is the single adapt point.
//
// This template ships ONE example task type, "example.task". To add a real
// task type:
//   1. Add its routing key + broker resources to DefaultRoutingMap below.
//   2. (Optional) Declare a typed payload struct like ExamplePayload.
//   3. (Optional) Add a typed Submit helper like SubmitExample so callers
//      get a compile-checked API instead of a stringly-typed one.
// Nothing else in the module needs to change: BuildEnvelope, the outbox, the
// relay, and the HTTP handler are all driven by this map.
// ============================================================================

// ExampleTaskRoutingKey is the routing key for the example task type.
const ExampleTaskRoutingKey = "example.task"

// ExamplePayload is the typed body of an example.task envelope. Replace its
// fields with your task's real contract (keep everything JSON-serializable;
// consumers must tolerate extra fields for forward compatibility).
type ExamplePayload struct {
	ResourceID string `json:"resource_id"`
	Note       string `json:"note,omitempty"`
}

// DefaultRoutingMap is the publish-side lookup: routing key -> the broker
// resources that ALREADY exist for it. There is intentionally no fallback —
// an unknown routing key fails fast in BuildEnvelope (a publisher bug, not a
// parking lot). Names here MUST match your broker topology (CONTRACTS.md §2).
var DefaultRoutingMap = map[string]RoutingConfig{
	ExampleTaskRoutingKey: {
		Exchange:    "example-tasks",
		Queue:       "example-processing",
		Prefetch:    1,
		Description: "Example processing tasks",
	},
}

// SubmitExample is the typed convenience wrapper over Service.Submit for the
// example task type. Copy this shape for each real task type you add — it
// keeps the type and the routing key in lock-step and gives callers a
// compile-checked payload.
func (s *Service) SubmitExample(ctx context.Context, payload ExamplePayload, metadata map[string]string) (string, error) {
	return s.Submit(ctx, ExampleTaskRoutingKey, ExampleTaskRoutingKey, payload, metadata)
}
