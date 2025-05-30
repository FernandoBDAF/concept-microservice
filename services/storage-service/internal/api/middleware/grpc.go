package middleware

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"microservices/services/profile-storage/internal/logger"
)

const (
	// RequestIDMetadataKey is the metadata key for the request ID
	RequestIDMetadataKey = "x-request-id"
)

// UnaryLoggingInterceptor creates a gRPC unary interceptor that logs requests and responses
func UnaryLoggingInterceptor() grpc.UnaryServerInterceptor {
	log := logger.Get()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Get request ID from context
		requestID := getRequestIDFromContext(ctx)

		// Log request
		log.Info("gRPC request started",
			logger.String("method", info.FullMethod),
			logger.String("request_id", requestID),
		)

		// Process request
		resp, err := handler(ctx, req)

		// Log response
		duration := time.Since(startTime)
		if err != nil {
			st, _ := status.FromError(err)
			log.Error("gRPC request failed",
				logger.String("method", info.FullMethod),
				logger.String("error", err.Error()),
				logger.String("code", st.Code().String()),
				logger.Duration("duration", duration),
				logger.String("request_id", requestID),
			)
		} else {
			log.Info("gRPC request completed",
				logger.String("method", info.FullMethod),
				logger.Duration("duration", duration),
				logger.String("request_id", requestID),
			)
		}

		return resp, err
	}
}

// StreamLoggingInterceptor creates a gRPC stream interceptor that logs stream operations
func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
	log := logger.Get()
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()

		// Get request ID from context
		requestID := getRequestIDFromContext(ss.Context())

		// Log stream start
		log.Info("gRPC stream started",
			logger.String("method", info.FullMethod),
			logger.String("request_id", requestID),
		)

		// Process stream
		err := handler(srv, ss)

		// Log stream end
		duration := time.Since(startTime)
		if err != nil {
			st, _ := status.FromError(err)
			log.Error("gRPC stream failed",
				logger.String("method", info.FullMethod),
				logger.String("error", err.Error()),
				logger.String("code", st.Code().String()),
				logger.Duration("duration", duration),
				logger.String("request_id", requestID),
			)
		} else {
			log.Info("gRPC stream completed",
				logger.String("method", info.FullMethod),
				logger.Duration("duration", duration),
				logger.String("request_id", requestID),
			)
		}

		return err
	}
}

// RecoveryInterceptor creates a gRPC interceptor that recovers from panics and logs them
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	log := logger.Get()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("gRPC request panic recovered",
					logger.String("panic", err.(string)),
					logger.String("method", info.FullMethod),
				)
			}
		}()

		return handler(ctx, req)
	}
}

// RequestIDInterceptor creates a gRPC interceptor that adds a request ID to the context
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get request ID from metadata or generate new one
		requestID := getRequestIDFromContext(ctx)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add request ID to outgoing metadata
		md := metadata.New(map[string]string{
			RequestIDMetadataKey: requestID,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		return handler(ctx, req)
	}
}

// TimeoutInterceptor creates a gRPC interceptor that adds a timeout to the request context
func TimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return handler(ctx, req)
	}
}

// getRequestIDFromContext extracts the request ID from the context
func getRequestIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "unknown"
	}

	requestIDs := md.Get(RequestIDMetadataKey)
	if len(requestIDs) == 0 {
		return "unknown"
	}

	return requestIDs[0]
}
