package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rec(id string, started time.Time, exit int) ActionRecord {
	ended := started.Add(time.Second)
	return ActionRecord{
		ID:        id,
		Request:   ActionRequest{System: "lab", Target: "compose", Verb: "up"},
		Command:   "make up",
		State:     "succeeded",
		ExitCode:  &exit,
		StartedAt: started,
		EndedAt:   &ended,
	}
}

func TestStoreAppendAndReadNewestFirst(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runs")
	s := NewStore(dir, quietLogger())

	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	// Two records same day, one the next day.
	must(t, s.Append(rec("aaaa", base, 0)))
	must(t, s.Append(rec("bbbb", base.Add(time.Hour), 0)))
	must(t, s.Append(rec("cccc", base.Add(26*time.Hour), 0))) // next UTC day

	runs, err := s.Runs(50)
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("want 3 runs, got %d", len(runs))
	}
	// Newest-first: cccc (next day), then bbbb, then aaaa within the first day.
	want := []string{"cccc", "bbbb", "aaaa"}
	for i, id := range want {
		if runs[i].ID != id {
			t.Errorf("runs[%d].ID = %s, want %s", i, runs[i].ID, id)
		}
	}
}

func TestStoreRunsLimit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runs")
	s := NewStore(dir, quietLogger())
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		must(t, s.Append(rec(string(rune('a'+i)), base.Add(time.Duration(i)*time.Minute), 0)))
	}
	runs, err := s.Runs(2)
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("limit not honored: got %d", len(runs))
	}
}

func TestStoreRunsEmpty(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "runs"), quietLogger())
	runs, err := s.Runs(50)
	if err != nil {
		t.Fatalf("Runs on empty: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("want 0 runs, got %d", len(runs))
	}
}

func TestStoreSkipsCorruptLine(t *testing.T) {
	// One torn JSONL line (e.g. a crash mid-append) must not permanently fail
	// GET /api/runs: it is skipped with a warning (logged once), and the good
	// records around it still come back.
	dir := filepath.Join(t.TempDir(), "runs")
	var logbuf bytes.Buffer
	s := NewStore(dir, slog.New(slog.NewTextHandler(&logbuf, nil)))

	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	must(t, s.Append(rec("good1", base, 0)))
	// Torn line: a truncated record, no closing brace.
	path := filepath.Join(dir, "2026-07-18.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	must(t, err)
	_, err = f.WriteString("{\"id\":\"torn\",\"request\":{\"sys\n")
	must(t, err)
	must(t, f.Close())
	must(t, s.Append(rec("good2", base.Add(time.Hour), 0)))

	runs, err := s.Runs(50)
	if err != nil {
		t.Fatalf("Runs with corrupt line: %v", err)
	}
	if len(runs) != 2 || runs[0].ID != "good2" || runs[1].ID != "good1" {
		t.Fatalf("runs = %+v, want [good2 good1]", runs)
	}
	if !strings.Contains(logbuf.String(), "corrupt") {
		t.Errorf("expected a corrupt-line warning, log: %s", logbuf.String())
	}

	// The warning is logged once per file:line, not on every read.
	before := strings.Count(logbuf.String(), "corrupt")
	if _, err := s.Runs(50); err != nil {
		t.Fatalf("second Runs: %v", err)
	}
	if after := strings.Count(logbuf.String(), "corrupt"); after != before {
		t.Errorf("corrupt warning repeated: %d -> %d occurrences", before, after)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
