package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// expValidYAML is a valid experiment modeled on experiments/exp-02.yaml, with a
// numeric assertion value that must survive the YAML→JSON round-trip.
const expValidYAML = `id: exp-02
title: Golden-path smoke
needs: [compose]
steps:
  - run: make sim-smoke
watch:
  - "Lab Overview → API request rate"
assertions:
  - type: promql
    query: sum(rabbitmq_queue_messages{queue=~".*-processing"})
    op: "<="
    value: 0
    timeout: 60s
  - type: http
    url: http://localhost:8080/ready
    status: 200
    timeout: 30s
cleanup: []
`

// expBrokenYAML fails to parse (invalid YAML structure).
const expBrokenYAML = "id: exp-broken\n\ttitle: bad indent: [unclosed\n"

// expNoIDYAML parses but lacks an id → must be skipped.
const expNoIDYAML = `title: Missing id
needs: [kind]
`

// writeExpRepo lays out a temp repo root with an experiments/ dir holding the
// given files, and returns the repo root.
func writeExpRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "experiments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir experiments: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func expRouter(c *Catalog) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/experiments", c.HandleList)
	mux.HandleFunc("POST /api/experiments/{id}/outcome", c.HandleOutcome)
	return mux
}

func TestCatalogListValidSortedWithFiles(t *testing.T) {
	root := writeExpRepo(t, map[string]string{
		"exp-02.yaml":     expValidYAML,
		"exp-broken.yaml": expBrokenYAML,
		"exp-noid.yaml":   expNoIDYAML,
		"README.md":       "# not yaml",
	})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/experiments")
	if err != nil {
		t.Fatalf("GET experiments: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got []Experiment
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Only the one valid experiment survives; broken and no-id are skipped.
	if len(got) != 1 {
		t.Fatalf("want 1 experiment, got %d: %+v", len(got), got)
	}
	exp := got[0]
	if exp.ID != "exp-02" || exp.Title != "Golden-path smoke" {
		t.Errorf("exp id/title = %q/%q", exp.ID, exp.Title)
	}
	if exp.File != "experiments/exp-02.yaml" {
		t.Errorf("file = %q, want experiments/exp-02.yaml", exp.File)
	}
	if len(exp.Assertions) != 2 {
		t.Fatalf("assertions = %d, want 2", len(exp.Assertions))
	}
	// Numeric value survives as a JSON number (0), not a string.
	if v, ok := exp.Assertions[0].Value.(float64); !ok || v != 0 {
		t.Errorf("assertion[0].value = %#v (%T), want numeric 0", exp.Assertions[0].Value, exp.Assertions[0].Value)
	}
	if exp.Assertions[1].Status != 200 {
		t.Errorf("assertion[1].status = %d, want 200", exp.Assertions[1].Status)
	}
}

// TestCatalogLoadsRealExperiments loads the repo's real experiments/ dir and
// asserts all 12 exp-*.yaml parse cleanly (no skips, no warnings), that the
// http-assertion fields json_path/json_equals are captured, and that "300s"
// timeouts and numeric values survive a JSON round-trip.
func TestCatalogLoadsRealExperiments(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "experiments")); err != nil {
		t.Skipf("real experiments dir not present: %v", err)
	}

	// A buffer logger so any warn/skip line (malformed / missing id-title) fails
	// the test — valid files must load silently.
	var logbuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logbuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	c := NewCatalog(Config{RepoRoot: root}, nil, log)
	got, err := c.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 12 {
		t.Fatalf("loaded %d experiments, want 12", len(got))
	}
	if s := strings.TrimSpace(logbuf.String()); s != "" {
		t.Fatalf("expected no warn/skip lines, got:\n%s", s)
	}

	// Locate exp-01, which uses an http assertion with json_path/json_equals and
	// a promql assertion with a numeric value + "300s" timeout.
	byID := map[string]Experiment{}
	for _, e := range got {
		byID[e.ID] = e
	}
	exp01, ok := byID["exp-01"]
	if !ok {
		t.Fatalf("exp-01 missing from catalog; ids=%v", keysOf(byID))
	}

	var httpA, promqlA *ExpAssertion
	for i := range exp01.Assertions {
		a := &exp01.Assertions[i]
		if a.Type == "http" && a.JSONPath != "" {
			httpA = a
		}
		// Select the count(up == 1) assertion by query, not by timeout: exp-01
		// now carries a second "300s" promql assertion (the consumer-liveness
		// gate added alongside the graphrag cold-start fix), so timeout alone
		// is ambiguous.
		if a.Type == "promql" && a.Query == "count(up == 1)" {
			promqlA = a
		}
	}
	if httpA == nil {
		t.Fatal("exp-01 http assertion with json_path not captured (struct field missing?)")
	}
	if httpA.JSONPath != "status" || httpA.JSONEquals != "ok" {
		t.Errorf("json_path/json_equals = %q/%v, want status/ok", httpA.JSONPath, httpA.JSONEquals)
	}
	if promqlA == nil {
		t.Fatal("exp-01 count(up == 1) promql assertion not found")
	}

	// JSON round-trip: "300s" stays a string, numeric value stays numeric.
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back []Experiment
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var rtHTTP, rtPromql *ExpAssertion
	for i := range back {
		if back[i].ID != "exp-01" {
			continue
		}
		for j := range back[i].Assertions {
			a := &back[i].Assertions[j]
			if a.Type == "http" && a.JSONPath != "" {
				rtHTTP = a
			}
			if a.Type == "promql" && a.Query == "count(up == 1)" {
				rtPromql = a
			}
		}
	}
	if rtHTTP == nil || rtHTTP.JSONEquals != "ok" {
		t.Errorf("json_equals lost in round-trip: %+v", rtHTTP)
	}
	if rtPromql == nil {
		t.Fatal("count(up == 1) promql assertion lost in round-trip")
	}
	if rtPromql.Timeout != "300s" {
		t.Errorf("timeout = %q after round-trip, want \"300s\" (string preserved)", rtPromql.Timeout)
	}
	if v, ok := rtPromql.Value.(float64); !ok || v != 8 {
		t.Errorf("numeric value = %#v (%T) after round-trip, want float64 8", rtPromql.Value, rtPromql.Value)
	}
}

