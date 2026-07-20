package main

// experiments.go — the scored-experiment catalog + outcome recording
// (v6-HANDOFF §3, wave 2). Two surfaces:
//
//   GET  /api/experiments               -> the parsed catalog, sorted by id
//   POST /api/experiments/{id}/outcome  -> append a structured markdown entry
//
// The catalog is DISPLAY data, not an exec whitelist — the action engine's
// registry whitelist (registry.go) is the only path to the shell, so a
// malformed experiment file is a warn+skip here, never fatal. The directory is
// re-read on every request (cheap; no caching/reload machinery).

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// expIDRe bounds the {id} path segment for outcome recording. It mirrors the
// action engine's experimentIDRe (actions.go) — kept local so this file owns
// its own validation and does not couple to a wave-1 symbol.
var expIDRe = regexp.MustCompile(`^exp-[a-z0-9-]+$`)

// validOutcomeResults is the closed set accepted by the outcome endpoint.
var validOutcomeResults = map[string]bool{"pass": true, "fail": true, "aborted": true}

// outcomeNotesMax caps outcome notes: they are appended verbatim to a
// repo-committed markdown file, so they must stay a write-up, not a dump.
const outcomeNotesMax = 16 << 10 // 16 KiB

// Experiment is one scored-experiment definition (experiments/README.md schema
// v0), enriched with the repo-relative file path for the UI.
type Experiment struct {
	ID         string         `json:"id" yaml:"id"`
	Title      string         `json:"title" yaml:"title"`
	Needs      []string       `json:"needs" yaml:"needs"`
	Steps      []ExpStep      `json:"steps" yaml:"steps"`
	Watch      []string       `json:"watch" yaml:"watch"`
	Assertions []ExpAssertion `json:"assertions" yaml:"assertions"`
	Cleanup    []ExpCleanup   `json:"cleanup" yaml:"cleanup"`
	File       string         `json:"file" yaml:"-"`
}

// ExpStep is one shell step (background=true → fire-and-continue).
type ExpStep struct {
	Run        string `json:"run" yaml:"run"`
	Background bool   `json:"background,omitempty" yaml:"background"`
}

// ExpAssertion is one polled assertion. `value` may be int or float in YAML;
// decoding into `any` keeps it numeric on the way back out to JSON.
type ExpAssertion struct {
	Type string `json:"type" yaml:"type"`
	// promql
	Query string `json:"query,omitempty" yaml:"query"`
	Op    string `json:"op,omitempty" yaml:"op"`
	Value any    `json:"value,omitempty" yaml:"value"`
	// http
	URL        string `json:"url,omitempty" yaml:"url"`
	Status     int    `json:"status,omitempty" yaml:"status"`
	JSONPath   string `json:"json_path,omitempty" yaml:"json_path"`
	JSONEquals any    `json:"json_equals,omitempty" yaml:"json_equals"`
	// cli
	Run string `json:"run,omitempty" yaml:"run"`
	// all assertions
	Timeout string `json:"timeout" yaml:"timeout"`
}

// ExpCleanup is one always-run cleanup step.
type ExpCleanup struct {
	Run string `json:"run" yaml:"run"`
}

// OutcomeRequest is the POST body for outcome recording.
type OutcomeRequest struct {
	Result string `json:"result"`
	Notes  string `json:"notes"`
}

// Catalog serves the experiment catalog and records outcomes. It consults the
// session recorder (which this wave also owns) to stamp the open session id
// onto an outcome and to attach the outcome as a session event.
type Catalog struct {
	cfg      Config
	recorder *Recorder
	log      *slog.Logger

	// now is injectable so outcome-log timestamps are deterministic in tests.
	now func() time.Time

	mu sync.Mutex // serializes outcome-log appends
}

// NewCatalog constructs a Catalog. recorder may be nil (outcomes are then
// recorded to the markdown log only, with no session attachment).
func NewCatalog(cfg Config, recorder *Recorder, log *slog.Logger) *Catalog {
	if log == nil {
		log = slog.Default()
	}
	return &Catalog{cfg: cfg, recorder: recorder, log: log, now: time.Now}
}

