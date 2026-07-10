# Security Best Practices

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

### 2. OAuth2 Integration

```go
// OAuth2 configuration
type OAuth2Config struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
    Scopes       []string
    AuthURL      string
    TokenURL     string
}

// OAuth2 service
type OAuth2Service struct {
    config *oauth2.Config
    logger *zap.Logger
}

// Handle OAuth2 callback
func (s *OAuth2Service) HandleCallback(ctx context.Context, code string) (*oauth2.Token, error) {
    token, err := s.config.Exchange(ctx, code)
    if err != nil {
        return nil, fmt.Errorf("failed to exchange token: %w", err)
    }

    return token, nil
}

// Get user info from OAuth2 provider
func (s *OAuth2Service) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
    client := s.config.Client(ctx, token)
    resp, err := client.Get("https://api.provider.com/userinfo")
    if err != nil {
        return nil, fmt.Errorf("failed to get user info: %w", err)
    }
    defer resp.Body.Close()

    var userInfo UserInfo
    if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
        return nil, fmt.Errorf("failed to decode user info: %w", err)
    }

    return &userInfo, nil
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

### 2. Policy-Based Access Control

```go
// Policy definition
type Policy struct {
    ID          string
    Name        string
    Description string
    Effect      string
    Actions     []string
    Resources   []string
    Conditions  map[string]interface{}
}

// Policy service
type PolicyService struct {
    policies []Policy
    logger   *zap.Logger
}

// Evaluate policy
func (s *PolicyService) EvaluatePolicy(policy Policy, request Request) bool {
    // Check actions
    if !contains(policy.Actions, request.Action) {
        return false
    }

    // Check resources
    if !contains(policy.Resources, request.Resource) {
        return false
    }

    // Check conditions
    for key, value := range policy.Conditions {
        if !s.evaluateCondition(key, value, request) {
            return false
        }
    }

    return true
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

### 2. mTLS Implementation

```go
// mTLS configuration
type mTLSConfig struct {
    CertFile   string
    KeyFile    string
    CAFile     string
    ServerName string
}

// mTLS client setup
func NewmTLSClient(config mTLSConfig) (*http.Client, error) {
    cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load client certificate: %w", err)
    }

    caCert, err := ioutil.ReadFile(config.CAFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load CA certificate: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to append CA certificate")
    }

    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:     caCertPool,
        ServerName:  config.ServerName,
    }

    return &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
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

### 2. Secure Storage

```go
// Secure storage service
type SecureStorage struct {
    encryption *EncryptionService
    storage    Storage
}

// Store sensitive data
func (s *SecureStorage) Store(ctx context.Context, key string, data []byte) error {
    encrypted, err := s.encryption.Encrypt(data)
    if err != nil {
        return fmt.Errorf("failed to encrypt data: %w", err)
    }

    return s.storage.Put(ctx, key, encrypted)
}

// Retrieve sensitive data
func (s *SecureStorage) Retrieve(ctx context.Context, key string) ([]byte, error) {
    encrypted, err := s.storage.Get(ctx, key)
    if err != nil {
        return nil, fmt.Errorf("failed to get data: %w", err)
    }

    return s.encryption.Decrypt(encrypted)
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
   - Implement mTLS for service-to-service
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

## References

- [OWASP Security Guidelines](https://owasp.org/www-project-top-ten/)
- [JWT Best Practices](https://auth0.com/blog/jwt-security-best-practices/)
- [TLS Configuration Guide](https://ssl-config.mozilla.org/)
- [OAuth2 Security](https://oauth.net/articles/security/)
