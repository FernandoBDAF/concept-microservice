// controld is the read-only seed of lab-controld (ADR-005.1/.2).
//
// STRICTLY read-only in this phase: it only observes the compose and kind
// targets (docker compose ps, kubectl get pods, HTTP health probes) and
// serves summaries as JSON. It performs no control actions of any kind.
// Binds 127.0.0.1 only (ADR-005.4).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultAddr    = "127.0.0.1:4900"
	execTimeout    = 10 * time.Second
	probeTimeout   = 3 * time.Second
	targetCacheTTL = 5 * time.Second

	composeProjectName = "microservices"
	kindClusterName    = "lab"
)

var kindNamespaces = []string{"lab-core", "lab-infra", "lab-obs"}

// allowedOrigins for CORS: the status page UI only.
var allowedOrigins = map[string]bool{
	"http://127.0.0.1:4901": true,
	"http://localhost:4901": true,
}

// Link is one UI link for a target.
type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Note string `json:"note,omitempty"`
}

// linksByTarget is the static per-target links map, data-driven so the UI
// stays target-agnostic.
var linksByTarget = map[string][]Link{
	"compose": {
		{Name: "Grafana", URL: "http://localhost:3001"},
		{Name: "Prometheus", URL: "http://localhost:9090"},
		{Name: "RabbitMQ", URL: "http://localhost:15672"},
		{Name: "MinIO Console", URL: "http://localhost:9001"},
	},
	"kind": {
		{Name: "Grafana", URL: "https://grafana.lab.local"},
		{Name: "OpenSearch", URL: "https://opensearch.lab.local"},
		{Name: "API", URL: "https://api.lab.local"},
		{Name: "RabbitMQ", URL: "https://grafana.lab.local", Note: "via Grafana"},
		{Name: "Tempo", URL: "https://grafana.lab.local", Note: "via Grafana"},
	},
}

// HealthResult is one service's health probe outcome.
type HealthResult struct {
	Service   string  `json:"service"`
	OK        bool    `json:"ok"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

// composeHealthEndpoints are the host-reachable health URLs on the compose
// target. Workers are not port-mapped in compose, so they are not probed.
var composeHealthEndpoints = []struct {
	Service string
	URL     string
}{
	{"api-service", "http://localhost:8080/health"},
	{"auth-service", "http://localhost:3000/health"},
	{"graphrag-service", "http://localhost:8082/health"},
}

// Target is one lab target and whether its runtime is currently reachable.
type Target struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// runner abstracts command execution so handlers can be tested without
// docker/kubectl on the machine.
type runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

type server struct {
	log  *slog.Logger
	run  runner
	http *http.Client

	mu            sync.Mutex
	targetsCache  []Target
	targetsCached time.Time
}

func newServer(log *slog.Logger) *server {
	return &server{
		log:  log,
		run:  execRunner,
		http: &http.Client{Timeout: probeTimeout},
	}
}

func main() {
	addr := flag.String("addr", envOr("CONTROLD_ADDR", defaultAddr), "listen address (keep it on 127.0.0.1)")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := newServer(log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/targets", s.handleTargets)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/links", s.handleLinks)

	log.Info("controld listening", "addr", *addr, "mode", "read-only")
	if err := http.ListenAndServe(*addr, s.middleware(mux)); err != nil {
		log.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// middleware adds CORS headers and request logging.
func (s *server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path,
			"target", r.URL.Query().Get("target"), "duration_ms", time.Since(start).Milliseconds())
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleTargets probes which targets are available, cached for ~5s.
func (s *server) handleTargets(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.targetsCache != nil && time.Since(s.targetsCached) < targetCacheTTL {
		cached := s.targetsCache
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, cached)
		return
	}
	s.mu.Unlock()

	ctx := r.Context()
	composeUp := false
	if out, err := s.run(ctx, "docker", "compose", "ls", "--format", "json"); err == nil {
		composeUp = composeProjectListed(out, composeProjectName)
	} else {
		s.log.Warn("compose ls failed", "error", err.Error())
	}
	kindUp := false
	if out, err := s.run(ctx, "kind", "get", "clusters"); err == nil {
		kindUp = kindClusterListed(out, kindClusterName)
	} else {
		s.log.Warn("kind get clusters failed", "error", err.Error())
	}

	targets := []Target{
		{Name: "compose", Available: composeUp},
		{Name: "kind", Available: kindUp},
	}
	s.mu.Lock()
	s.targetsCache = targets
	s.targetsCached = time.Now()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, targets)
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	switch target := r.URL.Query().Get("target"); target {
	case "compose":
		out, err := s.run(r.Context(), "docker", "compose", "-p", composeProjectName, "ps", "--format", "json")
		if err != nil {
			s.log.Warn("compose ps failed", "error", err.Error())
			writeError(w, http.StatusBadGateway, "docker compose ps failed: "+err.Error())
			return
		}
		services, perr := summarizeComposePS(out)
		if perr != nil {
			writeError(w, http.StatusBadGateway, perr.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"target": "compose", "services": services})
	case "kind":
		workloads := []KindWorkload{}
		for _, ns := range kindNamespaces {
			out, err := s.run(r.Context(), "kubectl", "get", "pods", "-n", ns, "-o", "json")
			if err != nil {
				s.log.Warn("kubectl get pods failed", "namespace", ns, "error", err.Error())
				writeError(w, http.StatusBadGateway, "kubectl get pods -n "+ns+" failed: "+err.Error())
				return
			}
			ws, perr := summarizeKindPods(out, ns)
			if perr != nil {
				writeError(w, http.StatusBadGateway, perr.Error())
				return
			}
			workloads = append(workloads, ws...)
		}
		writeJSON(w, http.StatusOK, map[string]any{"target": "kind", "workloads": workloads})
	default:
		writeError(w, http.StatusBadRequest, "unknown target: use ?target=compose or ?target=kind")
	}
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	switch target := r.URL.Query().Get("target"); target {
	case "compose":
		results := make([]HealthResult, 0, len(composeHealthEndpoints))
		for _, ep := range composeHealthEndpoints {
			results = append(results, s.probe(r.Context(), ep.Service, ep.URL))
		}
		writeJSON(w, http.StatusOK, map[string]any{"target": "compose", "results": results})
	case "kind":
		out, err := s.run(r.Context(), "kubectl", "get", "pods", "-n", "lab-core", "-o", "json")
		if err != nil {
			s.log.Warn("kubectl get pods failed", "namespace", "lab-core", "error", err.Error())
			writeError(w, http.StatusBadGateway, "kubectl get pods -n lab-core failed: "+err.Error())
			return
		}
		results, perr := kindHealthFromPods(out)
		if perr != nil {
			writeError(w, http.StatusBadGateway, perr.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"target": "kind", "results": results})
	default:
		writeError(w, http.StatusBadRequest, "unknown target: use ?target=compose or ?target=kind")
	}
}

// probe issues a single GET against a health endpoint.
func (s *server) probe(ctx context.Context, service, url string) HealthResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthResult{Service: service, OK: false, Error: err.Error()}
	}
	start := time.Now()
	resp, err := s.http.Do(req)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return HealthResult{Service: service, OK: false, LatencyMS: latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	res := HealthResult{Service: service, OK: resp.StatusCode >= 200 && resp.StatusCode < 300, LatencyMS: latency}
	if !res.OK {
		res.Error = "status " + resp.Status
	}
	return res
}

func (s *server) handleLinks(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	links, ok := linksByTarget[target]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown target: use ?target=compose or ?target=kind")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"target": target, "links": links})
}
