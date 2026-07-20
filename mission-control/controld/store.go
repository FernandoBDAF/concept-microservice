package main

// store.go — run history as append-only JSONL, one file per UTC day under
// runs/YYYY-MM-DD.jsonl (no DB, per HANDOFF §2). One line is written per
// action, on completion, holding the full terminal ActionRecord. /api/runs
// reads newest-first by walking the day files from newest date backward.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	runsDefaultLimit = 50
	runsMaxLimit     = 500
)

// Store persists and reads ActionRecords as JSONL day files.
type Store struct {
	dir string
	log *slog.Logger
	mu  sync.Mutex // serializes appends AND reads (a read must not race an in-flight append)

	// warned dedupes corrupt-line warnings: one log line per file:line.
	warned map[string]bool
}

// NewStore constructs a Store. log may be nil (slog.Default is used).
func NewStore(dir string, log *slog.Logger) *Store {
	if log == nil {
		log = slog.Default()
	}
	return &Store{dir: dir, log: log, warned: map[string]bool{}}
}

// Dir returns the run-history directory. Per-action scored-run report subdirs
// (reports/<action-id>/) live under it — inside the gitignored runs/ tree.
func (s *Store) Dir() string { return s.dir }

// Append writes one JSON line for a completed record into the day file keyed
// by its StartedAt (UTC) date. The runs/ dir is created on demand (0755).
func (s *Store) Append(rec ActionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", s.dir, err)
	}
	day := rec.StartedAt.UTC().Format("2006-01-02")
	path := filepath.Join(s.dir, day+".jsonl")

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("store: marshal record %s: %w", rec.ID, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("store: open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("store: write %s: %w", path, err)
	}
	return nil
}

// Runs returns up to limit records, newest-first, walking day files from the
// newest date backward and reading each file's lines in reverse. It takes the
// store mutex so a read never races an in-flight Append.
func (s *Store) Runs(limit int) ([]ActionRecord, error) {
	if limit <= 0 {
		limit = runsDefaultLimit
	}
	if limit > runsMaxLimit {
		limit = runsMaxLimit
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	paths, err := filepath.Glob(filepath.Join(s.dir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("store: glob %s: %w", s.dir, err)
	}
	// Newest date first (filenames sort lexically == chronologically).
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))

	out := make([]ActionRecord, 0, limit)
	for _, p := range paths {
		recs, err := s.readDayReversed(p)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			out = append(out, r)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// readDayReversed reads one JSONL file and returns its records newest-first
// (i.e. last line first). Blank lines are skipped. A torn/corrupt line is
// skipped with a warning (logged once per file:line) — one bad line must never
// permanently fail the whole history read. Called with s.mu held.
func (s *Store) readDayReversed(path string) ([]ActionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	defer f.Close()

	var recs []ActionRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec ActionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			s.warnBadLine(filepath.Base(path), n, err)
			continue
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("store: scan %s: %w", filepath.Base(path), err)
	}
	// Reverse in place → newest-first within the day.
	for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
		recs[i], recs[j] = recs[j], recs[i]
	}
	return recs, nil
}

// warnBadLine logs a corrupt JSONL line once per file:line. Called with s.mu
// held.
func (s *Store) warnBadLine(file string, line int, err error) {
	key := fmt.Sprintf("%s:%d", file, line)
	if s.warned[key] {
		return
	}
	s.warned[key] = true
	s.log.Warn("store: skipping corrupt run-history line", "file", file, "line", line, "error", err.Error())
}
