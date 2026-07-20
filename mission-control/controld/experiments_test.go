package main

import (
	"encoding/json"
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

func TestOutcomeBadResult400(t *testing.T) {
	root := writeExpRepo(t, map[string]string{"exp-02.yaml": expValidYAML})
	c := NewCatalog(Config{RepoRoot: root}, nil, quietLogger())
	srv := httptest.NewServer(expRouter(c))
	t.Cleanup(srv.Close)

	if s := postOutcome(t, srv, "exp-02", `{"result":"maybe"}`); s != http.StatusBadRequest {
		t.Errorf("bad result status = %d, want 400", s)
	}
}
