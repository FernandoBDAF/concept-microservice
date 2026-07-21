package main

// sessions.go — the practice-session recorder (v6-HANDOFF §6, wave 2). This is
// the EXP-60 artifact: a terminal-free write-up. A session bounds a window of
// lab work; actions (from the wave-1 run history), experiment outcomes, and
// free-text notes recorded during that window are merged into a paste-ready
// markdown summary.
//
//   POST  /api/sessions               -> 201 {id,title,started_at}; 409 if one open
//   GET   /api/sessions/current       -> 200 the open session | 404
//   PATCH /api/sessions/{id}          -> 200; body {note?, close?}
//   GET   /api/sessions/{id}/summary  -> text/markdown write-up | 404
//
// Persistence: JSONL events (opened|note|outcome|closed) appended to
// runs/sessions.jsonl (the same gitignored dir as wave-1 run history). State is
// memory-first: on daemon restart an open session MAY be forgotten, but its
// events remain on disk. No database (ADR-005.2).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sessionTitleMax = 200
	// sessionRunsLimit bounds how many run-history records the summary scans
	// for actions that fall inside the session window.
	sessionRunsLimit = 500
	// sessionsMax caps the in-memory session map: closed sessions beyond this
	// count are evicted oldest-first (the open session is never evicted).
	// After eviction — or a daemon restart — PATCH/summary 404 for that id;
	// the JSONL event log under runs/sessions.jsonl remains on disk.
	sessionsMax = 200
)

// runsSource is the minimal slice of wave-1's *Store the summary needs. *Store
// satisfies it via `func (s *Store) Runs(limit int) ([]ActionRecord, error)`.
type runsSource interface {
	Runs(limit int) ([]ActionRecord, error)
}

// Session is one practice session. Only the exported scalar fields are
// serialized (see SessionView); notes/outcomes are internal timeline data.
type Session struct {
	ID        string
	Title     string
	StartedAt time.Time
	EndedAt   *time.Time

	notes    []noteEntry
	outcomes []outcomeEntry
}

type noteEntry struct {
	at   time.Time
	text string
}

type outcomeEntry struct {
	at     time.Time
	exp    string
	result string
	notes  string
}

// SessionView is the JSON projection of a Session (the API object).
type SessionView struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func (s *Session) view() SessionView {
	return SessionView{ID: s.ID, Title: s.Title, StartedAt: s.StartedAt, EndedAt: s.EndedAt}
}

// sessionError carries an HTTP status; the open-conflict case also carries the
// id of the session already open (rendered as {"error","open_id"}).
type sessionError struct {
	status int
	msg    string
	openID string
}

func (e *sessionError) Error() string { return e.msg }

// Recorder owns session state and event persistence. Concurrency-safe: every
// mutation and read goes through mu.
type Recorder struct {
	dir  string     // directory for sessions.jsonl (same as wave-1 runs/)
	runs runsSource // wave-1 run history for the summary timeline; may be nil
	log  *slog.Logger

	// now is injectable so timestamps are deterministic in tests.
	now func() time.Time
	// closedMax caps retained closed sessions; a field so tests can shrink it.
	closedMax int

	mu     sync.Mutex
	open   *Session            // the currently-open session, nil if none
	byID   map[string]*Session // retained sessions (open + last closedMax closed)
	closed []string            // closed session ids, oldest first (eviction order)
}

// NewRecorder constructs a Recorder. runs may be nil (the summary then carries
// no action rows — useful in tests that stub or omit history).
func NewRecorder(dir string, runs runsSource, log *slog.Logger) *Recorder {
	if log == nil {
		log = slog.Default()
	}
	return &Recorder{
		dir:       dir,
		runs:      runs,
		log:       log,
		now:       time.Now,
		closedMax: sessionsMax,
		byID:      map[string]*Session{},
	}
}

// Open starts a new session. Only one may be open at a time.
func (r *Recorder) Open(title string) (SessionView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return SessionView{}, &sessionError{status: http.StatusBadRequest, msg: "title is required"}
	}
	if len(title) > sessionTitleMax {
		return SessionView{}, &sessionError{status: http.StatusBadRequest,
			msg: fmt.Sprintf("title too long (max %d chars)", sessionTitleMax)}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.open != nil {
		return SessionView{}, &sessionError{status: http.StatusConflict,
			msg: "a session is already open", openID: r.open.ID}
	}
	s := &Session{ID: newSessionID(), Title: title, StartedAt: r.now().UTC()}
	r.open = s
	r.byID[s.ID] = s
	r.appendEvent(sessionEvent{Session: s.ID, Kind: "opened", At: s.StartedAt, Title: s.Title})
	r.log.Info("session opened", "id", s.ID, "title", s.Title)
	return s.view(), nil
}

