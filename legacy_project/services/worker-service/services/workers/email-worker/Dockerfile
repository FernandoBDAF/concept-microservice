# Build stage
FROM golang:1.22-alpine AS builder

# Install dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o email-worker cmd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S emailworker && \
    adduser -u 1001 -S emailworker -G emailworker

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/email-worker .

# Change ownership to non-root user
RUN chown emailworker:emailworker email-worker && \
    chmod +x email-worker

# Switch to non-root user
USER emailworker

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["./email-worker"] 