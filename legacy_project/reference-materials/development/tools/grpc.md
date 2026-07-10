# gRPC Usage Guide

## Overview

gRPC is our primary RPC framework for service-to-service communication, providing high-performance, language-agnostic communication between microservices. This guide covers our gRPC implementation, best practices, and common patterns.

## Key Features Used

### 1. Service Definition

We use Protocol Buffers for service definitions:

```protobuf
syntax = "proto3";

package profile.v1;

import "google/protobuf/timestamp.proto";
import "google/api/annotations.proto";

// Profile service definition
service ProfileService {
  // GetProfile retrieves a profile by ID
  rpc GetProfile(GetProfileRequest) returns (Profile) {
    option (google.api.http) = {
      get: "/v1/profiles/{id}"
    };
  }

  // CreateProfile creates a new profile
  rpc CreateProfile(CreateProfileRequest) returns (Profile) {
    option (google.api.http) = {
      post: "/v1/profiles"
      body: "*"
    };
  }

  // UpdateProfile updates an existing profile
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile) {
    option (google.api.http) = {
      put: "/v1/profiles/{id}"
      body: "*"
    };
  }

  // DeleteProfile deletes a profile
  rpc DeleteProfile(DeleteProfileRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      delete: "/v1/profiles/{id}"
    };
  }
}

// Request/Response messages
message GetProfileRequest {
  string id = 1;
}

message CreateProfileRequest {
  string name = 1;
  string email = 2;
  map<string, string> metadata = 3;
}

message UpdateProfileRequest {
  string id = 1;
  string name = 2;
  string email = 3;
  map<string, string> metadata = 4;
}

message DeleteProfileRequest {
  string id = 1;
}

message Profile {
  string id = 1;
  string name = 2;
  string email = 3;
  map<string, string> metadata = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}
```

### 2. Server Implementation

```go
// Server implementation
type ProfileServer struct {
    pb.UnimplementedProfileServiceServer
    repository Repository
    logger     *zap.Logger
}

func NewProfileServer(repo Repository, logger *zap.Logger) *ProfileServer {
    return &ProfileServer{
        repository: repo,
        logger:     logger,
    }
}

func (s *ProfileServer) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.Profile, error) {
    s.logger.Info("Getting profile",
        zap.String("id", req.Id),
        zap.String("request_id", requestid.FromContext(ctx)))

    profile, err := s.repository.Get(ctx, req.Id)
    if err != nil {
        return nil, status.Error(codes.NotFound, "profile not found")
    }

    return &pb.Profile{
        Id:        profile.ID,
        Name:      profile.Name,
        Email:     profile.Email,
        Metadata:  profile.Metadata,
        CreatedAt: timestamppb.New(profile.CreatedAt),
        UpdatedAt: timestamppb.New(profile.UpdatedAt),
    }, nil
}
```

### 3. Client Implementation

```go
// Client implementation
type ProfileClient struct {
    client pb.ProfileServiceClient
    logger *zap.Logger
}

func NewProfileClient(conn *grpc.ClientConn, logger *zap.Logger) *ProfileClient {
    return &ProfileClient{
        client: pb.NewProfileServiceClient(conn),
        logger: logger,
    }
}

func (c *ProfileClient) GetProfile(ctx context.Context, id string) (*Profile, error) {
    c.logger.Info("Requesting profile",
        zap.String("id", id),
        zap.String("request_id", requestid.FromContext(ctx)))

    resp, err := c.client.GetProfile(ctx, &pb.GetProfileRequest{Id: id})
    if err != nil {
        return nil, fmt.Errorf("failed to get profile: %w", err)
    }

    return &Profile{
        ID:        resp.Id,
        Name:      resp.Name,
        Email:     resp.Email,
        Metadata:  resp.Metadata,
        CreatedAt: resp.CreatedAt.AsTime(),
        UpdatedAt: resp.UpdatedAt.AsTime(),
    }, nil
}
```

### 4. Error Handling

```go
// Error handling middleware
func ErrorHandlingInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        resp, err := handler(ctx, req)
        if err == nil {
            return resp, nil
        }

        // Convert domain errors to gRPC status
        switch {
        case errors.Is(err, ErrNotFound):
            return nil, status.Error(codes.NotFound, err.Error())
        case errors.Is(err, ErrInvalidInput):
            return nil, status.Error(codes.InvalidArgument, err.Error())
        case errors.Is(err, ErrUnauthorized):
            return nil, status.Error(codes.PermissionDenied, err.Error())
        default:
            return nil, status.Error(codes.Internal, "internal server error")
        }
    }
}
```

## Best Practices

1. **Service Design**

   - Use clear, descriptive service names
   - Follow RESTful principles
   - Version your services
   - Document your APIs

2. **Message Design**

   - Use meaningful field names
   - Include proper field numbers
   - Add field comments
   - Use appropriate types

3. **Error Handling**

   - Use proper status codes
   - Include error details
   - Handle timeouts
   - Implement retries

4. **Performance**

   - Use streaming when appropriate
   - Implement proper timeouts
   - Use connection pooling
   - Monitor performance

## Common Issues and Solutions

1. **Connection Issues**

   - Problem: Connection failures
   - Solution: Implement retry logic, use connection pooling

2. **Timeout Issues**

   - Problem: Requests timing out
   - Solution: Set appropriate timeouts, implement circuit breakers

3. **Version Compatibility**
   - Problem: Breaking changes
   - Solution: Use proper versioning, maintain backward compatibility

## Examples from Our Project

### Service Registration

```go
func RegisterServices(server *grpc.Server, deps *Dependencies) {
    pb.RegisterProfileServiceServer(server, NewProfileServer(deps.Repository, deps.Logger))
    pb.RegisterAuthServiceServer(server, NewAuthServer(deps.AuthService, deps.Logger))
}
```

### Client Configuration

```go
func NewClientConn(ctx context.Context, target string) (*grpc.ClientConn, error) {
    return grpc.DialContext(ctx, target,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
        grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
    )
}
```

## References

- [gRPC Official Documentation](https://grpc.io/docs/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers/docs/overview)
- [gRPC Best Practices](https://grpc.io/docs/guides/best-practices/)
- [gRPC Error Handling](https://grpc.io/docs/guides/error/)
