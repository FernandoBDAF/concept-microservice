package main

// auth.go — the ADR-005.4 / EXP-63 auth gate.
//
// Two modes, chosen by environment:
//   - no CONTROLD_TOKEN  → localhost no-auth mode (current v3 behavior).
//   - CONTROLD_TOKEN set  → every /api/* request must carry
//     Authorization: Bearer <token>; the SSE stream route alone also accepts
//     ?token= (EventSource cannot set headers); mismatch → 401 + an audit
//     slog line.
//
// The aws target additionally requires token + TLS (ValidateStartup enforces
// it): remote aws control must never run unauthenticated or in cleartext.

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config is the runtime configuration the engine and auth gate consult. It is
// assembled by the orchestrator in main.go from flags/env (see
// mission-control/README.md); ConfigFromEnv builds the env-only portion.
type Config struct {
	RepoRoot  string // absolute; commands exec with Dir = RepoRoot
	Token     string // CONTROLD_TOKEN; "" → no-auth localhost mode
	EnableAWS bool   // CONTROLD_ENABLE_AWS=1 unlocks the aws target
	TLSCert   string // CONTROLD_TLS_CERT
	TLSKey    string // CONTROLD_TLS_KEY
}

// ConfigFromEnv assembles a Config from environment variables, resolving the
// repo root (default "../.." relative to the controld working dir) to an
// absolute path. The orchestrator may override RepoRoot from a -repo-root flag.
func ConfigFromEnv() Config {
	return Config{
		RepoRoot:  ResolveRepoRoot(envOr("CONTROLD_REPO_ROOT", "../..")),
		Token:     os.Getenv("CONTROLD_TOKEN"),
		EnableAWS: os.Getenv("CONTROLD_ENABLE_AWS") == "1",
		TLSCert:   os.Getenv("CONTROLD_TLS_CERT"),
		TLSKey:    os.Getenv("CONTROLD_TLS_KEY"),
	}
}

// ResolveRepoRoot turns a possibly-relative repo root into an absolute path,
// falling back to the input unchanged if resolution fails.
func ResolveRepoRoot(v string) string {
	if abs, err := filepath.Abs(v); err == nil {
		return abs
	}
	return v
}

// ValidateStartup fails fast when the aws target is enabled without the
// token + TLS it requires. Localhost/no-aws mode needs neither.
func ValidateStartup(cfg Config) error {
	if !cfg.EnableAWS {
		return nil
	}
	if cfg.Token == "" {
		return errors.New("CONTROLD_ENABLE_AWS=1 requires CONTROLD_TOKEN to be set")
	}
	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		return errors.New("CONTROLD_ENABLE_AWS=1 requires CONTROLD_TLS_CERT and CONTROLD_TLS_KEY to be set")
	}
	return nil
}

// AuthMiddleware guards /api/* when a token is configured. It is a plain
// func(http.Handler) http.Handler the orchestrator chains ahead of the mux.
func AuthMiddleware(cfg Config, log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	want := []byte(cfg.Token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No token configured, or a non-API path (e.g. /healthz, UI):
			// pass through unauthenticated.
			if cfg.Token == "" || !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			got := bearerToken(r)
			if got == "" && isStreamPath(r) {
				got = r.URL.Query().Get("token") // SSE via EventSource
			}
			if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				log.Warn("unauthorized",
					"remote", r.RemoteAddr, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isStreamPath reports whether the request targets the SSE stream route
// (GET /api/actions/{id}/stream, see main.go's mux) — the ONLY endpoint where
// the ?token= query fallback is accepted, because EventSource cannot set
// headers. Every other /api endpoint requires the Bearer header.
func isStreamPath(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.HasPrefix(r.URL.Path, "/api/actions/") &&
		strings.HasSuffix(r.URL.Path, "/stream")
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(h, prefix))
	}
	return ""
}

// AWSTargetEntry is the /api/targets row for the aws target when it is enabled.
// main.go owns /api/targets; the orchestrator appends this entry to its response
// when cfg.EnableAWS (see mission-control/README.md). It is a map rather than the
// main.go Target struct so it can carry the extra "note" field without altering
// that read-only type.
//
// Availability is a cheap read-only probe (v6-HANDOFF §4): the aws target is
// "available" only when a terraform session is up. The probe result is cached
// for awsProbeTTL so a flurry of /api/targets polls does not shell out on every
// request. A probe failure is NEVER surfaced as an error to the client — it is
// reported as {available:false, note:<short reason>}. The JSON shape stays
// exactly {"name","available","note"} — the UI depends on it.
func AWSTargetEntry(cfg Config) map[string]any {
	awsProbe.mu.Lock()
	defer awsProbe.mu.Unlock()
	if awsProbe.entry != nil && time.Since(awsProbe.at) < awsProbeTTL {
		return awsProbe.entry
	}
	entry := probeAWS(cfg)
	awsProbe.entry = entry
	awsProbe.at = time.Now()
	return entry
}

// awsProbeTTL bounds how long a probe result is reused.
const awsProbeTTL = 60 * time.Second

// awsProbe caches the last availability probe. The zero value is a cold cache.
var awsProbe struct {
	mu    sync.Mutex
	entry map[string]any
	at    time.Time
}

// awsProbeRun runs the read-only session probe. It is a package-level var so
// tests can inject a fake without a terraform binary or live AWS. It returns the
// raw stdout (the cluster name) and an error (with a short, honest message).
var awsProbeRun = func(ctx context.Context, repoRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "terraform", "-chdir=deploy/aws/session", "output", "-raw", "cluster_name")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := firstLine(stderr.String()); msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return string(out), nil
}

// probeAWS runs (once) the terraform session probe with a 10s budget and maps
// the outcome to the /api/targets entry. Exit 0 + non-empty output → available;
// anything else → unavailable with a short reason.
func probeAWS(cfg Config) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := awsProbeRun(ctx, cfg.RepoRoot)
	if err != nil {
		return awsEntry(false, "session down: "+short(err.Error()))
	}
	name := strings.TrimSpace(out)
	if name == "" {
		return awsEntry(false, "session down: no cluster_name output")
	}
	return awsEntry(true, "session up: cluster "+name)
}

// awsEntry builds the fixed {"name","available","note"} shape.
func awsEntry(available bool, note string) map[string]any {
	return map[string]any{"name": "aws", "available": available, "note": note}
}

// resetAWSProbeCache clears the cached probe result (used by tests).
func resetAWSProbeCache() {
	awsProbe.mu.Lock()
	defer awsProbe.mu.Unlock()
	awsProbe.entry = nil
	awsProbe.at = time.Time{}
}

// firstLine returns the first non-empty trimmed line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// short trims and caps a reason string so the note stays terse.
func short(s string) string {
	s = strings.TrimSpace(s)
	const max = 140
	if len(s) > max {
		return s[:max-1] + "…"
	}
	if s == "" {
		return "unavailable"
	}
	return s
}
