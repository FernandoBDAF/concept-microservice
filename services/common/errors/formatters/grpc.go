package formatters

import (
	"context"

	"github.com/FBDAF/microservices/services/common/errors/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// FormatGRPCError formats an error into a gRPC status
func FormatGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// If it's already a gRPC status, return it
	if _, ok := status.FromError(err); ok {
		return err
	}

	// Default to internal error
	code := codes.Internal
	message := err.Error()

	// If it's our custom error type, map it to gRPC codes
	if customErr, ok := err.(types.Error); ok {
		message = customErr.Error()

		// Map error codes to gRPC codes
		switch customErr.Code() {
		case types.ErrCodeValidation:
			code = codes.InvalidArgument
		case types.ErrCodeNotFound:
			code = codes.NotFound
		case types.ErrCodeUnauthorized:
			code = codes.Unauthenticated
		case types.ErrCodeForbidden:
			code = codes.PermissionDenied
		case types.ErrCodeTimeout:
			code = codes.DeadlineExceeded
		case types.ErrCodeConflict:
			code = codes.AlreadyExists
		}

		// Add trace ID to error details if available
		if traceID := customErr.TraceID(); traceID != "" {
			details := map[string]interface{}{
				"trace_id": traceID,
				"severity": string(customErr.Severity()),
			}
			detailsProto, err := structpb.NewStruct(details)
			if err != nil {
				return status.Error(code, message)
			}
			st := status.New(code, message)
			st, err = st.WithDetails(detailsProto)
			if err != nil {
				return status.Error(code, message)
			}
			return st.Err()
		}
	}

	return status.Error(code, message)
}

// FromGRPCError converts a gRPC error to our custom error type
func FromGRPCError(err error) types.Error {
	if err == nil {
		return nil
	}

	// If it's already our custom error type, return it
	if customErr, ok := err.(types.Error); ok {
		return customErr
	}

	// Convert gRPC status to our error type
	if st, ok := status.FromError(err); ok {
		var code types.ErrorCode
		var severity types.ErrorSeverity = types.SeverityMedium

		// Map gRPC codes to our error codes
		switch st.Code() {
		case codes.InvalidArgument:
			code = types.ErrCodeValidation
		case codes.NotFound:
			code = types.ErrCodeNotFound
		case codes.Unauthenticated:
			code = types.ErrCodeUnauthorized
		case codes.PermissionDenied:
			code = types.ErrCodeForbidden
		case codes.DeadlineExceeded:
			code = types.ErrCodeTimeout
		case codes.AlreadyExists:
			code = types.ErrCodeConflict
		default:
			code = types.ErrCodeInternal
			severity = types.SeverityHigh
		}

		// Create base error
		baseErr := types.New(code, st.Message()).WithSeverity(severity)

		// Extract trace ID from error details if available
		if len(st.Details()) > 0 {
			if details, ok := st.Details()[0].(map[string]string); ok {
				if traceID, ok := details["trace_id"]; ok {
					baseErr = baseErr.WithTraceID(traceID)
				}
				if sev, ok := details["severity"]; ok {
					baseErr = baseErr.WithSeverity(types.ErrorSeverity(sev))
				}
			}
		}

		return baseErr
	}

	// For unknown errors, create an internal error
	return types.New(types.ErrCodeInternal, err.Error()).WithSeverity(types.SeverityHigh)
}

// GRPCErrorInterceptor is a gRPC interceptor that formats errors
func GRPCErrorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		return nil, FormatGRPCError(err)
	}
	return resp, nil
}
