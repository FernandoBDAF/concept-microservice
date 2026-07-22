package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"example.com/api-publisher/internal/auth"
	"example.com/api-publisher/internal/config"
	"example.com/api-publisher/internal/task"
)

// NewServer builds the public HTTP server. It exposes exactly three routes:
//
//	POST /tasks    — submit a task (JWKS-authenticated); 202 + {task_id}
//	GET  /healthz  — liveness + DB readiness (pings postgres)
//	GET  /metrics  — Prometheus metrics (includes the outbox gauges/counters)
//
// The method+path patterns require Go 1.22+ (ServeMux routing).
func NewServer(cfg config.ServerConfig, authCfg config.AuthConfig, verifier *auth.JWKSVerifier, taskService *task.Service, db *sqlx.DB, log *zap.Logger) *http.Server {
	mux := http.NewServeMux()

	th := NewTaskHandler(taskService)
	authMW := LocalAuth(verifier, log)
	if authCfg.Disabled {
		authMW = DevAuthBypass(log)
	}
	mux.Handle("POST /tasks", authMW(http.HandlerFunc(th.SubmitTask)))

	mux.HandleFunc("GET /healthz", healthz(db))
	mux.Handle("GET /metrics", promhttp.Handler())

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}
}

// healthz reports 200 when postgres answers a ping, 503 otherwise. Keep
// liveness cheap; add your broker check here if you want readiness to gate on
// it too (the outbox tolerates a broker outage — rows just stay pending).
func healthz(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status": "degraded",
				"checks": map[string]string{"postgres": "down"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
			"checks": map[string]string{"postgres": "ok"},
		})
	}
}
