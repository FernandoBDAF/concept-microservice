package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- test harness ---------------------------------------------------------

func execEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	reg, err := LoadRegistry("testdata/exec", quietLogger())
	if err != nil {
		t.Fatalf("load exec registry: %v", err)
	}
	if cfg.RepoRoot == "" {
		cfg.RepoRoot = t.TempDir()
	}
	store := NewStore(filepath.Join(t.TempDir(), "runs"), quietLogger())
	return NewEngine(cfg, reg, store, quietLogger())
}

func testRouter(e *Engine, cfg Config, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/systems", e.HandleSystems)
	mux.HandleFunc("POST /api/actions", e.HandleCreateAction)
	mux.HandleFunc("GET /api/actions/{id}", e.HandleGetAction)
	mux.HandleFunc("GET /api/actions/{id}/stream", e.HandleStreamAction)
	mux.HandleFunc("GET /api/runs", e.HandleRuns)
	return AuthMiddleware(cfg, log)(mux)
}

func newTestServer(t *testing.T, e *Engine, cfg Config, log *slog.Logger) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(testRouter(e, cfg, log))
	t.Cleanup(srv.Close)
	return srv
}

func postAction(t *testing.T, srv *httptest.Server, token string, req ActionRequest) (int, map[string]string) {
	t.Helper()
	body, _ := json.Marshal(req)
	hreq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/actions", bytes.NewReader(body))
	if token != "" {
		hreq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		t.Fatalf("POST /api/actions: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func getAction(t *testing.T, srv *httptest.Server, id, token string) ActionRecord {
	t.Helper()
	hreq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/actions/"+id, nil)
	if token != "" {
		hreq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		t.Fatalf("GET action: %v", err)
	}
	defer resp.Body.Close()
	var rec ActionRecord
	_ = json.NewDecoder(resp.Body).Decode(&rec)
	return rec
}

func waitTerminal(t *testing.T, srv *httptest.Server, id, token string) ActionRecord {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		rec := getAction(t, srv, id, token)
		if rec.State == "succeeded" || rec.State == "failed" {
			return rec
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("action %s did not reach a terminal state", id)
	return ActionRecord{}
}

// readSSE consumes an event stream, returning the "line" event payloads and
// the terminal "end" payload.
func readSSE(t *testing.T, url string) (lines []string, endData string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("stream content-type = %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	var event, data string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "":
			switch event {
			case "line":
				lines = append(lines, data)
			case "end":
				return lines, data
			}
			event, data = "", ""
		}
	}
	return lines, endData
}

// --- tests ----------------------------------------------------------------

func TestActionStreamingSuccess(t *testing.T) {
	e := execEngine(t, Config{})
	srv := newTestServer(t, e, Config{}, quietLogger())

	status, body := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "compose", Verb: "up"})
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, body %v", status, body)
	}
	id := body["id"]
	if id == "" {
		t.Fatal("no id returned")
	}
	if body["command"] != "printf 'line1\\nline2\\n'" {
		t.Errorf("echoed command = %q", body["command"])
	}

	lines, endData := readSSE(t, srv.URL+"/api/actions/"+id+"/stream")
	if len(lines) < 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Fatalf("stream lines = %v", lines)
	}
	if !strings.Contains(endData, `"state":"succeeded"`) || !strings.Contains(endData, `"exit_code":0`) {
		t.Fatalf("end payload = %q", endData)
	}

	rec := waitTerminal(t, srv, id, "")
	if rec.State != "succeeded" || rec.ExitCode == nil || *rec.ExitCode != 0 {
		t.Fatalf("record = %+v", rec)
	}
}

