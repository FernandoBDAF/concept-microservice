package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubRuns is a runsSource returning a fixed set of records — lets the summary
// test assert a stubbed action row without touching disk or the exec engine.
type stubRuns struct{ recs []ActionRecord }

func (s stubRuns) Runs(limit int) ([]ActionRecord, error) { return s.recs, nil }

func sessionRouter(r *Recorder) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sessions", r.HandleCreate)
	mux.HandleFunc("GET /api/sessions/current", r.HandleCurrent)
	mux.HandleFunc("PATCH /api/sessions/{id}", r.HandlePatch)
	mux.HandleFunc("GET /api/sessions/{id}/summary", r.HandleSummary)
	return mux
}

func doReq(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	var rdr *bytes.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	resp.Body.Close()
	return resp, buf.String()
}

func TestSessionLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runs")
	// A controlled clock so the session window deterministically contains the
	// stubbed action. Guarded by a mutex since handlers run in server goroutines.
	base := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clk := base
	now := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return clk }
	setClock := func(tm time.Time) { clockMu.Lock(); clk = tm; clockMu.Unlock() }

	// A stubbed action started inside the [open, close] window.
	ended := base.Add(31 * time.Minute)
	exit := 0
	action := ActionRecord{
		ID:        "act01",
		Request:   ActionRequest{System: "lab", Target: "compose", Verb: "up"},
		Command:   "make up",
		State:     "succeeded",
		ExitCode:  &exit,
		StartedAt: base.Add(30 * time.Minute),
		EndedAt:   &ended,
	}
	r := NewRecorder(dir, stubRuns{recs: []ActionRecord{action}}, quietLogger())
	r.now = now
	srv := httptest.NewServer(sessionRouter(r))
	t.Cleanup(srv.Close)

	// Create → 201.
	resp, body := doReq(t, http.MethodPost, srv.URL+"/api/sessions", `{"title":"Burst practice"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", resp.StatusCode, body)
	}
	var sv SessionView
	if err := json.Unmarshal([]byte(body), &sv); err != nil {
		t.Fatalf("decode created session: %v", err)
	}
	if sv.ID == "" || len(sv.ID) != 16 || sv.Title != "Burst practice" || sv.StartedAt.IsZero() {
		t.Fatalf("created session = %+v", sv)
	}
	id := sv.ID

	// Current → 200.
	resp, body = doReq(t, http.MethodGet, srv.URL+"/api/sessions/current", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("current status = %d", resp.StatusCode)
	}
	var cur SessionView
	json.Unmarshal([]byte(body), &cur)
	if cur.ID != id {
		t.Errorf("current id = %s, want %s", cur.ID, id)
	}

	// Second create → 409 with open_id.
	resp, body = doReq(t, http.MethodPost, srv.URL+"/api/sessions", `{"title":"Another"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409", resp.StatusCode)
	}
	var conflict map[string]string
	json.Unmarshal([]byte(body), &conflict)
	if conflict["open_id"] != id {
		t.Errorf("409 open_id = %q, want %q", conflict["open_id"], id)
	}

	// Record an experiment outcome (attaches to the open session).
	r.RecordOutcome("exp-02", "pass", "queues drained")

	// Add a note via PATCH.
	resp, _ = doReq(t, http.MethodPatch, srv.URL+"/api/sessions/"+id, `{"note":"looked healthy"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("note patch status = %d", resp.StatusCode)
	}

	// Advance the clock past the action, then close via PATCH so the window
	// [open, close] contains the stubbed action's StartedAt.
	setClock(base.Add(1 * time.Hour))
	resp, body = doReq(t, http.MethodPatch, srv.URL+"/api/sessions/"+id, `{"close":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("close status = %d", resp.StatusCode)
	}
	json.Unmarshal([]byte(body), &sv)
	if sv.EndedAt == nil {
		t.Errorf("closed session has no ended_at")
	}

	// Second close → 409.
	resp, _ = doReq(t, http.MethodPatch, srv.URL+"/api/sessions/"+id, `{"close":true}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second close status = %d, want 409", resp.StatusCode)
	}

	// Current → 404 (nothing open now).
	resp, _ = doReq(t, http.MethodGet, srv.URL+"/api/sessions/current", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("current after close status = %d, want 404", resp.StatusCode)
	}

	// Summary → contains title, the stubbed action row, the outcome, and the note.
	resp, body = doReq(t, http.MethodGet, srv.URL+"/api/sessions/"+id+"/summary", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("summary status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("summary content-type = %q", ct)
	}
	for _, want := range []string{
		"# Burst practice",
		"make up",        // the exact command of the stubbed action
		"lab/compose/up", // system/target/verb
		"exit 0",         // action exit code
		"exp-02",         // the outcome
		"queues drained", // the outcome notes
		"looked healthy", // the note
		"phase v6",       // footer
	} {
		if !strings.Contains(body, want) {
			t.Errorf("summary missing %q\n---\n%s", want, body)
		}
	}

	// Persistence: sessions.jsonl exists with the event lines.
	if data, err := readFileTrim(filepath.Join(dir, "sessions.jsonl")); err != nil {
		t.Errorf("sessions.jsonl: %v", err)
	} else {
		for _, kind := range []string{"opened", "outcome", "note", "closed"} {
			if !strings.Contains(data, `"kind":"`+kind+`"`) {
				t.Errorf("sessions.jsonl missing kind %q", kind)
			}
		}
	}
}

func TestSessionPatchUnknownID404(t *testing.T) {
	r := NewRecorder(filepath.Join(t.TempDir(), "runs"), nil, quietLogger())
	srv := httptest.NewServer(sessionRouter(r))
	t.Cleanup(srv.Close)

	resp, _ := doReq(t, http.MethodPatch, srv.URL+"/api/sessions/deadbeefdeadbeef", `{"note":"x"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("patch unknown id status = %d, want 404", resp.StatusCode)
	}
}

func TestSessionSummaryUnknownID404(t *testing.T) {
	r := NewRecorder(filepath.Join(t.TempDir(), "runs"), nil, quietLogger())
	srv := httptest.NewServer(sessionRouter(r))
	t.Cleanup(srv.Close)

	resp, _ := doReq(t, http.MethodGet, srv.URL+"/api/sessions/deadbeefdeadbeef/summary", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("summary unknown id status = %d, want 404", resp.StatusCode)
	}
}

func TestSessionTitleRequired(t *testing.T) {
	r := NewRecorder(filepath.Join(t.TempDir(), "runs"), nil, quietLogger())
	srv := httptest.NewServer(sessionRouter(r))
	t.Cleanup(srv.Close)

	resp, _ := doReq(t, http.MethodPost, srv.URL+"/api/sessions", `{"title":"   "}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty title status = %d, want 400", resp.StatusCode)
	}
	resp, _ = doReq(t, http.MethodPost, srv.URL+"/api/sessions", `{"title":"`+strings.Repeat("x", 201)+`"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("too-long title status = %d, want 400", resp.StatusCode)
	}
}

func readFileTrim(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
