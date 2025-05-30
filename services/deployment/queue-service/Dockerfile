# Build stage
FROM golang:1.22.0-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies with GOTOOLCHAIN=auto to handle version requirements
RUN GOTOOLCHAIN=auto go mod download

# Copy source code
COPY . .

# Set the module path
ENV GO111MODULE=on
ENV GOPATH=/go
ENV PATH=$PATH:$GOPATH/bin

# Build the application
RUN cd queue-service && \
    CGO_ENABLED=0 GOOS=linux go build -o queue-service ./cmd/main.go

# Final stage
FROM alpine:3.18

WORKDIR /app

# Create non-root user
RUN adduser -D -g '' appuser

# Copy the binary from builder
COPY --from=builder /app/queue-service/queue-service .

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./queue-service"] 