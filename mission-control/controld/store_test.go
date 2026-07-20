package main

import (
	"path/filepath"
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
	s := NewStore(dir)

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
	s := NewStore(dir)
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
	s := NewStore(filepath.Join(t.TempDir(), "runs"))
	runs, err := s.Runs(50)
	if err != nil {
		t.Fatalf("Runs on empty: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("want 0 runs, got %d", len(runs))
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
