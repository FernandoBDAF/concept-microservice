// hello-guest worker — a loop doing a fake "job" every HELLO_JOB_INTERVAL
// (default 2s), instrumented per the host contract (HOST_CONTRACT.md §1.2).
// Its whole purpose is to draw a steady line on the shared dashboards that
// flatlines the moment the container dies (EXP-HG-01).
package main

import (
	"context"
	"log"
	"math/rand"
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

	interval := 2 * time.Second
	if v := os.Getenv("HELLO_JOB_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Fatalf("bad HELLO_JOB_INTERVAL %q: %v", v, err)
		}
		interval = d
	}

	reg := metrics.NewRegistry()
	jobs := reg.Counter("hello_guest_jobs_total",
		"Total fake jobs completed.")
	durations := reg.Histogram("hello_guest_job_duration_seconds",
		"Fake job duration in seconds.",
		[]float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5})
	reg.GaugeFunc("hello_guest_worker_uptime_seconds",
		"Seconds since the worker process started.", func() float64 {
			return time.Since(start).Seconds()
		})

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ready\n"))
	})
	mux.Handle("/metrics", reg.Handler())

	srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("hello-guest worker listening on :8080 (host=%s, interval=%s)", hostname, interval)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
		sig := <-stop
		log.Printf("received %s, finishing current job and shutting down", sig)
		cancel()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			t0 := time.Now()
			// The "job": burn 10–60ms so the duration histogram has shape.
			time.Sleep(time.Duration(10+rand.Intn(50)) * time.Millisecond)
			durations.Observe(time.Since(t0).Seconds())
			jobs.Inc()
		}
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("bye after %d jobs", jobs.Value())
}
