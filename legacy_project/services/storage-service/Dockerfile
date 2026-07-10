# Build stage
# Using golang:1.23-alpine as the base image for building
# Alpine is chosen for its small size and security features
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Install required tools and dependencies
# - protobuf-dev: Required for Protocol Buffers compilation
# - make: For running make commands
# - git: For fetching dependencies
# - dos2unix: To handle potential Windows/Unix line ending issues in scripts
# Also install Go protobuf plugins for code generation
RUN apk add --no-cache protobuf-dev make git dos2unix && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy dependency files first to leverage Docker's layer caching
# This way, dependencies are only re-downloaded if go.mod or go.sum changes
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy the entire source code
# This is done after dependency download to optimize build caching
COPY . .

# Debug: List contents of scripts directory to verify script presence
RUN ls -la scripts/

# Fix potential line ending issues and make the script executable
# dos2unix converts Windows line endings to Unix format
# chmod +x makes the script executable
RUN dos2unix scripts/generate_proto.sh && \
    chmod +x scripts/generate_proto.sh

# Debug: Verify script permissions and existence
RUN ls -la scripts/generate_proto.sh

# Generate Protocol Buffer code
# Using 'sh' explicitly to ensure proper script execution
RUN sh scripts/generate_proto.sh

# Build the application
# CGO_ENABLED=0: Creates a statically linked binary
# GOOS=linux: Ensures the binary runs in Linux containers
RUN CGO_ENABLED=0 GOOS=linux go build -o storage-service ./cmd/server

# Final stage
# Using a minimal Alpine image for the runtime
# This significantly reduces the final image size
FROM alpine:3.19

# Set the working directory for the runtime container
WORKDIR /app

# Copy only the compiled binary from the builder stage
# This follows the multi-stage build pattern to minimize the final image size
COPY --from=builder /app/storage-service .

# Expose the ports the application will use
# 8080: REST API port
# 50051: gRPC port
EXPOSE 8080 50051

# Command to run the application
CMD ["./storage-service"] 