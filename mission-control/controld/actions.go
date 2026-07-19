package main

// lab-controld v1 control plane (ADR-005.2) — SKELETON for the v6 handoff.
//
// v3 shipped the read-only sliver (main.go). v6 adds ACTIONS: launch/stop/
// scale/experiment-run, each executed as a `make` invocation from the
// systems registry (systems/*.yaml — the registry doubles as the action
// whitelist; nothing else is ever exec'd). Types and API contract below are
// final; documentation/phases/v6-HANDOFF.md §2-3 sequences the work.
//
// API contract (add to main.go route table when implementing):
//   GET  /api/systems                 -> []System (registry, parsed)
//   POST /api/actions                 -> start an Action; 202 {id}
//   GET  /api/actions/{id}            -> ActionRecord (state + exit code)
//   GET  /api/actions/{id}/stream     -> SSE: stdout/stderr lines as events
//                                        (SSE chosen over WebSocket: one-way
//                                        stream, EventSource is enough)
//   GET  /api/runs                    -> run history (JSONL on disk, no DB)
//   POST /api/sessions, PATCH .../{id}, GET .../{id}/summary
//                                     -> session recorder (HANDOFF §6)
//
// Security (ADR-005.4): localhost binding stays the default no-auth mode;
// enabling the aws target requires CONTROLD_TOKEN + TLS (HANDOFF §5) —
// requests must carry Authorization: Bearer <token>; wrong token → 401 +
// audit log line. EXP-63 asserts both properties.

import (
	"encoding/json"
	"net/http"
	"time"
)

// System mirrors systems/README.md schema v0.
type System struct {
	Name        string                       `json:"name" yaml:"name"`
	Description string                       `json:"description" yaml:"description"`
	PortBlock   string                       `json:"port_block" yaml:"port_block"`
	Targets     map[string]SystemTargetCmds  `json:"targets" yaml:"targets"`
	Scale       []ScaleSpec                  `json:"scale,omitempty" yaml:"scale"`
	Links       map[string]map[string]string `json:"links,omitempty" yaml:"links"`
	Experiments string                       `json:"experiments,omitempty" yaml:"experiments"`
}

type SystemTargetCmds struct {
	Up     string `json:"up" yaml:"up"`
	Down   string `json:"down" yaml:"down"`
	Status string `json:"status" yaml:"status"`
}

type ScaleSpec struct {
	Component string `json:"component" yaml:"component"`
	Compose   string `json:"compose,omitempty" yaml:"compose"`
	Kind      string `json:"kind,omitempty" yaml:"kind"`
}

// ActionRequest is the whole write surface of Mission Control.
type ActionRequest struct {
	System string `json:"system"`           // registry name
	Target string `json:"target"`           // compose|kind|aws
	Verb   string `json:"verb"`             // up|down|status|scale|experiment
	Params map[string]string `json:"params,omitempty"` // scale: component,n; experiment: id
}

// ActionRecord is what history persists (JSONL, one file per day under
// mission-control/controld/runs/ — gitignored) and what the UI polls.
type ActionRecord struct {
	ID        string    `json:"id"`
	Request   ActionRequest `json:"request"`
	Command   string    `json:"command"` // the exact make invocation (teaching surface)
	State     string    `json:"state"`   // pending|running|succeeded|failed
	ExitCode  *int      `json:"exit_code,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// resolveCommand maps a validated request to the registry command —
// the ONLY path from HTTP input to exec. TODO(v6, HANDOFF §2): load the
// registry (gopkg.in/yaml.v3), validate system/target/verb existence,
// fill {n}-placeholders from Params after strict validation
// (n: 1-10 integer; experiment id: ^exp-[a-z0-9-]+$), destructive verbs
// (down, nuke-anything) get a `confirm: true` param requirement.
func resolveCommand(req ActionRequest) (string, error) {
	_ = req
	return "", errNotImplemented
}

// handleActions is wired into main.go's mux by the v6 implementation
// (POST → spawn; the exec loop: exec.CommandContext("sh","-c",cmd) from
// repo root, line-scan stdout+stderr into a bounded ring buffer + SSE
// fanout + JSONL append on completion).
func handleActions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "actions land in phase v6 — documentation/phases/v6-HANDOFF.md",
	})
	_ = r
}

var errNotImplemented = &notImplementedError{}

type notImplementedError struct{}

func (*notImplementedError) Error() string { return "not implemented (v6 handoff)" }