func keysOf(m map[string]Experiment) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCatalogListSorted(t *testing.T) {
	root := writeExpRepo(t, map[string]string{
		"z.yaml": "id: exp-zulu\ntitle: Zulu\n",
		"a.yaml": "id: exp-alpha\ntitle: Alpha\n",
		"m.yaml": "id: exp-mike\ntitle: Mike\n",
	})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	got, err := c.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []string{"exp-alpha", "exp-mike", "exp-zulu"}
	if len(got) != len(want) {
		t.Fatalf("got %d experiments", len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("got[%d].ID = %s, want %s", i, got[i].ID, id)
		}
	}
	// Nil list fields normalized to [] (JSON should carry [], not null).
	if got[0].Needs == nil || got[0].Steps == nil || got[0].Assertions == nil {
		t.Errorf("nil list fields not normalized: %+v", got[0])
	}
}

func postOutcome(t *testing.T, srv *httptest.Server, id, body string) int {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/experiments/"+id+"/outcome", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST outcome: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestOutcomeAppendGolden(t *testing.T) {
	root := writeExpRepo(t, map[string]string{"exp-02.yaml": expValidYAML})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	fixed := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return fixed }
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	// First outcome creates the file (with header) — with notes.
	if s := postOutcome(t, srv, "exp-02", `{"result":"pass","notes":"drained clean"}`); s != http.StatusNoContent {
		t.Fatalf("first outcome status = %d, want 204", s)
	}
	// Second outcome appends — no notes.
	if s := postOutcome(t, srv, "exp-02", `{"result":"fail"}`); s != http.StatusNoContent {
		t.Fatalf("second outcome status = %d, want 204", s)
	}

	path := filepath.Join(root, "documentation", "experiments", "mission-control-outcomes.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outcome log: %v", err)
	}
	want := outcomeLogHeader +
		"## exp-02 — pass — 2026-07-19T12:00:00Z\n" +
		"Golden-path smoke\n" +
		"Session: none\n\n" +
		"> drained clean\n\n" +
		"## exp-02 — fail — 2026-07-19T12:00:00Z\n" +
		"Golden-path smoke\n" +
		"Session: none\n\n" +
		"> (no notes)\n\n"
	if string(data) != want {
		t.Errorf("outcome log mismatch:\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestOutcomeUnknownID404(t *testing.T) {
	root := writeExpRepo(t, map[string]string{"exp-02.yaml": expValidYAML})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	// Well-formed id, but not in the catalog.
	if s := postOutcome(t, srv, "exp-ghost", `{"result":"pass"}`); s != http.StatusNotFound {
		t.Errorf("unknown id status = %d, want 404", s)
	}
	// Malformed id (fails the regex) → 404 too.
	if s := postOutcome(t, srv, "EXP_BAD", `{"result":"pass"}`); s != http.StatusNotFound {
		t.Errorf("malformed id status = %d, want 404", s)
	}
	// No outcome file should have been created.
	if _, err := os.Stat(filepath.Join(root, "documentation", "experiments", "mission-control-outcomes.md")); !os.IsNotExist(err) {
		t.Errorf("outcome file created for unknown id")
	}
}

func TestOutcomeNotesTooLong400(t *testing.T) {
	// Notes land verbatim in a repo-committed markdown file — cap them.
	root := writeExpRepo(t, map[string]string{"exp-02.yaml": expValidYAML})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	huge := strings.Repeat("a", outcomeNotesMax+1)
	if s := postOutcome(t, srv, "exp-02", `{"result":"pass","notes":"`+huge+`"}`); s != http.StatusBadRequest {
		t.Errorf("oversized notes status = %d, want 400", s)
	}
	// Nothing was appended.
	if _, err := os.Stat(filepath.Join(root, "documentation", "experiments", "mission-control-outcomes.md")); !os.IsNotExist(err) {
		t.Errorf("outcome file created for rejected notes")
	}
	// At the cap is still accepted.
	ok := strings.Repeat("a", outcomeNotesMax)
	if s := postOutcome(t, srv, "exp-02", `{"result":"pass","notes":"`+ok+`"}`); s != http.StatusNoContent {
		t.Errorf("at-cap notes status = %d, want 204", s)
	}
}

func TestOutcomeBadResult400(t *testing.T) {
	root := writeExpRepo(t, map[string]string{"exp-02.yaml": expValidYAML})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	if s := postOutcome(t, srv, "exp-02", `{"result":"maybe"}`); s != http.StatusBadRequest {
		t.Errorf("bad result status = %d, want 400", s)
	}
}
