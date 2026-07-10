# Interfaces Package

A collection of common interfaces for building microservices in Go.

## Features

- Transport interfaces for HTTP servers and clients
- Service interfaces for business logic
- Repository interfaces for data access
- Support for transactions and querying
- Generic type support for type-safe implementations

## Installation

```bash
go get github.com/FBDAF/microservices/services/common/interfaces
```

## Usage

### Transport Interfaces

```go
import "github.com/FBDAF/microservices/services/common/interfaces/transport"

// Implement the Handler interface
type MyHandler struct{}

func (h *MyHandler) RegisterRoutes(router *gin.Engine) {
    // Register your routes
}

// Use the Response type for standardized responses
resp := transport.NewResponse(http.StatusOK, "success", data, nil)
```

### Service Interfaces

```go
import "github.com/FBDAF/microservices/services/common/interfaces/service"

// Implement the Service interface
type UserService struct{}

func (s *UserService) Create(ctx context.Context, user *User) error {
    // Create user implementation
}

// Use QueryableService for advanced querying
type QueryableUserService struct{}

func (s *QueryableUserService) Query(ctx context.Context, options service.QueryOptions) ([]*User, error) {
    // Query implementation
}
```

### Repository Interfaces

```go
import "github.com/FBDAF/microservices/services/common/interfaces/repository"

// Implement the Repository interface
type UserRepository struct{}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    // Create user implementation
}

// Use TransactionalRepository for transaction support
type TransactionalUserRepository struct{}

func (r *TransactionalUserRepository) Begin(ctx context.Context) (repository.Transaction, error) {
    // Begin transaction implementation
}
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
