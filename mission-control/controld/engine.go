package main

// engine.go — the action engine (HANDOFF §2, the heart of v6).
//
// It resolves a request to a whitelisted command (resolveCommand, actions.go),
// enforces one running action per (system,target), execs it as
// `sh -c <cmd>` from the repo root in its own process group, streams merged
// stdout/stderr into a broker (ring + SSE fanout), and finalizes the
// ActionRecord (state/exit code) with a JSONL append. It also carries the
// HTTP handlers the orchestrator wires into main.go's mux.

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// apiError carries an HTTP status alongside a message; the 409 case also
// carries the id of the action already running for the (system,target) pair.
type apiError struct {
	status    int
	msg       string
	runningID string
}

func (e *apiError) Error() string { return e.msg }

func apiErr(status int, msg string) error { return &apiError{status: status, msg: msg} }

// statusOf extracts the HTTP status from an error (500 if not an apiError).
func statusOf(err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.status
	}
	return http.StatusInternalServerError
}

// engineRecordsMax caps the in-memory record map: terminal records beyond this
// count are evicted oldest-first (running actions are never evicted). After
// eviction — or a daemon restart — GET /api/actions/{id} 404s for the evicted
// id; /api/runs remains the durable history (JSONL on disk).
const engineRecordsMax = 500

// outputDrainGrace is how long, after the action process itself exits, the
// output pump keeps reading for lingering pipe writers (backgrounded
// descendants that inherited the write end) before the read end is closed.
const outputDrainGrace = 2 * time.Second

// Engine owns the live action state.
type Engine struct {
	cfg   Config
	reg   *Registry
	store *Store
	log   *slog.Logger

	// timeout is the per-verb budget; a field so tests can shorten it.
	timeout func(verb, target string) time.Duration
	// recordsMax caps the terminal-record memory; a field so tests can shrink it.
	recordsMax int

	// shutdownCtx parents every action's exec context: canceling it (Shutdown)
	// kills the running process groups so their actions finalize and persist.
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	actions        sync.WaitGroup // one Add per launched action, Done on finalize

	mu       sync.Mutex
	records  map[string]*actionState // id -> state
	running  map[string]string       // "system\x00target" -> running action id
	terminal []string                // terminal record ids, oldest first (eviction order)
}

// actionState bundles a record with its output broker.
type actionState struct {
	mu     sync.Mutex
	rec    *ActionRecord
	broker *broker
}

func (st *actionState) snapshot() ActionRecord {
	st.mu.Lock()
	defer st.mu.Unlock()
	return *st.rec
}

// NewEngine constructs an engine. store may be nil (JSONL append is then a
// no-op — useful in tests that don't assert persistence).
func NewEngine(cfg Config, reg *Registry, store *Store, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	sctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		cfg:            cfg,
		reg:            reg,
		store:          store,
		log:            log,
		timeout:        verbTimeout,
		recordsMax:     engineRecordsMax,
		shutdownCtx:    sctx,
		shutdownCancel: cancel,
		records:        map[string]*actionState{},
		running:        map[string]string{},
	}
}

// Shutdown cancels every running action (their exec contexts are children of
// shutdownCtx, so the process groups are SIGKILLed) and waits — bounded by ctx
// — for them to finalize and persist to run history.
func (e *Engine) Shutdown(ctx context.Context) {
	e.shutdownCancel()
	done := make(chan struct{})
	go func() {
		e.actions.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		e.log.Warn("shutdown: timed out waiting for running actions to finalize")
	}
}

