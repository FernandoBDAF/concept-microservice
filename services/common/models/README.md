# Models Package

A standardized data model package for Go microservices with support for validation, serialization, and common operations.

## Overview

The models package provides a set of standardized data models and utilities for working with them across services. It includes common data structures, validation rules, and serialization methods to ensure consistency and type safety.

## Features

- Standardized data models
- JSON/XML serialization
- Validation rules
- Type safety
- Common operations
- Database integration
- API documentation
- Example implementations

## Installation

```bash
go get github.com/your-org/common/models
```

## Quick Start

```go
package main

import (
    "github.com/your-org/common/models"
)

func main() {
    // Create a new user
    user := &models.User{
        ID:        "123",
        Name:      "John Doe",
        Email:     "john@example.com",
        CreatedAt: time.Now(),
    }

    // Validate user
    if err := user.Validate(); err != nil {
        // Handle validation error
    }

    // Convert to JSON
    jsonData, err := user.ToJSON()
    if err != nil {
        // Handle error
    }
}
```

## Model Types

### User Model

```go
// Create a new user
user := &models.User{
    ID:        "123",
    Name:      "John Doe",
    Email:     "john@example.com",
    CreatedAt: time.Now(),
}

// Validate user
if err := user.Validate(); err != nil {
    // Handle validation error
}

// Convert to JSON
jsonData, err := user.ToJSON()
if err != nil {
    // Handle error
}

// Convert from JSON
newUser := &models.User{}
err = newUser.FromJSON(jsonData)
if err != nil {
    // Handle error
}
```

### Product Model

```go
// Create a new product
product := &models.Product{
    ID:          "123",
    Name:        "Product Name",
    Description: "Product Description",
    Price:       99.99,
    CreatedAt:   time.Now(),
}

// Validate product
if err := product.Validate(); err != nil {
    // Handle validation error
}

// Convert to JSON
jsonData, err := product.ToJSON()
if err != nil {
    // Handle error
}
```

### Order Model

```go
// Create a new order
order := &models.Order{
    ID:        "123",
    UserID:    "456",
    Products:  []string{"789", "012"},
    Total:     199.98,
    CreatedAt: time.Now(),
}

// Validate order
if err := order.Validate(); err != nil {
    // Handle validation error
}

// Convert to JSON
jsonData, err := order.ToJSON()
if err != nil {
    // Handle error
}
```

## Validation

```go
// Validate user
if err := user.Validate(); err != nil {
    // Handle validation error
}

// Validate product
if err := product.Validate(); err != nil {
    // Handle validation error
}

// Validate order
if err := order.Validate(); err != nil {
    // Handle validation error
}
```

## Serialization

```go
// Convert to JSON
jsonData, err := user.ToJSON()
if err != nil {
    // Handle error
}

// Convert from JSON
newUser := &models.User{}
err = newUser.FromJSON(jsonData)
if err != nil {
    // Handle error
}

// Convert to XML
xmlData, err := user.ToXML()
if err != nil {
    // Handle error
}

// Convert from XML
newUser := &models.User{}
err = newUser.FromXML(xmlData)
if err != nil {
    // Handle error
}
```

## Best Practices

1. **Model Design**

   - Use clear and descriptive names
   - Include validation rules
   - Document fields and methods
   - Follow naming conventions

2. **Validation**

   - Validate all input data
   - Use appropriate validation rules
   - Handle validation errors
   - Document validation rules

3. **Serialization**

   - Use consistent serialization methods
   - Handle serialization errors
   - Document serialization format
   - Test serialization/deserialization

4. **Maintenance**
   - Keep models up to date
   - Document changes
   - Test all operations
   - Review and refactor

## Examples

### HTTP Handler

```go
package main

import (
    "net/http"
    "github.com/your-org/common/models"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Parse request body
    user := &models.User{}
    if err := user.FromJSON(r.Body); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Validate user
    if err := user.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Process user
    // ...

    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(user.ToJSON())
}
```

### Database Operations

```go
package main

import (
    "database/sql"
    "github.com/your-org/common/models"
)

func main() {
    // Create user
    user := &models.User{
        ID:        "123",
        Name:      "John Doe",
        Email:     "john@example.com",
        CreatedAt: time.Now(),
    }

    // Validate user
    if err := user.Validate(); err != nil {
        // Handle validation error
    }

    // Save to database
    if err := saveUser(db, user); err != nil {
        // Handle error
    }
}

func saveUser(db *sql.DB, user *models.User) error {
    // Save user to database
    // ...
    return nil
}
```

### API Client

```go
package main

import (
    "net/http"
    "github.com/your-org/common/models"
)

func main() {
    // Create HTTP client
    client := &http.Client{}

    // Create request
    req, err := http.NewRequest("GET", "http://api.example.com/users/123", nil)
    if err != nil {
        // Handle error
    }

    // Send request
    resp, err := client.Do(req)
    if err != nil {
        // Handle error
    }
    defer resp.Body.Close()

    // Parse response
    user := &models.User{}
    if err := user.FromJSON(resp.Body); err != nil {
        // Handle error
    }

    // Validate user
    if err := user.Validate(); err != nil {
        // Handle validation error
    }
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
