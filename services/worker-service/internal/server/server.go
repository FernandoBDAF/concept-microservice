package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fernandobarroso/worker-service/internal/adapters/queue"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router   *gin.Engine
	port     string
	mu       sync.RWMutex
	ready    bool
	consumer *queue.Consumer
	http     *http.Server
}

// NewServer creates a new HTTP server instance
func NewServer(consumer *queue.Consumer, port string) *Server {
	router := gin.Default()
	s := &Server{
		router:   router,
		ready:    false,
		consumer: consumer,
		port:     port,
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
func (s *Server) registerRoutes() {
	s.router.GET("/health", s.healthCheck)
}

// healthCheck handles the health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	s.mu.RLock()
	ready := s.ready
	s.mu.RUnlock()

	if ready {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"ready":  true,
		})
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"status": "unhealthy",
		"ready":  false,
	})
}

// SetReady sets the ready state of the server
func (s *Server) SetReady(ready bool) {
	s.mu.Lock()
	s.ready = ready
	s.mu.Unlock()
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Start HTTP server
	go func() {
		s.SetReady(true)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.SetReady(false)
			panic(err)
		}
	}()

	// Start consumer
	return s.consumer.Start(ctx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return err
	}

	// Close consumer
	return s.consumer.Close()
}