// Current returns the open session, if any.
func (r *Recorder) Current() (SessionView, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.open == nil {
		return SessionView{}, false
	}
	return r.open.view(), true
}

// Patch applies an optional note and/or a close to a session. note==nil (or
// blank) is a no-op for notes; a note on a closed session → 409 (matching
// close-after-close); doClose closes an open session (second close → 409).
// Unknown id → 404.
func (r *Recorder) Patch(id string, note *string, doClose bool) (SessionView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok {
		return SessionView{}, &sessionError{status: http.StatusNotFound, msg: "no such session: " + id}
	}
	if note != nil && strings.TrimSpace(*note) != "" {
		if s.EndedAt != nil {
			return SessionView{}, &sessionError{status: http.StatusConflict, msg: "session already closed"}
		}
		at := r.now().UTC()
		s.notes = append(s.notes, noteEntry{at: at, text: *note})
		r.appendEvent(sessionEvent{Session: s.ID, Kind: "note", At: at, Note: *note})
		r.log.Info("session note added", "id", s.ID)
	}
	if doClose {
		if s.EndedAt != nil {
			return SessionView{}, &sessionError{status: http.StatusConflict, msg: "session already closed"}
		}
		at := r.now().UTC()
		s.EndedAt = &at
		if r.open != nil && r.open.ID == s.ID {
			r.open = nil
		}
		// Cap retained closed sessions, evicting oldest-first (the view is
		// snapshotted before a potential self-eviction at closedMax=0 edge;
		// the JSONL event log keeps the durable trace).
		r.closed = append(r.closed, s.ID)
		for len(r.closed) > r.closedMax {
			delete(r.byID, r.closed[0])
			r.closed = r.closed[1:]
		}
		r.appendEvent(sessionEvent{Session: s.ID, Kind: "closed", At: at})
		r.log.Info("session closed", "id", s.ID)
	}
	return s.view(), nil
}

// RecordOutcome attaches an experiment outcome to the open session (if any) as
// a timeline entry and a persisted event. Best-effort: a no-op when nothing is
// open. Called directly by the experiment outcome handler (same wave).
func (r *Recorder) RecordOutcome(exp, result, notes string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.open == nil {
		return
	}
	at := r.now().UTC()
	r.open.outcomes = append(r.open.outcomes, outcomeEntry{at: at, exp: exp, result: result, notes: notes})
	r.appendEvent(sessionEvent{Session: r.open.ID, Kind: "outcome", At: at, Exp: exp, Result: result, Note: notes})
}

// Summary renders the paste-ready markdown write-up for a session. Unknown id →
// (\"\", false).
func (r *Recorder) Summary(id string) (string, bool) {
	r.mu.Lock()
	s, ok := r.byID[id]
	if !ok {
		r.mu.Unlock()
		return "", false
	}
	// Snapshot everything we need under the lock.
	start := s.StartedAt
	var end time.Time
	if s.EndedAt != nil {
		end = *s.EndedAt
	} else {
		end = r.now().UTC()
	}
	view := s.view()
	notes := append([]noteEntry(nil), s.notes...)
	outcomes := append([]outcomeEntry(nil), s.outcomes...)
	runs := r.runs
	r.mu.Unlock()

	return renderSummary(view, start, end, notes, outcomes, runs, r.log), true
}

// timelineItem is one line in the chronological summary timeline.
type timelineItem struct {
	at   time.Time
	text string
}

// renderSummary builds the markdown. It merges (a) actions whose StartedAt
// falls inside [start,end], (b) experiment outcomes, and (c) notes, sorted by
// time, then frames them with the header and the v6 footer.
func renderSummary(v SessionView, start, end time.Time, notes []noteEntry, outcomes []outcomeEntry, runs runsSource, log *slog.Logger) string {
	var items []timelineItem

	if runs != nil {
		recs, err := runs.Runs(sessionRunsLimit)
		if err != nil {
			log.Warn("session summary: reading run history failed", "error", err.Error())
		}
		for _, rec := range recs {
			st := rec.StartedAt.UTC()
			if st.Before(start) || st.After(end) {
				continue
			}
			items = append(items, timelineItem{at: st, text: actionLine(rec)})
		}
	}
	for _, o := range outcomes {
		items = append(items, timelineItem{at: o.at, text: outcomeLine(o)})
	}
	for _, n := range notes {
		items = append(items, timelineItem{at: n.at, text: fmt.Sprintf("**note** — %s", n.text)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].at.Before(items[j].at) })

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", v.Title)
	fmt.Fprintf(&b, "- Started: %s\n", start.Format(time.RFC3339))
	if v.EndedAt != nil {
		fmt.Fprintf(&b, "- Ended: %s\n", end.Format(time.RFC3339))
		fmt.Fprintf(&b, "- Duration: %s\n", end.Sub(start).Round(time.Second))
	} else {
		b.WriteString("- Ended: (in progress)\n")
		fmt.Fprintf(&b, "- Duration: %s (so far)\n", end.Sub(start).Round(time.Second))
	}
	b.WriteString("\n## Timeline\n\n")
	if len(items) == 0 {
		b.WriteString("_(nothing recorded)_\n")
	}
	for _, it := range items {
		fmt.Fprintf(&b, "- `%s` %s\n", it.at.Format(time.RFC3339), it.text)
	}
	b.WriteString("\nRecorded by Mission Control (lab-controld) — phase v6.\n")
	return b.String()
}

