package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/FBDAF/microservices/services/common/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create a new registry
	registry := metrics.NewRegistry()

	// Create metrics
	requestCounter := metrics.NewCounter(
		"http_requests_total",
		"Total number of HTTP requests",
		[]string{"method", "path"},
	)

	requestDuration := metrics.NewTimer(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		[]string{"method", "path"},
	)

	responseSize := metrics.NewHistogram(
		"http_response_size_bytes",
		"HTTP response size in bytes",
		[]string{"method", "path"},
		[]float64{100, 1000, 10000, 100000},
	)

	activeConnections := metrics.NewGauge(
		"http_active_connections",
		"Number of active HTTP connections",
		[]string{"method", "path"},
	)

	// Register metrics with Prometheus
	registry.RegisterCounter(requestCounter)
	registry.RegisterTimer(requestDuration)
	registry.RegisterHistogram(responseSize)
	registry.RegisterGauge(activeConnections)

	// Register collectors with Prometheus
	for _, collector := range registry.Collectors() {
		prometheus.MustRegister(collector)
	}

	// Create a simple HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Record request
		requestCounter.Inc()

		// Start timer
		timer := requestDuration.Start()
		defer timer.Stop()

		// Simulate some work
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

		// Record response size
		size := rand.Intn(10000)
		responseSize.Observe(float64(size))

		// Update active connections
		activeConnections.Inc()
		defer activeConnections.Dec()

		// Send response
		fmt.Fprintf(w, "Hello, World! Response size: %d bytes\n", size)
	})

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Start server
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
