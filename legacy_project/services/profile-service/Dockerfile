# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o profile-service ./cmd

# Final stage
FROM alpine:3.18

# Install runtime dependencies and security updates
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates

# Set timezone
ENV TZ=UTC

# Create non-root user for security
RUN adduser -D -g '' -s /bin/sh appuser

# Create necessary directories
RUN mkdir -p /app/logs /app/tmp /app/cache && \
    chown -R appuser:appuser /app

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/profile-service .

# Set proper permissions
RUN chown appuser:appuser /app/profile-service && \
    chmod +x /app/profile-service

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 8081

# Environment variables for multi-worker architecture
ENV ENV=production
ENV LOG_LEVEL=info
ENV LOG_FORMAT=json

# Multi-worker configuration
ENV SUPPORTED_TASK_TYPES=profile_update,email_notification,image_processing
ENV ROUTING_KEY_PROFILE_UPDATE=profile.task
ENV ROUTING_KEY_EMAIL_NOTIFICATION=email.send
ENV ROUTING_KEY_IMAGE_PROCESSING=image.process
ENV ROUTING_KEY_DEFAULT_FALLBACK=profile.task

# Queue service configuration
ENV QUEUE_SERVICE_URL=http://queue-service:8080
ENV QUEUE_SERVICE_TIMEOUT=30s
ENV QUEUE_SERVICE_RETRIES=3

# Performance and monitoring
ENV METRICS_ENABLED=true
ENV HEALTH_CHECK_ENABLED=true
ENV CIRCUIT_BREAKER_ENABLED=true

# Task timeout configurations (based on performance testing)
ENV PROFILE_TASK_TIMEOUT=5m
ENV EMAIL_TASK_TIMEOUT=2m
ENV IMAGE_TASK_TIMEOUT=10m

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./profile-service"] 