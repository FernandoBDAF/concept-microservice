package main

// lab-controld v1 control plane (ADR-005.2) — v6 action surface.
//
// v3 shipped the read-only sliver (main.go). v6 adds ACTIONS: launch/stop/
// scale/experiment-run, each executed as a command drawn from the systems
// registry (systems/*.yaml — the registry doubles as the action whitelist;
// nothing else is ever exec'd). The types below are the FINAL contract;
// their bodies live across registry.go / engine.go / sse.go / store.go /
// auth.go. documentation/phases/v6-HANDOFF.md §2-3 sequences the work.
//
// API contract (wired into main.go's mux by the orchestrator — see
// mission-control/README.md):
//   GET  /api/systems                 -> []System (registry, parsed, by name)
//   POST /api/actions                 -> start an Action; 202 {id, command}
//   GET  /api/actions/{id}            -> ActionRecord (state + exit code)
//   GET  /api/actions/{id}/stream     -> SSE: stdout/stderr lines as events
//   GET  /api/runs                    -> run history (JSONL on disk, no DB)
//
// Security (ADR-005.4): localhost binding stays the default no-auth mode;
// enabling the aws target requires CONTROLD_TOKEN + TLS (HANDOFF §5) —
// requests must carry Authorization: Bearer <token>; wrong token → 401 +
// audit log line. EXP-63 asserts both properties.

import (
	"regexp"
	"strconv"
	"strings"
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
	System string            `json:"system"`           // registry name
	Target string            `json:"target"`           // compose|kind|aws
	Verb   string            `json:"verb"`             // up|down|status|scale|experiment
	Params map[string]string `json:"params,omitempty"` // scale: component,n; experiment: id
}

// ActionRecord is what history persists (JSONL, one file per day under
// mission-control/controld/runs/ — gitignored) and what the UI polls.
type ActionRecord struct {
	ID        string        `json:"id"`
	Request   ActionRequest `json:"request"`
	Command   string        `json:"command"` // the exact invocation (teaching surface)
	State     string        `json:"state"`   // pending|running|succeeded|failed
	ExitCode  *int          `json:"exit_code,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`

	// Report is the parsed scored-run report for experiment verbs (nil for
	// other verbs, or when the runner emitted no parseable XML). Its presence
	// never changes the action's exit-code-driven pass/fail — it is an
	// enrichment attached in the engine just before finalize (report.go).
	Report *ExperimentReport `json:"report,omitempty"`
}

// experimentIDRe bounds the only free-form value that ever reaches the shell
// besides the validated integer {n}: the experiment id.
var experimentIDRe = regexp.MustCompile(`^exp-[a-z0-9-]+$`)

// resolveCommand maps a validated request to the registry command — the ONLY
// path from HTTP input to exec. It consults the registry (and the aws gate)
// ONLY; no Params value reaches the shell except the validated integer {n}
// and the regex-checked experiment id.
//
// NOTE: the free-function signature is extended from the original stub to take
// the loaded registry and config it must consult (the stub had no state to
// read). Callers: engine.StartAction and the resolve tests.
func resolveCommand(reg *Registry, cfg Config, req ActionRequest) (string, error) {
	sys, ok := reg.System(req.System)
	if !ok {
		return "", apiErr(404, "unknown system: "+req.System)
	}

	// aws target is gated globally: disabled → 403 regardless of verb or
	// whether this system even declares an aws target.
	if req.Target == "aws" && !cfg.EnableAWS {
		return "", apiErr(403, "aws target is disabled — start controld with CONTROLD_ENABLE_AWS=1 (requires token + TLS)")
	}

	switch req.Verb {
	case "up", "down", "status":
		tc, ok := sys.Targets[req.Target]
		if !ok {
			return "", apiErr(404, "target not available for system "+req.System+": "+req.Target)
		}
		if req.Verb == "down" && req.Params["confirm"] != "true" {
			return "", apiErr(400, `destructive verb "down" requires params.confirm="true"`)
		}
		switch req.Verb {
		case "up":
			return tc.Up, nil
		case "down":
			return tc.Down, nil
		default:
			return tc.Status, nil
		}

	case "scale":
		if _, ok := sys.Targets[req.Target]; !ok {
			return "", apiErr(404, "target not available for system "+req.System+": "+req.Target)
		}
		comp := req.Params["component"]
		spec, ok := findScale(sys, comp)
		if !ok {
			return "", apiErr(400, "unknown scale component for system "+req.System+": "+comp)
		}
		tmpl := scaleTemplate(spec, req.Target)
		if tmpl == "" {
			return "", apiErr(400, "component "+comp+" is not scalable on target "+req.Target)
		}
		n, err := parseScaleN(req.Params["n"])
		if err != nil {
			return "", apiErr(400, err.Error())
		}
		return strings.ReplaceAll(tmpl, "{n}", strconv.Itoa(n)), nil

	case "experiment":
		if _, ok := sys.Targets[req.Target]; !ok {
			return "", apiErr(404, "target not available for system "+req.System+": "+req.Target)
		}
		if sys.Experiments == "" {
			return "", apiErr(400, "system "+req.System+" declares no experiments")
		}
		id := req.Params["id"]
		if !experimentIDRe.MatchString(id) {
			return "", apiErr(400, `invalid experiment id (must match ^exp-[a-z0-9-]+$): `+strconv.Quote(id))
		}
		return "make experiment E=" + id, nil

	default:
		return "", apiErr(400, "unknown verb: "+req.Verb)
	}
}

// findScale returns the ScaleSpec for a component within a system.
func findScale(sys System, component string) (ScaleSpec, bool) {
	if component == "" {
		return ScaleSpec{}, false
	}
	for _, s := range sys.Scale {
		if s.Component == component {
			return s, true
		}
	}
	return ScaleSpec{}, false
}

// scaleTemplate returns the per-target scale command template ("" if none).
// aws has no scale template field in the schema, so aws scale is never valid.
func scaleTemplate(spec ScaleSpec, target string) string {
	switch target {
	case "compose":
		return spec.Compose
	case "kind":
		return spec.Kind
	default:
		return ""
	}
}

// parseScaleN enforces a strict integer 1..10 — the {n} placeholder is one of
// only two Params values allowed to reach the shell.
func parseScaleN(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, errScaleN(s)
	}
	if n < 1 || n > 10 {
		return 0, errScaleN(s)
	}
	return n, nil
}

func errScaleN(s string) error {
	return apiErr(400, "scale param n must be an integer 1..10, got "+strconv.Quote(s))
}
