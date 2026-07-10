package rest

import (
	"fmt"
	"log"
	"net/http"

	"microservices/services/profile-storage/internal/config"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the REST API server
type Server struct {
	config *config.Config
	router *mux.Router
}

// NewServer creates a new REST server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
		router: mux.NewRouter(),
	}
}

// RegisterRoutes registers all API routes
func (s *Server) RegisterRoutes(profileHandler *ProfileHandler, batchHandler *BatchHandler, healthHandler *HealthHandler) {
	// Health check
	s.router.HandleFunc("/health", healthHandler.handleHealth).Methods(http.MethodGet)

	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Profile routes
	profileHandler.RegisterRoutes(api)

	// Batch routes
	batchHandler.RegisterRoutes(api)

	// Metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())
}

// Start starts the REST server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.config.ServerPort)
	log.Printf("Starting REST server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}
