# Common Packages

A collection of shared packages for microservices, providing consistent implementations of common functionality across services.

## Overview

This repository contains a set of common packages that provide standardized implementations for cross-cutting concerns in microservices. These packages are designed to be:

- Independent and modular
- Well-documented
- Easy to integrate
- Production-ready
- Maintainable and extensible

## Available Packages

### Configuration Management (`config/`)

Provides a standardized way to handle configuration across services.

- Environment-based configuration
- File-based configuration
- Secret management
- Configuration validation

[Documentation](config/README.md)

### Error Handling (`errors/`)

Standardized error handling and wrapping utilities.

- Error wrapping
- Error types
- Error context
- Error logging integration

[Documentation](errors/README.md)

### Logging (`logging/`)

Structured logging implementation with multiple backends.

- JSON formatting
- Log levels
- Context support
- Multiple outputs

[Documentation](logging/README.md)

### Metrics (`metrics/`)

Metrics collection and reporting with Prometheus integration.

- Counter metrics
- Gauge metrics
- Histogram metrics
- Timer metrics
- Prometheus integration

[Documentation](metrics/README.md)

### Models (`models/`)

Common data models and interfaces.

- Base model interfaces
- Validation
- Serialization
- Versioning

[Documentation](models/README.md)

### Security (`security/`)

Security utilities and middleware.

- Authentication
- Authorization
- JWT handling
- Rate limiting

[Documentation](security/README.md)

### Utils (`utils/`)

Common utility functions and helpers.

- String manipulation
- Time handling
- File operations
- Common algorithms

[Documentation](utils/README.md)

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git

### Installation

Add the required packages to your service's `go.mod`:

```go
require (
    github.com/your-org/common/config v0.1.0
    github.com/your-org/common/errors v0.1.0
    github.com/your-org/common/logging v0.1.0
    github.com/your-org/common/metrics v0.1.0
    github.com/your-org/common/models v0.1.0
    // Add other packages as needed
)
```

### Quick Start

1. Choose the packages you need
2. Add them to your dependencies
3. Follow the package-specific documentation
4. Run the examples

## Best Practices

1. **Configuration**

   - Use environment variables for sensitive data
   - Keep configuration files in version control
   - Use different config files for different environments

2. **Error Handling**

   - Always wrap errors with context
   - Use appropriate error types
   - Log errors with sufficient detail

3. **Logging**

   - Use appropriate log levels
   - Include relevant context
   - Structure log messages consistently

4. **Metrics**

   - Use descriptive metric names
   - Include appropriate labels
   - Document metric purposes

5. **Models**
   - Implement all required interfaces
   - Use provided validation methods
   - Follow naming conventions

## Migration

For detailed migration instructions, see [MIGRATION.md](MIGRATION.md).

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## Versioning

We use [SemVer](http://semver.org/) for versioning. For available versions, see the [tags](https://github.com/your-org/common/tags).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For support, please:

1. Check the package documentation
2. Review the examples
3. Open an issue
4. Contact the development team

## Roadmap

- [ ] Additional metrics backends
- [ ] Enhanced security features
- [ ] More utility functions
- [ ] Additional model types
- [ ] Performance improvements

## Acknowledgments

- [Prometheus](https://prometheus.io/)
- [Zap](https://github.com/uber-go/zap)
- [Viper](https://github.com/spf13/viper)
- [JWT-Go](https://github.com/golang-jwt/jwt)
