package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"example.com/api-publisher/internal/task"
)

// TaskHandler serves POST /tasks: it validates the routing key against the
// contract whitelist and submits through the outbox-backed task service.
type TaskHandler struct {
	service *task.Service
}

func NewTaskHandler(service *task.Service) *TaskHandler {
	return &TaskHandler{service: service}
}

// TaskRequest is the POST /tasks body. routing_key and type must both be one
// of the contract task types (see internal/task/tasktypes.go).
type TaskRequest struct {
	RoutingKey string                 `json:"routing_key"`
	Type       string                 `json:"type"`
	Payload    map[string]interface{} `json:"payload"`
	Metadata   map[string]string      `json:"metadata"`
}

// SubmitTask handles POST /tasks. On success it returns 202 with the envelope
// id; the outbox relay publishes the envelope asynchronously.
func (h *TaskHandler) SubmitTask(w http.ResponseWriter, r *http.Request) {
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RoutingKey == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "routing_key and type are required")
		return
	}

	// The endpoint accepts ONLY the contract task types — there is no parking
	// lot. Unknown types are a client bug and 400 immediately.
	if !task.IsContractTaskType(req.RoutingKey) || !task.IsContractTaskType(req.Type) {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":         "unknown task type: routing_key and type must be one of the contract task types",
			"allowed_types": contractTaskTypes(),
		})
		return
	}

	if req.Payload == nil {
		req.Payload = map[string]interface{}{}
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	if uid := UserID(r.Context()); uid != "" {
		metadata["user_id"] = uid
	}

	taskID, err := h.service.Submit(r.Context(), req.RoutingKey, req.Type, req.Payload, metadata)
	if err != nil {
		// Defense in depth: the envelope builder also rejects non-contract
		// keys, and that is a client error, not a server one.
		if errors.Is(err, task.ErrUnknownRoutingKey) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"task_id": taskID})
}

// contractTaskTypes returns the whitelist in stable order for error bodies.
func contractTaskTypes() string {
	types := make([]string, 0, len(task.DefaultRoutingMap))
	for rk := range task.DefaultRoutingMap {
		types = append(types, rk)
	}
	sort.Strings(types)
	return strings.Join(types, ", ")
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
