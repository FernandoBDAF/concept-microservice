package base

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPServer provides health check endpoints for workers
type HTTPServer struct {
	router *gin.Engine
	port   string
	mu     sync.RWMutex
	ready  bool
	http   *http.Server
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(port string) *HTTPServer {
	// gin.New instead of gin.Default: Prometheus hits /metrics every scrape
	// interval, and access-logging that would drown the worker's real logs.
	router := gin.New()
	router.Use(gin.Recovery())
	s := &HTTPServer{
		router: router,
		ready:  false,
		port:   port,
	}

	// Register routes
	s.registerRoutes()

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", s.port),
		Handler: router,
	}
	s.http = httpServer

	return s
}

// registerRoutes registers all HTTP routes
func (s *HTTPServer) registerRoutes() {
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/ready", s.readinessCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// healthCheck handles the health check endpoint
func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"ready":  s.isReady(),
	})
}

// readinessCheck handles the readiness check endpoint
func (s *HTTPServer) readinessCheck(c *gin.Context) {
	if s.isReady() {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"status": "not ready",
	})
}

// SetReady sets the ready state of the server
func (s *HTTPServer) SetReady(ready bool) {
	s.mu.Lock()
	s.ready = ready
	s.mu.Unlock()
}

// isReady returns the current ready state
func (s *HTTPServer) isReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// Start starts the HTTP server
func (s *HTTPServer) Start(ctx context.Context) error {
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.SetReady(false)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
