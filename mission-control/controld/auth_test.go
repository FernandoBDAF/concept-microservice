package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateStartup(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"localhost no-aws needs nothing", Config{}, false},
		{"no-aws with nothing set", Config{EnableAWS: false}, false},
		{"aws without token", Config{EnableAWS: true, TLSCert: "c", TLSKey: "k"}, true},
		{"aws without tls cert", Config{EnableAWS: true, Token: "t", TLSKey: "k"}, true},
		{"aws without tls key", Config{EnableAWS: true, Token: "t", TLSCert: "c"}, true},
		{"aws with token+tls ok", Config{EnableAWS: true, Token: "t", TLSCert: "c", TLSKey: "k"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateStartup(c.cfg)
			if c.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// withAWSProbe swaps the injectable probe and clears the cache, restoring both
// on cleanup so probe tests do not leak state into one another.
func withAWSProbe(t *testing.T, fn func(ctx context.Context, repoRoot string) (string, error)) {
	t.Helper()
	prev := awsProbeRun
	awsProbeRun = fn
	resetAWSProbeCache()
	t.Cleanup(func() {
		awsProbeRun = prev
		resetAWSProbeCache()
	})
}

func TestAWSTargetEntryShape(t *testing.T) {
	// The UI depends on exactly {"name","available","note"}.
	withAWSProbe(t, func(context.Context, string) (string, error) {
		return "", errors.New("no state")
	})
	e := AWSTargetEntry(Config{})
	if len(e) != 3 {
		t.Fatalf("entry has %d keys, want exactly 3: %v", len(e), e)
	}
	for _, k := range []string{"name", "available", "note"} {
		if _, ok := e[k]; !ok {
			t.Errorf("missing key %q in %v", k, e)
		}
	}
	if e["name"] != "aws" {
		t.Errorf("name = %v, want aws", e["name"])
	}
}

func TestAWSTargetEntryAvailability(t *testing.T) {
	cases := []struct {
		name          string
		out           string
		err           error
		wantAvailable bool
		wantNoteHas   string
	}{
		{"session up", "lab-prod-eks\n", nil, true, "cluster lab-prod-eks"},
		{"terraform missing", "", errors.New("exec: \"terraform\": executable file not found in $PATH"), false, "terraform"},
		{"not initialized", "", errors.New("No state file was found!"), false, "No state file"},
		{"empty output", "  \n", nil, false, "no cluster_name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withAWSProbe(t, func(context.Context, string) (string, error) {
				return c.out, c.err
			})
			e := AWSTargetEntry(Config{})
			if e["available"] != c.wantAvailable {
				t.Errorf("available = %v, want %v (note %q)", e["available"], c.wantAvailable, e["note"])
			}
			note, _ := e["note"].(string)
			if !strings.Contains(note, c.wantNoteHas) {
				t.Errorf("note = %q, want it to contain %q", note, c.wantNoteHas)
			}
		})
	}
}

func TestAWSTargetEntryCaches(t *testing.T) {
	calls := 0
	withAWSProbe(t, func(context.Context, string) (string, error) {
		calls++
		return "cluster-x\n", nil
	})
	// Three calls within the TTL → the underlying probe runs exactly once.
	for i := 0; i < 3; i++ {
		if e := AWSTargetEntry(Config{}); e["available"] != true {
			t.Fatalf("call %d: available = %v", i, e["available"])
		}
	}
	if calls != 1 {
		t.Errorf("probe ran %d times, want 1 (cached for %s)", calls, awsProbeTTL)
	}

	// After a cache reset the probe runs again — and a new outcome shows through.
	resetAWSProbeCache()
	awsProbeRun = func(context.Context, string) (string, error) {
		calls++
		return "", errors.New("session torn down")
	}
	if e := AWSTargetEntry(Config{}); e["available"] != false {
		t.Errorf("after reset, available = %v, want false", e["available"])
	}
	if calls != 2 {
		t.Errorf("probe ran %d times total, want 2", calls)
	}
}
