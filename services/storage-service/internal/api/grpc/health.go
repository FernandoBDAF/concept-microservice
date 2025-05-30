package grpc

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"microservices/services/profile-storage/internal/logger"
)

// HealthServer implements the gRPC health check service
type HealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	db  *sql.DB
	log *zap.Logger
}

// NewHealthServer creates a new health check server
func NewHealthServer(db *sql.DB) *HealthServer {
	return &HealthServer{
		db:  db,
		log: logger.Get(),
	}
}

// Check implements the health check service
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	startTime := time.Now()
	s.log.Debug("Health check requested",
		logger.String("service", req.Service),
	)

	// Check database connection with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		s.log.Error("Health check failed",
			logger.ErrorField(err),
			logger.String("service", req.Service),
			logger.Duration("duration", time.Since(startTime)),
		)
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	s.log.Debug("Health check successful",
		logger.String("service", req.Service),
		logger.Duration("duration", time.Since(startTime)),
	)

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch implements the health check watch service
func (s *HealthServer) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	s.log.Warn("Health watch requested but not implemented",
		logger.String("service", req.Service),
	)
	return status.Error(codes.Unimplemented, "watch not implemented")
}
