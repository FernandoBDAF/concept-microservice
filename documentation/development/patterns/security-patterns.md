# Security Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/security-patterns.md*

## Overview

This document outlines the security patterns implemented in the API Service, with focus on authentication delegation to auth-service and authorization.

## Architecture Context

In the consolidated architecture:
- **Authentication** is delegated to the external auth-service (the only HTTP dependency)
- **Authorization** is handled internally based on JWT claims

## Authentication Patterns

### 1. JWT Validation via Auth Service

```go
type AuthClient struct {
    baseURL    string
    httpClient *http.Client
    logger     *zap.Logger
}

func NewAuthClient(baseURL string) *AuthClient {
    return &AuthClient{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 5 * time.Second,
        },
    }
}

// ValidateToken calls auth-service to validate JWT
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*User, error) {
    req, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/v1/auth/token/validate",
        bytes.NewReader([]byte(`{"token": "`+token+`"}`)))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("auth service unavailable: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return nil, ErrInvalidToken
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("auth service error: %d", resp.StatusCode)
    }

    var result struct {
        User *User `json:"user"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.User, nil
}
```

### 2. Auth Middleware

```go
func AuthMiddleware(authClient *AuthClient) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "missing authorization header",
            })
            return
        }

        // Parse Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid authorization header format",
            })
            return
        }
        token := parts[1]

        // Validate with auth-service
        user, err := authClient.ValidateToken(c.Request.Context(), token)
        if err != nil {
            if errors.Is(err, ErrInvalidToken) {
                c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                    "error": "invalid token",
                })
                return
            }
            c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
                "error": "authentication service unavailable",
            })
            return
        }

        // Store user in context
        c.Set("user", user)
        c.Set("user_id", user.ID)
        c.Set("user_roles", user.Roles)

        c.Next()
    }
}
```

## Authorization Patterns

### 1. Role-Based Access Control (RBAC)

```go
type Role string

const (
    RoleAdmin    Role = "admin"
    RoleUser     Role = "user"
    RoleReadOnly Role = "readonly"
)

type Permission string

const (
    PermissionRead   Permission = "read"
    PermissionWrite  Permission = "write"
    PermissionDelete Permission = "delete"
    PermissionAdmin  Permission = "admin"
)

// Role to permissions mapping
var rolePermissions = map[Role][]Permission{
    RoleAdmin:    {PermissionRead, PermissionWrite, PermissionDelete, PermissionAdmin},
    RoleUser:     {PermissionRead, PermissionWrite},
    RoleReadOnly: {PermissionRead},
}

func HasPermission(roles []Role, permission Permission) bool {
    for _, role := range roles {
        if perms, ok := rolePermissions[role]; ok {
            for _, p := range perms {
                if p == permission {
                    return true
                }
            }
        }
    }
    return false
}
```

### 2. Authorization Middleware

```go
func RequirePermission(permission Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        rolesInterface, exists := c.Get("user_roles")
        if !exists {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "no roles found",
            })
            return
        }

        roles, ok := rolesInterface.([]Role)
        if !ok {
            c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
                "error": "invalid roles format",
            })
            return
        }

        if !HasPermission(roles, permission) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "permission denied",
            })
            return
        }

        c.Next()
    }
}

// Usage in routes
func RegisterRoutes(r *gin.Engine, authClient *AuthClient) {
    api := r.Group("/api/v1")
    api.Use(AuthMiddleware(authClient))

    profiles := api.Group("/profiles")
    {
        profiles.GET("", RequirePermission(PermissionRead), listProfiles)
        profiles.GET("/:id", RequirePermission(PermissionRead), getProfile)
        profiles.POST("", RequirePermission(PermissionWrite), createProfile)
        profiles.PUT("/:id", RequirePermission(PermissionWrite), updateProfile)
        profiles.DELETE("/:id", RequirePermission(PermissionDelete), deleteProfile)
    }
}
```

### 3. Resource-Based Authorization

```go
// Check if user owns the profile
func RequireOwnerOrAdmin() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, _ := c.Get("user_id")
        roles, _ := c.Get("user_roles")
        profileID := c.Param("id")

        // Admin can access any profile
        if HasPermission(roles.([]Role), PermissionAdmin) {
            c.Next()
            return
        }

        // User can only access their own profile
        if userID.(string) != profileID {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "you can only access your own profile",
            })
            return
        }

        c.Next()
    }
}
```

## Input Validation

```go
type CreateProfileRequest struct {
    FirstName string `json:"firstName" binding:"required,min=1,max=100"`
    LastName  string `json:"lastName" binding:"required,min=1,max=100"`
    Email     string `json:"email" binding:"required,email"`
    Phone     string `json:"phone" binding:"omitempty,e164"`
}

func (h *ProfileHandler) CreateProfile(c *gin.Context) {
    var req CreateProfileRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "validation failed",
            "details": err.Error(),
        })
        return
    }

    // Process validated request
    // ...
}
```

## Best Practices

1. **Authentication**
   - Always validate tokens with auth-service
   - Handle auth-service unavailability gracefully
   - Cache validated tokens briefly if needed

2. **Authorization**
   - Check permissions at the handler level
   - Implement resource-based authorization
   - Use middleware for common checks

3. **Input Validation**
   - Validate all inputs
   - Use binding tags
   - Sanitize user inputs

4. **Error Handling**
   - Don't leak sensitive information
   - Use consistent error responses
   - Log security events

## Cross-References

- [Security Best Practices](../best-practices/security-best-practices.md)
- [API Design Best Practices](../best-practices/api-design-best-practices.md)
- [Error Handling Best Practices](../best-practices/error-handling-best-practices.md)

## Notes

- Auth-service is the only external HTTP dependency
- All other infrastructure is accessed directly
- Authorization is handled internally based on JWT claims
