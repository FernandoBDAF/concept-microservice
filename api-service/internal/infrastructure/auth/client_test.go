package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

func TestBreakerStateValue_Mapping(t *testing.T) {
	cases := []struct {
		state gobreaker.State
		want  float64
	}{
		{gobreaker.StateClosed, 0},
		{gobreaker.StateHalfOpen, 1},
		{gobreaker.StateOpen, 2},
	}
	for _, tc := range cases {
		if got := breakerStateValue(tc.state); got != tc.want {
			t.Errorf("breakerStateValue(%v) = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func newTestClient(handler http.HandlerFunc, readyToTrip uint32) *Client {
	server := httptest.NewServer(handler)

	cbCfg := config.CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    time.Minute,
		Timeout:     time.Minute,
		ReadyToTrip: readyToTrip,
	}
	authCfg := config.AuthConfig{URL: server.URL, Timeout: 2 * time.Second}

	return NewClient(authCfg, cbCfg)
}

func TestValidateToken_ValidTokenSucceeds(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","message":"ok","data":{"valid":true,"user":{"id":"u1","email":"a@b.com","role":"user"}}}`)
	}
	client := newTestClient(handler, 3)

	resp, err := client.ValidateToken(context.Background(), "good-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.Data.Valid {
		t.Errorf("expected valid=true")
	}
	if resp.Data.User.ID != "u1" {
		t.Errorf("expected user id 'u1', got %q", resp.Data.User.ID)
	}
}

func TestValidateToken_InvalidTokenDoesNotTripBreaker(t *testing.T) {
	var hits int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status":"error","message":"invalid token","data":{"valid":false}}`)
	}
	client := newTestClient(handler, 2) // breaker would open after 2 consecutive "failures"

	// Fire more invalid-token requests than the trip threshold. If 401s were
	// (incorrectly) counted as circuit-breaker failures, the breaker would
	// open and later calls would short-circuit without reaching the server.
	for i := 0; i < 5; i++ {
		resp, err := client.ValidateToken(context.Background(), "bad-token")
		if err != nil {
			t.Fatalf("call %d: expected no error for a well-formed 401 response, got %v", i, err)
		}
		if resp.Data.Valid {
			t.Fatalf("call %d: expected valid=false", i)
		}
	}

	if got := atomic.LoadInt32(&hits); got != 5 {
		t.Errorf("expected all 5 requests to reach auth-service (breaker should stay closed), got %d hits", got)
	}
}

func TestValidateToken_ServerErrorsTripBreaker(t *testing.T) {
	var hits int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}
	client := newTestClient(handler, 2)

	for i := 0; i < 2; i++ {
		if _, err := client.ValidateToken(context.Background(), "any-token"); err == nil {
			t.Fatalf("call %d: expected error for 5xx response", i)
		}
	}

	// Breaker should now be open; this call must fail without hitting the server.
	if _, err := client.ValidateToken(context.Background(), "any-token"); err == nil {
		t.Fatalf("expected error once breaker is open")
	}

	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("expected exactly 2 requests to reach the server before breaker opened, got %d", got)
	}
}