// verbTimeout is the per-verb execution budget (HANDOFF §2).
func verbTimeout(verb, target string) time.Duration {
	switch verb {
	case "status":
		return 60 * time.Second
	case "scale":
		return 5 * time.Minute
	case "experiment":
		return 30 * time.Minute
	case "up", "down":
		if target == "aws" {
			return 30 * time.Minute
		}
		return 10 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func runKey(system, target string) string { return system + "\x00" + target }

// StartAction resolves, guards, and launches an action. It returns the created
// record (state running) immediately; execution proceeds in a goroutine.
func (e *Engine) StartAction(req ActionRequest) (*ActionRecord, error) {
	cmd, err := resolveCommand(e.reg, e.cfg, req)
	if err != nil {
		return nil, err
	}

	key := runKey(req.System, req.Target)
	e.mu.Lock()
	if id, busy := e.running[key]; busy {
		e.mu.Unlock()
		return nil, &apiError{
			status:    http.StatusConflict,
			msg:       fmt.Sprintf("an action is already running for %s/%s (id %s)", req.System, req.Target, id),
			runningID: id,
		}
	}
	id := newID()
	now := time.Now().UTC()
	rec := &ActionRecord{
		ID:        id,
		Request:   req,
		Command:   cmd,
		State:     "running",
		StartedAt: now,
	}
	st := &actionState{rec: rec, broker: newBroker(ringMax)}
	e.records[id] = st
	e.running[key] = id
	e.mu.Unlock()

	e.actions.Add(1)
	go func() {
		defer e.actions.Done()
		e.execute(st, key)
	}()

	out := *rec
	return &out, nil
}

// execute runs the resolved command, streams output, and finalizes the record.
func (e *Engine) execute(st *actionState, key string) {
	req := st.rec.Request
	cmdStr := st.rec.Command
	budget := e.timeout(req.Verb, req.Target)

	// Child of shutdownCtx: daemon shutdown cancels the exec context, which
	// kills the process group (c.Cancel below) so the action still finalizes.
	ctx, cancel := context.WithTimeout(e.shutdownCtx, budget)
	defer cancel()

	c := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	c.Dir = e.cfg.RepoRoot
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// On timeout/cancel, kill the whole process group so make's children die.
	c.Cancel = func() error {
		if c.Process != nil {
			return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}

	// For experiment runs, point the scored runner at a per-action report dir
	// (under the gitignored runs/ tree) so its junit-ish XML can be attached to
	// the record after the process exits. Other verbs inherit the environment
	// untouched. A dir-creation failure degrades to no-report (warn), never a
	// hard failure of the action.
	reportDir := ""
	if req.Verb == "experiment" {
		if d := reportDirFor(e.store, st.rec.ID); d != "" {
			if err := os.MkdirAll(d, 0o755); err != nil {
				e.log.Warn("experiment report dir unavailable", "id", st.rec.ID, "error", err.Error())
			} else {
				reportDir = d
				c.Env = append(os.Environ(), "EXPERIMENT_REPORT_DIR="+d)
			}
		}
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		e.finalize(st, key, "failed", -1, fmt.Sprintf(startupErrMark, err))
		return
	}
	c.Stdout = pw
	c.Stderr = pw // merge stdout+stderr into one stream

	if err := c.Start(); err != nil {
		pw.Close()
		pr.Close()
		e.finalize(st, key, "failed", -1, fmt.Sprintf(startupErrMark, err))
		return
	}
	// Parent must drop its writer copy so the reader sees EOF when the child
	// (and its group) exit.
	pw.Close()

	// Pump merged output in a goroutine: the scanner must not gate c.Wait(),
	// because a backgrounded descendant holding the pipe write end would
	// otherwise keep a finished action "running" until the verb timeout. On a
	// scanner error (e.g. a single line beyond the buffer cap) the pump keeps
	// draining the pipe so the child never blocks on a full pipe, and the
	// error is surfaced as a marker line below instead of dying silently.
	scanDone := make(chan struct{})
	var scanErr error
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 4 MiB max line
		for sc.Scan() {
			st.broker.publish(sc.Text())
		}
		if err := sc.Err(); err != nil {
			scanErr = err
			io.Copy(io.Discard, pr) // keep draining so the child can exit
		}
	}()

	waitErr := c.Wait()

	// The process (group leader) has exited. Allow a short bounded drain for
	// stragglers still holding the pipe, then close the read end to unblock
	// the scanner — a setsid escapee must not wedge the (system,target) slot.
	drained := true
	select {
	case <-scanDone:
	case <-time.After(outputDrainGrace):
		drained = false
	}
	pr.Close()
	<-scanDone

	if !drained {
		st.broker.publish(outputCutMark)
	} else if scanErr != nil {
		st.broker.publish(fmt.Sprintf(scanErrMark, scanErr))
	}

	state, exit := "succeeded", 0
	var marker string
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		state, exit = "failed", -1
		marker = fmt.Sprintf(timeoutMarker, budget)
	case waitErr != nil && ctx.Err() == context.Canceled:
		// Daemon shutdown canceled the exec context and killed the group.
		state, exit = "failed", -1
		marker = shutdownMarker
	case waitErr != nil:
		state = "failed"
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			exit = ee.ExitCode() // real command exit code (EXP-61)
		} else {
			exit = -1
			marker = fmt.Sprintf(startupErrMark, waitErr)
		}
	}
	if marker != "" {
		st.broker.publish(marker)
	}

	// Attach the scored-run report (if any) BEFORE finalize snapshots the
	// record, so it lands in both the JSONL persistence and the GET response.
	// Lenient: no dir / no xml / parse error → Report stays nil + a warn.
	if reportDir != "" {
		if rep, rerr := parseExperimentReport(reportDir); rerr != nil {
			e.log.Warn("experiment report not attached", "id", st.rec.ID, "error", rerr.Error())
		} else {
			st.mu.Lock()
			st.rec.Report = rep
			st.mu.Unlock()
		}
	}

	e.finalize(st, key, state, exit, "")
}

