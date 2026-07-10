# API Patterns

## Overview

This document outlines the standard API patterns used across the Profile Service Microservices architecture.

## REST API Patterns

### 1. Resource-Based URLs

```mermaid
graph TD
    A[API Gateway] -->|/api/v1/profiles| B[Profile Service]
    A -->|/api/v1/profiles/{id}| B
    A -->|/api/v1/profiles/{id}/settings| B
    A -->|/api/v1/profiles/{id}/preferences| B
```

#### URL Structure

```yaml
api:
  base_path: /api/v1
  resources:
    profiles:
      path: /profiles
      sub_resources:
        - path: /{id}
          methods: [GET, PUT, DELETE]
        - path: /{id}/settings
          methods: [GET, PUT]
        - path: /{id}/preferences
          methods: [GET, PUT]
```

### 2. HTTP Methods

```yaml
methods:
  GET:
    description: Retrieve resources
    idempotent: true
    safe: true
    examples:
      - GET /api/v1/profiles
      - GET /api/v1/profiles/{id}

  POST:
    description: Create resources
    idempotent: false
    safe: false
    examples:
      - POST /api/v1/profiles

  PUT:
    description: Update resources
    idempotent: true
    safe: false
    examples:
      - PUT /api/v1/profiles/{id}

  DELETE:
    description: Remove resources
    idempotent: true
    safe: false
    examples:
      - DELETE /api/v1/profiles/{id}
```

### 3. Response Patterns

```yaml
responses:
  success:
    format:
      data: object|array
      metadata:
        pagination:
          total: number
          page: number
          size: number
        timestamp: string
        request_id: string

  error:
    format:
      error:
        code: string
        message: string
        details: object
      metadata:
        timestamp: string
        request_id: string
```

## gRPC API Patterns

### 1. Service Definition

```protobuf
// Common Service Pattern
service ProfileService {
  // Standard CRUD Operations
  rpc CreateProfile(CreateProfileRequest) returns (Profile);
  rpc GetProfile(GetProfileRequest) returns (Profile);
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile);
  rpc DeleteProfile(DeleteProfileRequest) returns (Empty);

  // List Operations
  rpc ListProfiles(ListProfilesRequest) returns (ListProfilesResponse);

  // Stream Operations
  rpc StreamProfileUpdates(StreamProfileRequest) returns (stream ProfileUpdate);

  // Batch Operations
  rpc BatchGetProfiles(BatchGetProfilesRequest) returns (BatchGetProfilesResponse);
}
```

### 2. Message Patterns

```protobuf
// Request Pattern
message ProfileRequest {
  string request_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  map<string, string> metadata = 3;
  oneof operation {
    CreateProfileRequest create = 4;
    GetProfileRequest get = 5;
    UpdateProfileRequest update = 6;
    DeleteProfileRequest delete = 7;
  }
}

// Response Pattern
message ProfileResponse {
  string request_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  Status status = 3;
  oneof result {
    Profile profile = 4;
    Error error = 5;
  }
}
```

## API Versioning

### 1. URL Versioning

```yaml
versioning:
  url:
    pattern: /api/v{version}/{resource}
    examples:
      - /api/v1/profiles
      - /api/v2/profiles
    supported_versions:
      - v1
      - v2
```

### 2. Header Versioning

```yaml
versioning:
  header:
    name: API-Version
    values:
      - 1.0
      - 2.0
    default: 1.0
```

## API Documentation

### 1. OpenAPI Specification

```yaml
openapi: 3.0.0
info:
  title: Profile Service API
  version: 1.0.0
  description: API for managing user profiles
paths:
  /profiles:
    get:
      summary: List profiles
      parameters:
        - name: page
          in: query
          schema:
            type: integer
        - name: size
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: Successful response
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ProfileList"
```

### 2. gRPC Documentation

```protobuf
// Service Documentation
service ProfileService {
  // GetProfile retrieves a profile by ID
  // Returns NOT_FOUND if the profile doesn't exist
  rpc GetProfile(GetProfileRequest) returns (Profile) {
    option (google.api.http) = {
      get: "/v1/profiles/{id}"
    };
  }
}
```

## API Security

### 1. Authentication

```yaml
authentication:
  methods:
    - type: JWT
      header: Authorization
      format: Bearer {token}
    - type: API Key
      header: X-API-Key
      format: { key }
```

### 2. Authorization

```yaml
authorization:
  roles:
    - name: admin
      permissions:
        - profiles:read
        - profiles:write
        - profiles:delete
    - name: user
      permissions:
        - profiles:read
        - profiles:write
```

## API Monitoring

### 1. Metrics

```yaml
metrics:
  - name: api_requests_total
    type: counter
    labels:
      - method
      - endpoint
      - status_code

  - name: api_request_duration_seconds
    type: histogram
    labels:
      - method
      - endpoint
```

### 2. Logging

```yaml
logging:
  format: json
  fields:
    - timestamp
    - request_id
    - method
    - path
    - status_code
    - duration
    - client_ip
    - user_agent
```

## Notes

- Keep documentation up to date
- Maintain cross-references
- Add practical examples
- Document decisions
- Track changes
- Ensure alignment with global architecture