// load re-reads <RepoRoot>/experiments/*.yaml on every call, parses each file,
// warn+skips anything malformed or missing id/title, and returns the survivors
// sorted by id.
func (c *Catalog) load() ([]Experiment, error) {
	dir := filepath.Join(c.cfg.RepoRoot, "experiments")
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("experiments: glob %s: %w", dir, err)
	}
	sort.Strings(paths)

	out := make([]Experiment, 0, len(paths))
	for _, p := range paths {
		base := filepath.Base(p)
		if strings.EqualFold(base, "README.md") { // defensive; glob already excludes it
			continue
		}
		exp, ok := parseExperimentFile(p, c.log)
		if !ok {
			continue
		}
		exp.File = filepath.ToSlash(filepath.Join("experiments", base))
		out = append(out, exp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// parseExperimentFile reads and parses one file. A parse failure or a missing
// id/title is logged and reported as a skip (ok=false) — never an error.
func parseExperimentFile(path string, log *slog.Logger) (Experiment, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Warn("experiment file unreadable — skipping", "file", path, "error", err.Error())
		return Experiment{}, false
	}
	var exp Experiment
	if err := yaml.Unmarshal(data, &exp); err != nil {
		log.Warn("experiment file failed to parse — skipping", "file", path, "error", err.Error())
		return Experiment{}, false
	}
	if strings.TrimSpace(exp.ID) == "" || strings.TrimSpace(exp.Title) == "" {
		log.Warn("experiment file missing id/title — skipping", "file", path)
		return Experiment{}, false
	}
	normalizeExperiment(&exp)
	return exp, true
}

// normalizeExperiment replaces nil slices with empty ones so the JSON output
// carries [] rather than null for the always-present list fields.
func normalizeExperiment(exp *Experiment) {
	if exp.Needs == nil {
		exp.Needs = []string{}
	}
	if exp.Steps == nil {
		exp.Steps = []ExpStep{}
	}
	if exp.Watch == nil {
		exp.Watch = []string{}
	}
	if exp.Assertions == nil {
		exp.Assertions = []ExpAssertion{}
	}
	if exp.Cleanup == nil {
		exp.Cleanup = []ExpCleanup{}
	}
}

// HandleList serves GET /api/experiments → JSON array sorted by id.
func (c *Catalog) HandleList(w http.ResponseWriter, r *http.Request) {
	exps, err := c.load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exps)
}

// HandleOutcome serves POST /api/experiments/{id}/outcome → 204.
func (c *Catalog) HandleOutcome(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !expIDRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "no such experiment: "+id)
		return
	}
	var body OutcomeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if !validOutcomeResults[body.Result] {
		writeError(w, http.StatusBadRequest, `result must be one of "pass", "fail", "aborted"`)
		return
	}
	if len(body.Notes) > outcomeNotesMax {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("notes too long (%d bytes, max %d)", len(body.Notes), outcomeNotesMax))
		return
	}

	// The id must exist in the catalog (display data, but a recorded outcome
	// should name a real experiment).
	exps, err := c.load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var title string
	found := false
	for _, e := range exps {
		if e.ID == id {
			title, found = e.Title, true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "no such experiment: "+id)
		return
	}

	// Stamp the open session id (if any) and attach the outcome as an event.
	session := "none"
	if c.recorder != nil {
		if sv, ok := c.recorder.Current(); ok {
			session = sv.ID
		}
		c.recorder.RecordOutcome(id, body.Result, body.Notes)
	}

	if err := c.appendOutcome(id, title, body.Result, body.Notes, session); err != nil {
		c.log.Error("outcome append failed", "id", id, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "failed to record outcome: "+err.Error())
		return
	}
	c.log.Info("experiment outcome recorded", "id", id, "result", body.Result, "session", session)
	w.WriteHeader(http.StatusNoContent)
}

// outcomeLogHeader is written once, when the log file is first created.
const outcomeLogHeader = "<!-- Outcome log appended by Mission Control (v6). One entry per recorded run. -->\n\n"

// appendOutcome appends one structured markdown entry to
// <RepoRoot>/documentation/experiments/mission-control-outcomes.md, creating
// the file (with its header) on first write. Append-only; never rewrites.
func (c *Catalog) appendOutcome(id, title, result, notes, session string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dir := filepath.Join(c.cfg.RepoRoot, "documentation", "experiments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "mission-control-outcomes.md")

	var buf strings.Builder
	if _, err := os.Stat(path); os.IsNotExist(err) {
		buf.WriteString(outcomeLogHeader)
	}
	buf.WriteString(outcomeEntryMarkdown(id, title, result, notes, session, c.now().UTC()))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(buf.String()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// outcomeEntryMarkdown renders one outcome-log entry.
func outcomeEntryMarkdown(id, title, result, notes, session string, ts time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s — %s — %s\n", id, result, ts.Format(time.RFC3339))
	fmt.Fprintf(&b, "%s\n", title)
	fmt.Fprintf(&b, "Session: %s\n\n", session)
	if strings.TrimSpace(notes) == "" {
		b.WriteString("> (no notes)\n\n")
	} else {
		for _, line := range strings.Split(strings.TrimRight(notes, "\n"), "\n") {
			fmt.Fprintf(&b, "> %s\n", line)
		}
		b.WriteString("\n")
	}
	return b.String()
}
