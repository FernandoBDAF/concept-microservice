package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransformPriorityMapping(t *testing.T) {
	cases := []struct {
		name     string
		in       alert
		priority string
		title    string
	}{
		{
			name: "critical firing is urgent",
			in: alert{
				Status: "firing",
				Labels: map[string]string{"alertname": "AuthBreakerOpen", "severity": "critical"},
			},
			priority: "urgent",
			title:    "AuthBreakerOpen [firing]",
		},
		{
			name: "warning firing is high",
			in: alert{
				Status: "firing",
				Labels: map[string]string{"alertname": "QueueDepthSustained", "severity": "warning"},
			},
			priority: "high",
			title:    "QueueDepthSustained [firing]",
		},
		{
			name: "resolved is default regardless of severity",
			in: alert{
				Status: "resolved",
				Labels: map[string]string{"alertname": "QueueDepthSustained", "severity": "critical"},
			},
			priority: "default",
			title:    "QueueDepthSustained [resolved]",
		},
		{
			name:     "missing everything still yields a sane notification",
			in:       alert{},
			priority: "default",
			title:    "unknown-alert [firing]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := transform(tc.in)
			if n.Priority != tc.priority {
				t.Errorf("priority = %q, want %q", n.Priority, tc.priority)
			}
			if n.Title != tc.title {
				t.Errorf("title = %q, want %q", n.Title, tc.title)
			}
		})
	}
}

func TestTransformBody(t *testing.T) {
	n := transform(alert{
		Status: "firing",
		Labels: map[string]string{"alertname": "DLQGrowth", "severity": "warning"},
		Annotations: map[string]string{
			"summary":     "DLQ email.dlq is growing",
			"description": "email.dlq gained messages.",
			"runbook_url": "https://example.test/EXPERIMENTS.md#exp-05",
		},
	})
	for _, want := range []string{
		"DLQ email.dlq is growing",
		"email.dlq gained messages.",
		"Runbook: https://example.test/EXPERIMENTS.md#exp-05",
	} {
		if !strings.Contains(n.Body, want) {
			t.Errorf("body missing %q; body = %q", want, n.Body)
		}
	}
}

// End-to-end through the HTTP handler: Alertmanager payload in, ntfy
// publishes out (captured by a fake ntfy server).
func TestHandleAlertForwards(t *testing.T) {
	type published struct {
		path, title, priority, body string
	}
	var got []published
	ntfy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, published{
			path:     r.URL.Path,
			title:    r.Header.Get("Title"),
			priority: r.Header.Get("Priority"),
			body:     string(body),
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfy.Close()

	rl := &relay{ntfyURL: ntfy.URL, topic: "lab-alerts", client: ntfy.Client()}

	payload := webhookPayload{
		Status: "firing",
		Alerts: []alert{
			{
				Status:      "firing",
				Labels:      map[string]string{"alertname": "ScrapeTargetDown", "severity": "warning"},
				Annotations: map[string]string{"summary": "target down"},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	rl.handleAlert(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handler returned %d: %s", rec.Code, rec.Body.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(got))
	}
	if got[0].path != "/lab-alerts" {
		t.Errorf("published to %q, want /lab-alerts", got[0].path)
	}
	if got[0].title != "ScrapeTargetDown [firing]" {
		t.Errorf("title = %q", got[0].title)
	}
	if got[0].priority != "high" {
		t.Errorf("priority = %q, want high", got[0].priority)
	}
}

func TestHandleAlertRejectsGarbage(t *testing.T) {
	rl := &relay{ntfyURL: "http://unused", topic: "t", client: http.DefaultClient}
	req := httptest.NewRequest(http.MethodPost, "/alert", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	rl.handleAlert(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}