func TestActionFailedSurfaces(t *testing.T) {
	// EXP-61: a failing command surfaces as a failed action with its exit code.
	e := execEngine(t, Config{})
	srv := newTestServer(t, e, Config{}, quietLogger())

	status, body := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "compose", Verb: "status"})
	if status != http.StatusAccepted {
		t.Fatalf("status = %d", status)
	}
	id := body["id"]

	_, endData := readSSE(t, srv.URL+"/api/actions/"+id+"/stream")
	if !strings.Contains(endData, `"state":"failed"`) || !strings.Contains(endData, `"exit_code":2`) {
		t.Fatalf("end payload = %q", endData)
	}
	rec := waitTerminal(t, srv, id, "")
	if rec.State != "failed" || rec.ExitCode == nil || *rec.ExitCode != 2 {
		t.Fatalf("record = %+v", rec)
	}
}

func TestConcurrencyGuard409(t *testing.T) {
	e := execEngine(t, Config{})
	e.timeout = func(string, string) time.Duration { return time.Second } // reap the sleep quickly
	srv := newTestServer(t, e, Config{}, quietLogger())

	req := ActionRequest{System: "fixture", Target: "kind", Verb: "up"} // sleep 5
	status1, body1 := postAction(t, srv, "", req)
	if status1 != http.StatusAccepted {
		t.Fatalf("first status = %d", status1)
	}
	status2, body2 := postAction(t, srv, "", req)
	if status2 != http.StatusConflict {
		t.Fatalf("second status = %d, want 409", status2)
	}
	if body2["running_id"] != body1["id"] {
		t.Errorf("409 running_id = %q, want %q", body2["running_id"], body1["id"])
	}
	// A different target is not blocked.
	status3, _ := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "compose", Verb: "up"})
	if status3 != http.StatusAccepted {
		t.Fatalf("different target status = %d", status3)
	}
}

func TestActionTimeout(t *testing.T) {
	e := execEngine(t, Config{})
	e.timeout = func(string, string) time.Duration { return 120 * time.Millisecond }
	srv := newTestServer(t, e, Config{}, quietLogger())

	status, body := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "kind", Verb: "up"}) // sleep 5
	if status != http.StatusAccepted {
		t.Fatalf("status = %d", status)
	}
	rec := waitTerminal(t, srv, body["id"], "")
	if rec.State != "failed" {
		t.Fatalf("state = %q, want failed", rec.State)
	}
	if rec.ExitCode == nil || *rec.ExitCode != -1 {
		t.Fatalf("exit code = %v, want -1 (timeout)", rec.ExitCode)
	}
}

func TestHTTPRejections(t *testing.T) {
	e := execEngine(t, Config{})
	srv := newTestServer(t, e, Config{}, quietLogger())

	cases := []struct {
		name string
		req  ActionRequest
		want int
	}{
		{"unknown system", ActionRequest{System: "ghost", Target: "compose", Verb: "up"}, 404},
		{"unknown verb", ActionRequest{System: "fixture", Target: "compose", Verb: "explode"}, 400},
		{"unknown target", ActionRequest{System: "fixture", Target: "aws", Verb: "up"}, 403}, // aws disabled
		{"down without confirm", ActionRequest{System: "fixture", Target: "compose", Verb: "down"}, 400},
		{"scale n out of range", ActionRequest{System: "fixture", Target: "compose", Verb: "scale", Params: map[string]string{"component": "worker", "n": "99"}}, 400},
		{"experiment unknown target", ActionRequest{System: "fixture", Target: "anything", Verb: "experiment", Params: map[string]string{"id": "exp-1"}}, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _ := postAction(t, srv, "", c.req)
			if status != c.want {
				t.Errorf("status = %d, want %d", status, c.want)
			}
		})
	}
}

func TestAWSGated403(t *testing.T) {
	e := execEngine(t, Config{}) // EnableAWS false
	srv := newTestServer(t, e, Config{}, quietLogger())
	// exec fixture has no aws target, but the gate fires before target lookup.
	status, _ := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "aws", Verb: "up"})
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}

