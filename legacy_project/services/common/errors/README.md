# Errors Package

A robust error handling package for Go microservices, providing standardized error types, wrapping, and context management.

## Overview

The errors package provides a comprehensive set of tools for handling errors in microservices, including error wrapping, type checking, and context management. It's designed to make error handling more consistent and informative across services.

## Features

- Error wrapping with context
- Standardized error types
- Error classification
- Stack trace support
- Error logging integration
- Error recovery utilities
- Error translation
- Error validation

## Installation

```bash
go get github.com/your-org/common/errors
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/your-org/common/errors"
)

func main() {
    // Create a new error with context
    err := errors.New("failed to process request").
        WithCode("PROCESS_ERROR").
        WithField("request_id", "123").
        WithField("user_id", "456")

    // Check error type
    if errors.IsNotFound(err) {
        log.Printf("Resource not found: %v", err)
    }

    // Get error details
    if err != nil {
        log.Printf("Error code: %s", errors.GetCode(err))
        log.Printf("Error fields: %v", errors.GetFields(err))
    }
}
```

## Error Types

### Basic Error Types

```go
// Create a new error
err := errors.New("operation failed")

// Create an error with code
err := errors.NewWithCode("operation failed", "OP_FAILED")

// Create a not found error
err := errors.NotFound("user not found")

// Create a validation error
err := errors.Validation("invalid input")

// Create a conflict error
err := errors.Conflict("resource already exists")
```

### Error Wrapping

```go
// Wrap an existing error
err := errors.Wrap(originalErr, "failed to process request")

// Wrap with code
err := errors.WrapWithCode(originalErr, "failed to process request", "PROCESS_ERROR")

// Wrap with fields
err := errors.Wrap(originalErr, "failed to process request").
    WithField("request_id", "123").
    WithField("user_id", "456")
```

## Error Classification

```go
// Check error type
if errors.IsNotFound(err) {
    // Handle not found error
}

if errors.IsValidation(err) {
    // Handle validation error
}

if errors.IsConflict(err) {
    // Handle conflict error
}

// Get error code
code := errors.GetCode(err)

// Get error fields
fields := errors.GetFields(err)
```

## Error Recovery

```go
package main

import (
    "log"
    "github.com/your-org/common/errors"
)

func main() {
    defer errors.Recover(func(err error) {
        log.Printf("Recovered from panic: %v", err)
    })

    // Your code here
}
```

## Error Translation

```go
// Translate error to HTTP status
status := errors.ToHTTPStatus(err)

// Translate error to gRPC status
grpcStatus := errors.ToGRPCStatus(err)

// Translate error to custom format
customErr := errors.Translate(err, customTranslator)
```

## Best Practices

1. **Error Creation**

   - Use appropriate error types
   - Include relevant context
   - Add error codes
   - Include stack traces

2. **Error Handling**

   - Check error types
   - Log errors appropriately
   - Include error context
   - Handle errors at appropriate levels

3. **Error Recovery**

   - Use recovery in critical sections
   - Log recovered errors
   - Maintain service stability
   - Provide fallback behavior

4. **Error Translation**
   - Use consistent error codes
   - Translate errors appropriately
   - Maintain error context
   - Document error codes

## Examples

### HTTP Handler

```go
package main

import (
    "net/http"
    "github.com/your-org/common/errors"
)

func handler(w http.ResponseWriter, r *http.Request) {
    user, err := getUser(r.Context(), r.URL.Query().Get("id"))
    if err != nil {
        if errors.IsNotFound(err) {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        if errors.IsValidation(err) {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    // Process user...
}
```

### Database Operations

```go
package main

import (
    "database/sql"
    "github.com/your-org/common/errors"
)

func getUser(db *sql.DB, id string) (*User, error) {
    user := &User{}
    err := db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user.ID, &user.Name)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.NotFound("user not found").WithField("id", id)
        }
        return nil, errors.Wrap(err, "failed to query user").WithField("id", id)
    }
    return user, nil
}
```

### Error Recovery

```go
package main

import (
    "log"
    "github.com/your-org/common/errors"
)

func processRequest() {
    defer errors.Recover(func(err error) {
        log.Printf("Recovered from panic: %v", err)
        // Perform cleanup or fallback
    })

    // Your code here
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
