package main

// auth.go — the ADR-005.4 / EXP-63 auth gate.
//
// Two modes, chosen by environment:
//   - no CONTROLD_TOKEN  → localhost no-auth mode (current v3 behavior).
//   - CONTROLD_TOKEN set  → every /api/* request must carry
//     Authorization: Bearer <token> (or ?token= for SSE, since EventSource
//     cannot set headers); mismatch → 401 + an audit slog line.
//
// The aws target additionally requires token + TLS (ValidateStartup enforces
// it): remote aws control must never run unauthenticated or in cleartext.

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Config is the runtime configuration the engine and auth gate consult. It is
// assembled by the orchestrator in main.go from flags/env (see
// INTEGRATION-NOTES-A.md); ConfigFromEnv builds the env-only portion.
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
			if got == "" {
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

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(h, prefix))
	}
	return ""
}

// AWSTargetEntry is the /api/targets row for the aws target when it is
// enabled. main.go owns /api/targets; the orchestrator appends this entry to
// its response when cfg.EnableAWS (see INTEGRATION-NOTES-A.md). It is a map
// rather than the main.go Target struct so it can carry the extra "note"
// field without altering that read-only type.
func AWSTargetEntry() map[string]any {
	return map[string]any{
		"name":      "aws",
		"available": false,
		"note":      "session check pending v5 integration",
	}
}