func TestAuthMiddleware(t *testing.T) {
	const token = "s3cr3t-token"
	cfg := Config{Token: token}
	e := execEngine(t, cfg)

	var logbuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logbuf, nil))
	srv := newTestServer(t, e, cfg, log)

	// No token → 401.
	resp, _ := http.Get(srv.URL + "/api/systems")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Wrong token → 401.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/systems", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-token status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Audit line emitted.
	if !strings.Contains(logbuf.String(), "unauthorized") {
		t.Errorf("expected an audit log line, got: %s", logbuf.String())
	}

	// Right token → 200.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/systems", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("right-token status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSSETokenQueryParam(t *testing.T) {
	const token = "stream-token"
	cfg := Config{Token: token}
	e := execEngine(t, cfg)
	srv := newTestServer(t, e, cfg, quietLogger())

	status, body := postAction(t, srv, token, ActionRequest{System: "fixture", Target: "compose", Verb: "up"})
	if status != http.StatusAccepted {
		t.Fatalf("post status = %d", status)
	}
	id := body["id"]

	// Wrong ?token → 401.
	resp, _ := http.Get(srv.URL + "/api/actions/" + id + "/stream?token=nope")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong ?token stream = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Right ?token → 200 and streams (EventSource cannot set headers).
	lines, endData := readSSE(t, srv.URL+"/api/actions/"+id+"/stream?token="+token)
	if len(lines) < 2 {
		t.Fatalf("stream lines = %v", lines)
	}
	if !strings.Contains(endData, `"state":"succeeded"`) {
		t.Fatalf("end payload = %q", endData)
	}
}

// runRawAction registers a fabricated running action for cmd and executes it
// synchronously (same pattern as the report attachment tests), returning the
// terminal record and the broker's replay lines.
func runRawAction(t *testing.T, e *Engine, id, cmd string) (ActionRecord, []string) {
	t.Helper()
	key := runKey("raw", id)
	st := &actionState{
		rec: &ActionRecord{
			ID:        id,
			Request:   ActionRequest{System: "raw", Target: id, Verb: "up"},
			Command:   cmd,
			State:     "running",
			StartedAt: time.Now().UTC(),
		},
		broker: newBroker(ringMax),
	}
	e.mu.Lock()
	e.records[id] = st
	e.running[key] = id
	e.mu.Unlock()

	e.execute(st, key)

	replay, _, done, _ := st.broker.subscribe()
	if !done {
		t.Fatalf("broker not closed after execute")
	}
	return st.snapshot(), replay
}

func TestBackgroundWriterDoesNotBlockCompletion(t *testing.T) {
	// A backgrounded descendant holding the pipe write end must not keep a
	// finished action "running" until the verb timeout: the pump drains for a
	// short bounded grace after process exit, then the read end is closed.
	e := execEngine(t, Config{})
	start := time.Now()
	rec, replay := runRawAction(t, e, "bgwriter00000001", "echo hi; sleep 30 &")
	elapsed := time.Since(start)

	if rec.State != "succeeded" || rec.ExitCode == nil || *rec.ExitCode != 0 {
		t.Fatalf("record = %+v, want succeeded/0", rec)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("action took %s — the background writer gated completion", elapsed)
	}
	if len(replay) == 0 || replay[0] != "hi" {
		t.Errorf("replay = %v, want the echoed line first", replay)
	}
}

func TestScannerErrorPublishesMarker(t *testing.T) {
	// A single output line beyond the scanner buffer cap must not silently
	// fail the action: the child still runs to its own exit, and the stream
	// carries an explanatory truncation marker.
	e := execEngine(t, Config{})
	// ~6 MiB single line, over the 4 MiB cap, then a clean exit 0.
	rec, replay := runRawAction(t, e, "giantline0000001",
		"head -c 6291456 /dev/zero | tr '\\0' a; echo; echo done")
	if rec.State != "succeeded" || rec.ExitCode == nil || *rec.ExitCode != 0 {
		t.Fatalf("record = %+v, want succeeded/0 despite the oversized line", rec)
	}
	found := false
	for _, l := range replay {
		if strings.Contains(l, "output truncated") {
			found = true
		}
	}
	if !found {
		t.Errorf("no truncation marker in replay (len %d)", len(replay))
	}
}

