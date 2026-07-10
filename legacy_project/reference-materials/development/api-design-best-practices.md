# API Design Best Practices

## Overview

This document outlines the best practices for API design in our microservices architecture, covering both RESTful HTTP APIs and gRPC services. It provides guidelines for endpoint design, request/response handling, versioning, and documentation.

## RESTful API Design

### 1. Resource Naming

```go
// Good resource naming
GET    /api/v1/profiles          // List profiles
GET    /api/v1/profiles/{id}     // Get profile by ID
POST   /api/v1/profiles          // Create profile
PUT    /api/v1/profiles/{id}     // Update profile
DELETE /api/v1/profiles/{id}     // Delete profile

// Sub-resources
GET    /api/v1/profiles/{id}/preferences
PUT    /api/v1/profiles/{id}/preferences

// Actions
POST   /api/v1/profiles/{id}/verify
POST   /api/v1/profiles/{id}/deactivate
```

### 2. Request/Response Structure

```go
// Request structure
type CreateProfileRequest struct {
    FirstName string `json:"firstName" binding:"required"`
    LastName  string `json:"lastName" binding:"required"`
    Email     string `json:"email" binding:"required,email"`
    Phone     string `json:"phone" binding:"omitempty,e164"`
}

// Response structure
type ProfileResponse struct {
    ID        string    `json:"id"`
    FirstName string    `json:"firstName"`
    LastName  string    `json:"lastName"`
    Email     string    `json:"email"`
    Phone     string    `json:"phone,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}

// Error response
type ErrorResponse struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}
```

### 3. Query Parameters

```go
// Pagination
GET /api/v1/profiles?page=1&limit=20

// Filtering
GET /api/v1/profiles?status=active&type=premium

// Sorting
GET /api/v1/profiles?sort=createdAt&order=desc

// Field selection
GET /api/v1/profiles?fields=id,firstName,lastName
```

## gRPC Service Design

### 1. Service Definition

```protobuf
syntax = "proto3";

package profile.v1;

import "google/protobuf/timestamp.proto";
import "google/api/annotations.proto";

service ProfileService {
    // Create a new profile
    rpc CreateProfile(CreateProfileRequest) returns (Profile) {
        option (google.api.http) = {
            post: "/api/v1/profiles"
            body: "*"
        };
    }

    // Get profile by ID
    rpc GetProfile(GetProfileRequest) returns (Profile) {
        option (google.api.http) = {
            get: "/api/v1/profiles/{id}"
        };
    }

    // Update profile
    rpc UpdateProfile(UpdateProfileRequest) returns (Profile) {
        option (google.api.http) = {
            put: "/api/v1/profiles/{id}"
            body: "*"
        };
    }

    // Delete profile
    rpc DeleteProfile(DeleteProfileRequest) returns (google.protobuf.Empty) {
        option (google.api.http) = {
            delete: "/api/v1/profiles/{id}"
        };
    }
}

message Profile {
    string id = 1;
    string first_name = 2;
    string last_name = 3;
    string email = 4;
    optional string phone = 5;
    google.protobuf.Timestamp created_at = 6;
    google.protobuf.Timestamp updated_at = 7;
}
```

### 2. Request/Response Messages

```protobuf
message CreateProfileRequest {
    string first_name = 1;
    string last_name = 2;
    string email = 3;
    optional string phone = 4;
}

message GetProfileRequest {
    string id = 1;
}

message UpdateProfileRequest {
    string id = 1;
    optional string first_name = 2;
    optional string last_name = 3;
    optional string email = 4;
    optional string phone = 5;
}

message DeleteProfileRequest {
    string id = 1;
}
```

## API Versioning

### 1. URL Versioning

```go
// URL path versioning
GET /api/v1/profiles
GET /api/v2/profiles

// Implementation
func (s *Server) RegisterRoutes(r *gin.Engine) {
    v1 := r.Group("/api/v1")
    {
        v1.GET("/profiles", s.ListProfilesV1)
        v1.GET("/profiles/:id", s.GetProfileV1)
    }

    v2 := r.Group("/api/v2")
    {
        v2.GET("/profiles", s.ListProfilesV2)
        v2.GET("/profiles/:id", s.GetProfileV2)
    }
}
```

### 2. Header Versioning

```go
// Version header
Accept: application/vnd.profile.v1+json
Accept: application/vnd.profile.v2+json

// Implementation
func VersionMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        version := c.GetHeader("Accept")
        if version == "" {
            version = "application/vnd.profile.v1+json"
        }
        c.Set("version", version)
        c.Next()
    }
}
```

## API Documentation

### 1. OpenAPI/Swagger

```yaml
openapi: 3.0.0
info:
  title: Profile Service API
  version: 1.0.0
  description: API for managing user profiles

paths:
  /api/v1/profiles:
    get:
      summary: List profiles
      parameters:
        - name: page
          in: query
          schema:
            type: integer
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: List of profiles
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Profile"
```

### 2. gRPC Documentation

```protobuf
syntax = "proto3";

package profile.v1;

import "google/api/annotations.proto";
import "protoc-gen-swagger/options/annotations.proto";

option (grpc.gateway.protoc_gen_swagger.options.openapiv2_swagger) = {
    info: {
        title: "Profile Service API"
        version: "1.0"
        description: "API for managing user profiles"
    }
};

service ProfileService {
    // CreateProfile creates a new user profile
    rpc CreateProfile(CreateProfileRequest) returns (Profile) {
        option (google.api.http) = {
            post: "/api/v1/profiles"
            body: "*"
        };
        option (grpc.gateway.protoc_gen_swagger.options.openapiv2_operation) = {
            summary: "Create profile"
            description: "Creates a new user profile"
        };
    }
}
```

## Best Practices

1. **Resource Design**

   - Use nouns for resources
   - Use HTTP methods appropriately
   - Keep URLs simple and intuitive
   - Use plural nouns for collections

2. **Request/Response Design**

   - Use consistent response formats
   - Include pagination for collections
   - Support field filtering
   - Provide meaningful error messages

3. **Versioning**

   - Plan for versioning from the start
   - Document versioning strategy
   - Support multiple versions
   - Maintain backward compatibility

4. **Documentation**
   - Keep documentation up to date
   - Include examples
   - Document error responses
   - Provide SDK examples

## Common Issues and Solutions

1. **API Evolution**

   - Problem: Breaking changes in APIs
   - Solution: Use versioning and deprecation notices

2. **Performance**

   - Problem: Large response payloads
   - Solution: Implement field selection and pagination

3. **Security**
   - Problem: Unauthorized access
   - Solution: Implement proper authentication and authorization

## References

- [REST API Design Best Practices](https://restfulapi.net/)
- [gRPC Documentation](https://grpc.io/docs/)
- [OpenAPI Specification](https://swagger.io/specification/)
- [API Versioning Best Practices](https://www.moesif.com/blog/technical/api-design/Building-a-Versioned-API/)
