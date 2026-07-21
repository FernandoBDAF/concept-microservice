package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServeGracefulShutdown covers the daemon lifecycle: serve answers
// requests until the signal context is canceled, then cancels running actions
// (so they finalize and persist to run history) and drains HTTP before
// returning cleanly — Ctrl-C must actually stop the daemon.
func TestServeGracefulShutdown(t *testing.T) {
	e := execEngine(t, Config{})
	handler := testRouter(e, Config{}, quietLogger())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	base := "http://" + ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- serve(ctx, &http.Server{Handler: handler}, ln, Config{}, e, quietLogger())
	}()

	// The server answers while the context is live.
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(base + "/api/systems")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("systems status = %d", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Start a long-running action, then simulate SIGINT.
	rec, err := e.StartAction(ActionRequest{System: "fixture", Target: "kind", Verb: "up"}) // sleep 5
	if err != nil {
		t.Fatalf("start action: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // let the child start
	cancel()

	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("serve did not return after context cancel")
	}

	// The running action was canceled, finalized, and persisted.
	st, ok := e.lookup(rec.ID)
	if !ok {
		t.Fatal("action record gone after shutdown")
	}
	if got := st.snapshot(); got.State != "failed" || got.EndedAt == nil {
		t.Fatalf("action after shutdown = %+v, want finalized failed", got)
	}
	runs, err := e.store.Runs(10)
	if err != nil {
		t.Fatalf("store runs: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.ID == rec.ID {
			found = true
		}
	}
	if !found {
		t.Error("shutdown-canceled action missing from run history")
	}

	// And the listener is really closed.
	if _, err := http.Get(base + "/api/systems"); err == nil {
		t.Error("server still answering after shutdown")
	}
}
