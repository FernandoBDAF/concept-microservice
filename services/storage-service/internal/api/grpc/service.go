package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"microservices/services/profile-storage/internal/logger"
	"microservices/services/profile-storage/internal/models"
	"microservices/services/profile-storage/internal/service"
	pb "microservices/services/profile-storage/proto/profile"
)

// Server implements the gRPC profile service
type Server struct {
	pb.UnimplementedProfileServiceServer
	service *service.ProfileService
	log     *zap.Logger
}

// NewServer creates a new gRPC server instance
func NewServer(service *service.ProfileService) *Server {
	return &Server{
		service: service,
		log:     logger.Get(),
	}
}

// CreateProfile implements the CreateProfile gRPC method
func (s *Server) CreateProfile(ctx context.Context, req *pb.CreateProfileRequest) (*pb.Profile, error) {
	startTime := time.Now()
	s.log.Info("Creating profile via gRPC",
		logger.String("email", req.Email),
	)

	profileReq := &models.ProfileRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
	}

	// Convert addresses
	for _, addr := range req.Addresses {
		profileReq.Addresses = append(profileReq.Addresses, models.Address{
			Street:     addr.Street,
			City:       addr.City,
			State:      addr.State,
			Country:    addr.Country,
			PostalCode: addr.PostalCode,
			IsPrimary:  addr.IsPrimary,
		})
	}

	// Convert contacts
	for _, contact := range req.Contacts {
		profileReq.Contacts = append(profileReq.Contacts, models.Contact{
			Type:      contact.Type,
			Value:     contact.Value,
			IsPrimary: contact.IsPrimary,
		})
	}

	profile, err := s.service.CreateProfile(ctx, profileReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			s.log.Error("Invalid profile request",
				logger.ErrorField(err),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrDuplicateEmail):
			s.log.Error("Email already in use",
				logger.ErrorField(err),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case errors.Is(err, service.ErrTimeout):
			s.log.Error("Profile creation timed out",
				logger.ErrorField(err),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		default:
			s.log.Error("Failed to create profile",
				logger.ErrorField(err),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.Info("Successfully created profile via gRPC",
		logger.String("profile_id", profile.ID.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
	)

	return convertToProtoProfile(profile), nil
}

// GetProfile implements the GetProfile gRPC method
func (s *Server) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.Profile, error) {
	startTime := time.Now()
	profileID := uuid.MustParse(req.Id)
	s.log.Debug("Getting profile via gRPC",
		logger.String("profile_id", profileID.String()),
	)

	profile, err := s.service.GetProfile(ctx, profileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			s.log.Debug("Profile not found",
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, service.ErrTimeout):
			s.log.Error("Profile retrieval timed out",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		default:
			s.log.Error("Failed to get profile",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.Debug("Successfully retrieved profile via gRPC",
		logger.String("profile_id", profile.ID.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
	)

	return convertToProtoProfile(profile), nil
}

// UpdateProfile implements the UpdateProfile gRPC method
func (s *Server) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.Profile, error) {
	startTime := time.Now()
	profileID := uuid.MustParse(req.Id)
	s.log.Info("Updating profile via gRPC",
		logger.String("profile_id", profileID.String()),
		logger.String("email", req.Email),
	)

	profileReq := &models.ProfileRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
	}

	// Convert addresses
	for _, addr := range req.Addresses {
		profileReq.Addresses = append(profileReq.Addresses, models.Address{
			Street:     addr.Street,
			City:       addr.City,
			State:      addr.State,
			Country:    addr.Country,
			PostalCode: addr.PostalCode,
			IsPrimary:  addr.IsPrimary,
		})
	}

	// Convert contacts
	for _, contact := range req.Contacts {
		profileReq.Contacts = append(profileReq.Contacts, models.Contact{
			Type:      contact.Type,
			Value:     contact.Value,
			IsPrimary: contact.IsPrimary,
		})
	}

	profile, err := s.service.UpdateProfile(ctx, profileID, profileReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			s.log.Error("Invalid profile update request",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrProfileNotFound):
			s.log.Debug("Profile not found for update",
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, service.ErrDuplicateEmail):
			s.log.Error("Email already in use",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case errors.Is(err, service.ErrTimeout):
			s.log.Error("Profile update timed out",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		default:
			s.log.Error("Failed to update profile",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
				logger.String("email", req.Email),
			)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.Info("Successfully updated profile via gRPC",
		logger.String("profile_id", profile.ID.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
	)

	return convertToProtoProfile(profile), nil
}

// DeleteProfile implements the DeleteProfile gRPC method
func (s *Server) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest) (*pb.DeleteProfileResponse, error) {
	startTime := time.Now()
	profileID := uuid.MustParse(req.Id)
	s.log.Info("Deleting profile via gRPC",
		logger.String("profile_id", profileID.String()),
	)

	err := s.service.DeleteProfile(ctx, profileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			s.log.Debug("Profile not found for deletion",
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, service.ErrTimeout):
			s.log.Error("Profile deletion timed out",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		default:
			s.log.Error("Failed to delete profile",
				logger.ErrorField(err),
				logger.String("profile_id", profileID.String()),
			)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.Info("Successfully deleted profile via gRPC",
		logger.String("profile_id", profileID.String()),
		logger.Duration("duration", time.Since(startTime)),
	)

	return &pb.DeleteProfileResponse{}, nil
}

// convertToProtoProfile converts a domain profile to a protobuf profile
func convertToProtoProfile(profile *models.Profile) *pb.Profile {
	if profile == nil {
		return nil
	}

	protoProfile := &pb.Profile{
		Id:        profile.ID.String(),
		FirstName: profile.FirstName,
		LastName:  profile.LastName,
		Email:     profile.Email,
		Phone:     profile.Phone,
		CreatedAt: profile.CreatedAt.Unix(),
		UpdatedAt: profile.UpdatedAt.Unix(),
	}

	// Convert addresses
	for _, addr := range profile.Addresses {
		protoProfile.Addresses = append(protoProfile.Addresses, &pb.Address{
			Id:         addr.ID.String(),
			Street:     addr.Street,
			City:       addr.City,
			State:      addr.State,
			Country:    addr.Country,
			PostalCode: addr.PostalCode,
			IsPrimary:  addr.IsPrimary,
			CreatedAt:  addr.CreatedAt.Unix(),
			UpdatedAt:  addr.UpdatedAt.Unix(),
		})
	}

	// Convert contacts
	for _, contact := range profile.Contacts {
		protoProfile.Contacts = append(protoProfile.Contacts, &pb.Contact{
			Id:        contact.ID.String(),
			Type:      contact.Type,
			Value:     contact.Value,
			IsPrimary: contact.IsPrimary,
			CreatedAt: contact.CreatedAt.Unix(),
			UpdatedAt: contact.UpdatedAt.Unix(),
		})
	}

	return protoProfile
}
