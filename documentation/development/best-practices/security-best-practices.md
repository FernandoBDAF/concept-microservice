# Security Best Practices

> *Migrated from legacy_project/reference-materials/development/security-best-practices.md*

## Overview

This document outlines the security best practices for our microservices architecture, covering authentication, authorization, secure communication, and data protection.

## Authentication

### 1. JWT Authentication

```go
// JWT configuration
type JWTConfig struct {
    SecretKey     []byte
    TokenDuration time.Duration
    Issuer        string
}

// JWT service
type JWTService struct {
    config JWTConfig
    logger *zap.Logger
}

// Generate JWT token
func (s *JWTService) GenerateToken(user *User) (string, error) {
    claims := jwt.MapClaims{
        "sub":    user.ID,
        "email":  user.Email,
        "roles":  user.Roles,
        "exp":    time.Now().Add(s.config.TokenDuration).Unix(),
        "iat":    time.Now().Unix(),
        "iss":    s.config.Issuer,
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.config.SecretKey)
}

// Validate JWT token
func (s *JWTService) ValidateToken(tokenString string) (*jwt.Token, error) {
    return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return s.config.SecretKey, nil
    })
}
```

### 2. Auth Service Integration

In the consolidated architecture, JWT validation is delegated to the external auth-service:

```go
// Auth client for token validation
type AuthClient struct {
    baseURL    string
    httpClient *http.Client
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
    
    // Handle response...
    return user, nil
}
```

## Authorization

### 1. Role-Based Access Control (RBAC)

```go
// Role definitions
type Role string

const (
    RoleAdmin    Role = "admin"
    RoleUser     Role = "user"
    RoleReadOnly Role = "readonly"
)

// Permission definitions
type Permission string

const (
    PermissionRead  Permission = "read"
    PermissionWrite Permission = "write"
    PermissionDelete Permission = "delete"
)

// RBAC service
type RBACService struct {
    rolePermissions map[Role][]Permission
    logger          *zap.Logger
}

// Check permission
func (s *RBACService) HasPermission(role Role, permission Permission) bool {
    permissions, exists := s.rolePermissions[role]
    if !exists {
        return false
    }

    for _, p := range permissions {
        if p == permission {
            return true
        }
    }

    return false
}

// Authorization middleware
func (s *RBACService) RequirePermission(permission Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        role := Role(c.GetString("role"))
        if !s.HasPermission(role, permission) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "permission denied",
            })
            return
        }
        c.Next()
    }
}
```

## Secure Communication

### 1. TLS Configuration

```go
// TLS configuration
type TLSConfig struct {
    CertFile   string
    KeyFile    string
    CAFile     string
    MinVersion uint16
}

// TLS setup
func NewTLSConfig(config TLSConfig) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load certificate: %w", err)
    }

    caCert, err := ioutil.ReadFile(config.CAFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load CA certificate: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to append CA certificate")
    }

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:     caCertPool,
        MinVersion:  config.MinVersion,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        },
    }, nil
}
```

## Data Protection

### 1. Encryption

```go
// Encryption service
type EncryptionService struct {
    key []byte
}

// Encrypt data
func (s *EncryptionService) Encrypt(data []byte) ([]byte, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, fmt.Errorf("failed to create cipher: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, fmt.Errorf("failed to generate nonce: %w", err)
    }

    return gcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt data
func (s *EncryptionService) Decrypt(data []byte) ([]byte, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, fmt.Errorf("failed to create cipher: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

## Best Practices

1. **Authentication**

   - Use strong authentication methods
   - Implement proper token management
   - Handle session security
   - Monitor authentication attempts

2. **Authorization**

   - Implement least privilege principle
   - Use role-based access control
   - Validate all requests
   - Audit access logs

3. **Secure Communication**

   - Use TLS for all communications
   - Keep certificates up to date
   - Monitor certificate expiration

4. **Data Protection**
   - Encrypt sensitive data
   - Implement secure storage
   - Use proper key management
   - Regular security audits

## Common Issues and Solutions

1. **Token Security**

   - Problem: Token leakage
   - Solution: Implement proper token storage and rotation

2. **Certificate Management**

   - Problem: Certificate expiration
   - Solution: Implement automated certificate rotation

3. **Data Encryption**
   - Problem: Key management
   - Solution: Use a key management service

## Cross-References

- [API Design Best Practices](api-design-best-practices.md)
- [Error Handling Best Practices](error-handling-best-practices.md)
- [Logging Best Practices](logging-best-practices.md)

## References

- [OWASP Security Guidelines](https://owasp.org/www-project-top-ten/)
- [JWT Best Practices](https://auth0.com/blog/jwt-security-best-practices/)
- [TLS Configuration Guide](https://ssl-config.mozilla.org/)
