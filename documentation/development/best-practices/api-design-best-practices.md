# API Design Best Practices

> *Migrated from legacy_project/reference-materials/development/api-design-best-practices.md*

## Overview

This document outlines the best practices for API design in our microservices architecture, covering RESTful HTTP APIs. It provides guidelines for endpoint design, request/response handling, versioning, and documentation.

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

// Actions (task submission)
POST   /api/v1/profiles/{id}/tasks/email    // Submit email task
POST   /api/v1/profiles/{id}/tasks/image    // Submit image task
POST   /api/v1/profiles/{id}/tasks/profile  // Submit profile task
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
        v1.POST("/profiles", s.CreateProfileV1)
        v1.PUT("/profiles/:id", s.UpdateProfileV1)
        v1.DELETE("/profiles/:id", s.DeleteProfileV1)
        
        // Task submission
        v1.POST("/profiles/:id/tasks/email", s.SubmitEmailTask)
        v1.POST("/profiles/:id/tasks/image", s.SubmitImageTask)
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
    post:
      summary: Create profile
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateProfileRequest"
      responses:
        "201":
          description: Profile created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Profile"
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

## Cross-References

- [Error Handling Best Practices](error-handling-best-practices.md)
- [Security Best Practices](security-best-practices.md)
- [Logging Best Practices](logging-best-practices.md)

## References

- [REST API Design Best Practices](https://restfulapi.net/)
- [OpenAPI Specification](https://swagger.io/specification/)
- [API Versioning Best Practices](https://www.moesif.com/blog/technical/api-design/Building-a-Versioned-API/)