// finalize records terminal state, releases the concurrency slot, appends to
// the store, and closes the broker with the SSE end payload. An optional
// preMarker line is published before closing (used for start failures).
func (e *Engine) finalize(st *actionState, key, state string, exit int, preMarker string) {
	if preMarker != "" {
		st.broker.publish(preMarker)
	}
	now := time.Now().UTC()
	st.mu.Lock()
	st.rec.State = state
	ec := exit
	st.rec.ExitCode = &ec
	st.rec.EndedAt = &now
	rec := *st.rec
	st.mu.Unlock()

	e.mu.Lock()
	delete(e.running, key)
	// Cap the in-memory record map: evict the oldest terminal records beyond
	// recordsMax. GET /api/actions/{id} 404s after eviction (as after a
	// restart); /api/runs stays the durable history.
	e.terminal = append(e.terminal, rec.ID)
	for len(e.terminal) > e.recordsMax {
		delete(e.records, e.terminal[0])
		e.terminal = e.terminal[1:]
	}
	e.mu.Unlock()

	if e.store != nil {
		if err := e.store.Append(rec); err != nil {
			e.log.Error("run append failed", "id", rec.ID, "error", err.Error())
		}
	}
	e.log.Info("action finished", "id", rec.ID, "system", rec.Request.System,
		"target", rec.Request.Target, "verb", rec.Request.Verb, "state", state, "exit_code", exit)

	st.broker.close(endPayload(state, exit))
}

func (e *Engine) lookup(id string) (*actionState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st, ok := e.records[id]
	return st, ok
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; fall back to time-derived bytes.
		binaryPutTime(b[:])
	}
	return hex.EncodeToString(b[:]) // 16 hex chars
}

func binaryPutTime(b []byte) {
	n := time.Now().UnixNano()
	for i := range b {
		b[i] = byte(n >> (8 * uint(i)))
	}
}

// parsePositiveInt parses a strictly positive base-10 integer.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("not positive: %d", n)
	}
	return n, nil
}

// ---- HTTP handlers (wired into main.go's mux by the orchestrator) ----

// HandleSystems serves GET /api/systems → []System sorted by name.
func (e *Engine) HandleSystems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, e.reg.Systems())
}

// HandleCreateAction serves POST /api/actions → 202 {id, command}.
func (e *Engine) HandleCreateAction(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	rec, err := e.StartAction(req)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) {
			if ae.runningID != "" {
				writeJSON(w, ae.status, map[string]string{"error": ae.msg, "running_id": ae.runningID})
				return
			}
			writeError(w, ae.status, ae.msg)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": rec.ID, "command": rec.Command})
}

// HandleGetAction serves GET /api/actions/{id} → full ActionRecord. Records
// are in-memory only: an id 404s after eviction (engineRecordsMax terminal
// records retained) or a daemon restart — /api/runs is the durable history.
func (e *Engine) HandleGetAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, ok := e.lookup(id)
	if !ok {
		writeError(w, http.StatusNotFound, "no such action: "+id)
		return
	}
	writeJSON(w, http.StatusOK, st.snapshot())
}

// HandleStreamAction serves GET /api/actions/{id}/stream (SSE).
func (e *Engine) HandleStreamAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, ok := e.lookup(id)
	if !ok {
		writeError(w, http.StatusNotFound, "no such action: "+id)
		return
	}
	serveSSE(w, r, st.broker)
}

// HandleRuns serves GET /api/runs?limit=N → newest-first ActionRecords.
func (e *Engine) HandleRuns(w http.ResponseWriter, r *http.Request) {
	limit := runsDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			limit = n
		}
	}
	if e.store == nil {
		writeJSON(w, http.StatusOK, []ActionRecord{})
		return
	}
	runs, err := e.store.Runs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}
