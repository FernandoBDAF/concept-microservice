// ntfy-relay (ADR-003.4): a tiny Alertmanager-webhook -> ntfy bridge.
// Alertmanager POSTs its webhook payload to /alert; each alert in the batch
// becomes one ntfy publish to ${NTFY_URL}/${NTFY_TOPIC} with a Title,
// a body (summary/description + runbook link) and a Priority header
// (critical -> urgent, warning -> high, resolved -> default).
//
// Config (env):
//
//	NTFY_URL    base URL of the ntfy server (default http://ntfy;
//	            point at https://ntfy.sh to use a hosted topic instead)
//	NTFY_TOPIC  topic to publish to (default lab-alerts)
//	LISTEN_ADDR listen address (default :8080)
//
// Stdlib only on purpose — this is lab plumbing, not a service.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// webhookPayload is the subset of the Alertmanager webhook format
// (version "4") the relay cares about.
type webhookPayload struct {
	Status string  `json:"status"`
	Alerts []alert `json:"alerts"`
}

type alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// notification is one outgoing ntfy publish.
type notification struct {
	Title    string
	Body     string
	Priority string
	Tags     string
}

// transform maps one Alertmanager alert onto an ntfy notification.
func transform(a alert) notification {
	name := a.Labels["alertname"]
	if name == "" {
		name = "unknown-alert"
	}
	status := a.Status
	if status == "" {
		status = "firing"
	}

	var parts []string
	if s := a.Annotations["summary"]; s != "" {
		parts = append(parts, s)
	}
	if d := a.Annotations["description"]; d != "" {
		parts = append(parts, d)
	}
	if r := a.Annotations["runbook_url"]; r != "" {
		parts = append(parts, "Runbook: "+r)
	}
	if len(parts) == 0 {
		parts = append(parts, "(no summary)")
	}

	n := notification{
		Title: fmt.Sprintf("%s [%s]", name, status),
		Body:  strings.Join(parts, "\n"),
	}
	switch {
	case status == "resolved":
		n.Priority = "default"
		n.Tags = "white_check_mark"
	case a.Labels["severity"] == "critical":
		n.Priority = "urgent"
		n.Tags = "rotating_light"
	case a.Labels["severity"] == "warning":
		n.Priority = "high"
		n.Tags = "warning"
	default:
		n.Priority = "default"
		n.Tags = "bell"
	}
	return n
}

type relay struct {
	ntfyURL string
	topic   string
	client  *http.Client
}

func (r *relay) publish(n notification) error {
	req, err := http.NewRequest(http.MethodPost,
		strings.TrimRight(r.ntfyURL, "/")+"/"+r.topic,
		strings.NewReader(n.Body))
	if err != nil {
		return err
	}
	req.Header.Set("Title", n.Title)
	req.Header.Set("Priority", n.Priority)
	req.Header.Set("Tags", n.Tags)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned %s", resp.Status)
	}
	return nil
}

func (r *relay) handleAlert(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var payload webhookPayload
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<20)).Decode(&payload); err != nil {
		http.Error(w, "bad payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	var failed int
	for _, a := range payload.Alerts {
		n := transform(a)
		if err := r.publish(n); err != nil {
			failed++
			log.Printf("publish %q failed: %v", n.Title, err)
		} else {
			log.Printf("published %q (priority %s)", n.Title, n.Priority)
		}
	}
	if failed > 0 {
		// non-2xx makes Alertmanager retry the whole group — acceptable,
		// ntfy dedupes poorly but alerts are idempotent to a human.
		http.Error(w, fmt.Sprintf("%d/%d publishes failed", failed, len(payload.Alerts)),
			http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	r := &relay{
		ntfyURL: envOr("NTFY_URL", "http://ntfy"),
		topic:   envOr("NTFY_TOPIC", "lab-alerts"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/alert", r.handleAlert)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})

	addr := envOr("LISTEN_ADDR", ":8080")
	log.Printf("ntfy-relay listening on %s -> %s/%s", addr, r.ntfyURL, r.topic)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
