// hello-guest web — the smallest thing that satisfies the host contract
// (documentation/HOST_CONTRACT.md §1.2): /, /health, /ready, /metrics.
// Stdlib only, on purpose.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fernandobarroso/microservices/guests/hello-guest/internal/metrics"
)

func main() {
	start := time.Now()
	hostname, _ := os.Hostname()

	reg := metrics.NewRegistry()
	requests := reg.Counter("hello_guest_web_requests_total",
		"Total HTTP requests served (all paths).")
	reg.GaugeFunc("hello_guest_web_uptime_seconds",
		"Seconds since the web process started.", func() float64 {
			return time.Since(start).Seconds()
		})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message":        "hello from guest",
			"guest":          "hello-guest",
			"hostname":       hostname,
			"uptime_seconds": int64(time.Since(start).Seconds()),
		})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	// No external dependencies, so ready == alive. A real guest checks its
	// backends here (HOST_CONTRACT.md §1.2).
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ready\n"))
	})
	mux.Handle("/metrics", reg.Handler())

	srv := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Inc()
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("hello-guest web listening on :8080 (host=%s)", hostname)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	sig := <-stop
	log.Printf("received %s, shutting down", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Print("bye")
}
