# Security Package

A comprehensive security package for Go microservices with support for authentication, authorization, encryption, and secure communication.

## Overview

The security package provides a set of tools and utilities for implementing security features in microservices. It includes authentication, authorization, encryption, secure communication, and other security-related functionality.

## Features

- JWT authentication
- Role-based access control
- Password hashing
- Encryption/decryption
- Secure communication
- Input validation
- Security headers
- Rate limiting
- CORS support
- XSS protection

## Installation

```bash
go get github.com/your-org/common/security
```

## Quick Start

```go
package main

import (
    "github.com/your-org/common/security"
)

func main() {
    // Create a new security manager
    manager := security.NewManager()

    // Generate JWT token
    token, err := manager.GenerateToken("user123", []string{"admin"})
    if err != nil {
        // Handle error
    }

    // Validate token
    claims, err := manager.ValidateToken(token)
    if err != nil {
        // Handle error
    }
}
```

## Authentication

### JWT Authentication

```go
// Generate token
token, err := manager.GenerateToken("user123", []string{"admin"})
if err != nil {
    // Handle error
}

// Validate token
claims, err := manager.ValidateToken(token)
if err != nil {
    // Handle error
}

// Get user ID from claims
userID := claims.UserID

// Check roles
if claims.HasRole("admin") {
    // Handle admin access
}
```

### Password Hashing

```go
// Hash password
hashedPassword, err := security.HashPassword("password123")
if err != nil {
    // Handle error
}

// Verify password
if security.VerifyPassword(hashedPassword, "password123") {
    // Password is correct
}
```

## Authorization

### Role-Based Access Control

```go
// Check role
if manager.HasRole("user123", "admin") {
    // User has admin role
}

// Check permission
if manager.HasPermission("user123", "read:users") {
    // User has permission to read users
}

// Add role
err := manager.AddRole("user123", "admin")
if err != nil {
    // Handle error
}

// Remove role
err := manager.RemoveRole("user123", "admin")
if err != nil {
    // Handle error
}
```

## Encryption

### Symmetric Encryption

```go
// Encrypt data
encrypted, err := security.Encrypt([]byte("sensitive data"), "secret-key")
if err != nil {
    // Handle error
}

// Decrypt data
decrypted, err := security.Decrypt(encrypted, "secret-key")
if err != nil {
    // Handle error
}
```

### Asymmetric Encryption

```go
// Generate key pair
publicKey, privateKey, err := security.GenerateKeyPair()
if err != nil {
    // Handle error
}

// Encrypt with public key
encrypted, err := security.EncryptWithPublicKey([]byte("sensitive data"), publicKey)
if err != nil {
    // Handle error
}

// Decrypt with private key
decrypted, err := security.DecryptWithPrivateKey(encrypted, privateKey)
if err != nil {
    // Handle error
}
```

## Secure Communication

### HTTPS

```go
// Create HTTPS server
server := &http.Server{
    Addr:    ":443",
    Handler: handler,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}

// Start server
err := server.ListenAndServeTLS("cert.pem", "key.pem")
if err != nil {
    // Handle error
}
```

### Security Headers

```go
// Add security headers
handler := security.HeadersMiddleware(http.HandlerFunc(handler))

// Start server
http.ListenAndServe(":8080", handler)
```

## Best Practices

1. **Authentication**

   - Use strong passwords
   - Implement MFA
   - Use secure session management
   - Implement proper token handling

2. **Authorization**

   - Use role-based access control
   - Implement least privilege
   - Validate permissions
   - Audit access logs

3. **Encryption**

   - Use strong encryption
   - Secure key management
   - Regular key rotation
   - Proper key storage

4. **Communication**
   - Use HTTPS
   - Implement security headers
   - Validate input
   - Sanitize output

## Examples

### HTTP Handler

```go
package main

import (
    "net/http"
    "github.com/your-org/common/security"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Get token from header
    token := r.Header.Get("Authorization")

    // Validate token
    claims, err := manager.ValidateToken(token)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Check role
    if !claims.HasRole("admin") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle request
    w.Write([]byte("Hello, Admin!"))
}
```

### Middleware

```go
package main

import (
    "net/http"
    "github.com/your-org/common/security"
)

func main() {
    // Create security middleware
    middleware := security.NewMiddleware(manager)

    // Create handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })

    // Add middleware
    http.Handle("/", middleware.Auth(handler))

    // Start server
    http.ListenAndServe(":8080", nil)
}
```

### API Client

```go
package main

import (
    "net/http"
    "github.com/your-org/common/security"
)

func main() {
    // Create HTTP client
    client := &http.Client{}

    // Create request
    req, err := http.NewRequest("GET", "https://api.example.com/users", nil)
    if err != nil {
        // Handle error
    }

    // Add token
    token, err := manager.GenerateToken("user123", []string{"admin"})
    if err != nil {
        // Handle error
    }
    req.Header.Set("Authorization", token)

    // Send request
    resp, err := client.Do(req)
    if err != nil {
        // Handle error
    }
    defer resp.Body.Close()
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This package is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