// actionLine renders one run-history record as a timeline line: system/target/
// verb, the exact command, terminal state + exit code, and duration.
func actionLine(rec ActionRecord) string {
	exit := "?"
	if rec.ExitCode != nil {
		exit = fmt.Sprintf("%d", *rec.ExitCode)
	}
	dur := "—"
	if rec.EndedAt != nil {
		dur = rec.EndedAt.Sub(rec.StartedAt).Round(time.Millisecond).String()
	}
	return fmt.Sprintf("**action** %s/%s/%s — `%s` — %s (exit %s) — %s",
		rec.Request.System, rec.Request.Target, rec.Request.Verb, rec.Command, rec.State, exit, dur)
}

// outcomeLine renders one experiment outcome as a timeline line.
func outcomeLine(o outcomeEntry) string {
	notes := strings.TrimSpace(o.notes)
	if notes == "" {
		notes = "(no notes)"
	}
	return fmt.Sprintf("**outcome** %s — %s — %s", o.exp, o.result, notes)
}

// sessionEvent is one persisted JSONL line under runs/sessions.jsonl.
type sessionEvent struct {
	Session string    `json:"session"`
	Kind    string    `json:"kind"` // opened|note|outcome|closed
	At      time.Time `json:"at"`
	Title   string    `json:"title,omitempty"`
	Note    string    `json:"note,omitempty"`
	Exp     string    `json:"exp,omitempty"`
	Result  string    `json:"result,omitempty"`
}

// appendEvent appends one JSONL line to runs/sessions.jsonl. Called with r.mu
// held. Errors are logged, not returned — persistence is best-effort and must
// never block a recorder mutation (the in-memory state is the live truth).
func (r *Recorder) appendEvent(ev sessionEvent) {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		r.log.Error("session event: mkdir failed", "dir", r.dir, "error", err.Error())
		return
	}
	line, err := json.Marshal(ev)
	if err != nil {
		r.log.Error("session event: marshal failed", "error", err.Error())
		return
	}
	path := filepath.Join(r.dir, "sessions.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		r.log.Error("session event: open failed", "path", path, "error", err.Error())
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		r.log.Error("session event: write failed", "path", path, "error", err.Error())
	}
}

// newSessionID returns 16 hex chars from crypto/rand (8 bytes).
func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; fall back to a time-derived value.
		n := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(n >> (8 * uint(i)))
		}
	}
	return hex.EncodeToString(b[:])
}

// ---- HTTP handlers (wired into main.go's mux by the orchestrator) ----

// HandleCreate serves POST /api/sessions → 201 {id,title,started_at}; 409 when
// one is already open ({"error","open_id"}).
func (r *Recorder) HandleCreate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	sv, err := r.Open(body.Title)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sv)
}

// HandleCurrent serves GET /api/sessions/current → 200 the open session | 404.
func (r *Recorder) HandleCurrent(w http.ResponseWriter, req *http.Request) {
	sv, ok := r.Current()
	if !ok {
		writeError(w, http.StatusNotFound, "no open session")
		return
	}
	writeJSON(w, http.StatusOK, sv)
}

// HandlePatch serves PATCH /api/sessions/{id} → 200 the session object.
func (r *Recorder) HandlePatch(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	var body struct {
		Note  *string `json:"note"`
		Close *bool   `json:"close"`
	}
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	doClose := body.Close != nil && *body.Close
	sv, err := r.Patch(id, body.Note, doClose)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sv)
}

// HandleSummary serves GET /api/sessions/{id}/summary → text/markdown | 404.
func (r *Recorder) HandleSummary(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	md, ok := r.Summary(id)
	if !ok {
		writeError(w, http.StatusNotFound, "no such session: "+id)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, md)
}

// writeSessionError maps a *sessionError to its HTTP response; the open-conflict
// case carries open_id.
func writeSessionError(w http.ResponseWriter, err error) {
	if se, ok := err.(*sessionError); ok {
		if se.openID != "" {
			writeJSON(w, se.status, map[string]string{"error": se.msg, "open_id": se.openID})
			return
		}
		writeError(w, se.status, se.msg)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
