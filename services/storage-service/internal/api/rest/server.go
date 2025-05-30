package rest

import (
	"fmt"
	"log"
	"net/http"

	"microservices/services/profile-storage/internal/config"

	"github.com/gorilla/mux"
)

// Server represents the REST API server
type Server struct {
	config *config.Config
	mux    *http.ServeMux
}

// NewServer creates a new REST server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
		mux:    http.NewServeMux(),
	}
}

// RegisterRoutes registers all API routes
func (s *Server) RegisterRoutes(handlers ...interface{}) {
	router := mux.NewRouter()
	for _, handler := range handlers {
		switch h := handler.(type) {
		case *ProfileHandler:
			h.RegisterRoutes(router)
		case *HealthHandler:
			h.RegisterRoutes(router)
		case *MetricsHandler:
			h.RegisterRoutes(router)
		case *Handler:
			h.RegisterRoutes(router)
		}
	}
	s.mux.Handle("/", router)
}

// Start starts the REST server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.config.ServerPort)
	log.Printf("Starting REST server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}
