# Docker Usage Guide

> *Migrated from legacy_project/reference-materials/development/tools/docker.md*

## Overview

Docker is a platform for developing, shipping, and running applications in containers. In our microservices architecture, we use Docker to containerize our services, ensuring consistent environments across development, testing, and production.

## Key Features Used

### 1. Container Configuration

We use Dockerfiles to define our container configurations:

```dockerfile
# Base image
FROM golang:1.21-alpine

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main ./cmd/api-service

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
```

### 2. Multi-stage Builds

For optimized production images:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o api-service ./cmd/api-service

# Final stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api-service .
EXPOSE 8080
CMD ["./api-service"]
```

### 3. Docker Compose

For local development and testing:

```yaml
version: "3.8"

services:
  api-service:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_ADDR=redis:6379
      - RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
    depends_on:
      - postgres
      - redis
      - rabbitmq

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=profiles
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  rabbitmq:
    image: rabbitmq:3-management-alpine
    ports:
      - "5672:5672"
      - "15672:15672"

volumes:
  postgres_data:
```

## Best Practices

1. **Image Size**

   - Use multi-stage builds
   - Choose minimal base images
   - Remove unnecessary files
   - Use .dockerignore

2. **Security**

   - Don't run as root
   - Scan images for vulnerabilities
   - Use specific versions
   - Keep images updated

3. **Performance**

   - Optimize layer caching
   - Use appropriate base images
   - Minimize layers
   - Use build cache

4. **Maintenance**
   - Regular updates
   - Version tagging
   - Clean up unused images
   - Monitor disk usage

## Common Issues and Solutions

1. **Build Context Too Large**

   - Problem: Slow builds due to large context
   - Solution: Use .dockerignore and optimize context

2. **Container Memory Issues**

   - Problem: Containers running out of memory
   - Solution: Set appropriate memory limits

3. **Network Connectivity**
   - Problem: Containers can't communicate
   - Solution: Use Docker networks and proper DNS

## API Service Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o api-service ./cmd/api-service

# Final stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api-service .
EXPOSE 8080
CMD ["./api-service"]
```

## References

- [Docker Official Documentation](https://docs.docker.com/)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Docker Security](https://docs.docker.com/engine/security/)