func TestEngineShutdownFinalizesRunningAction(t *testing.T) {
	// Daemon shutdown must kill running actions' process groups so they
	// finalize (state/exit persisted to run history) instead of being lost.
	e := execEngine(t, Config{})
	srv := newTestServer(t, e, Config{}, quietLogger())

	status, body := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "kind", Verb: "up"}) // sleep 5
	if status != http.StatusAccepted {
		t.Fatalf("status = %d", status)
	}
	id := body["id"]
	time.Sleep(100 * time.Millisecond) // let the child start

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	e.Shutdown(ctx)
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("Shutdown took %s, want prompt group-kill + finalize", elapsed)
	}

	rec := getAction(t, srv, id, "")
	if rec.State != "failed" || rec.ExitCode == nil || *rec.ExitCode != -1 {
		t.Fatalf("record after shutdown = %+v, want failed/-1", rec)
	}
	runs, err := e.store.Runs(10)
	if err != nil {
		t.Fatalf("store runs: %v", err)
	}
	persisted := false
	for _, r := range runs {
		if r.ID == id {
			persisted = true
		}
	}
	if !persisted {
		t.Error("shutdown-canceled action not persisted to run history")
	}
}

func TestEngineRecordEviction(t *testing.T) {
	// Terminal records beyond recordsMax are evicted oldest-first: the id
	// 404s afterward while /api/runs keeps the durable history.
	e := execEngine(t, Config{})
	e.recordsMax = 1
	srv := newTestServer(t, e, Config{}, quietLogger())

	_, body1 := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "compose", Verb: "up"})
	waitTerminal(t, srv, body1["id"], "")
	_, body2 := postAction(t, srv, "", ActionRequest{System: "fixture", Target: "compose", Verb: "up"})
	waitTerminal(t, srv, body2["id"], "")

	// Eviction happens inside finalize just after the state turns terminal;
	// poll briefly for the 404.
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(srv.URL + "/api/actions/" + body1["id"])
		if err != nil {
			t.Fatalf("GET evicted action: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("first action not evicted (status %d)", resp.StatusCode)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The newest record is still served.
	if rec := getAction(t, srv, body2["id"], ""); rec.State != "succeeded" {
		t.Errorf("retained record = %+v", rec)
	}
	// Both remain in the durable history.
	runs, err := e.store.Runs(10)
	if err != nil {
		t.Fatalf("store runs: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("durable history = %d records, want 2", len(runs))
	}
}

func TestTokenQueryRejectedOffStream(t *testing.T) {
	// The ?token= fallback exists for EventSource only: it must be accepted
	// solely on the SSE stream route, never on other /api endpoints.
	const token = "scoped-token"
	cfg := Config{Token: token}
	e := execEngine(t, cfg)
	srv := newTestServer(t, e, cfg, quietLogger())

	for _, path := range []string{"/api/systems", "/api/runs"} {
		resp, err := http.Get(srv.URL + path + "?token=" + token)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s with ?token = %d, want 401", path, resp.StatusCode)
		}
	}
	// POST with ?token is rejected too (correct-token Bearer still works —
	// covered by TestAuthMiddleware; stream ?token by TestSSETokenQueryParam).
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/actions?token="+token,
		strings.NewReader(`{"system":"fixture","target":"compose","verb":"up"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST actions: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /api/actions with ?token = %d, want 401", resp.StatusCode)
	}
}

func TestSystemsEndpoint(t *testing.T) {
	reg, err := LoadRegistry("testdata/registry", quietLogger())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	e := NewEngine(Config{RepoRoot: t.TempDir()}, reg, nil, quietLogger())
	srv := newTestServer(t, e, Config{}, quietLogger())

	resp, err := http.Get(srv.URL + "/api/systems")
	if err != nil {
		t.Fatalf("GET systems: %v", err)
	}
	defer resp.Body.Close()
	var systems []System
	if err := json.NewDecoder(resp.Body).Decode(&systems); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(systems) != 2 || systems[0].Name != "hello-guest" {
		t.Fatalf("systems = %+v", systems)
	}
	_ = io.Discard
}
