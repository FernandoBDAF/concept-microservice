# =============================================================================
# QUEUE SERVICE DOCKERFILE FOR KIND DEPLOYMENT
# =============================================================================
#
# 📚 EDUCATIONAL OVERVIEW:
# This Dockerfile creates a lightweight container for the Queue Service,
# a Go-based microservice that provides message queuing capabilities
# using RabbitMQ as the backend message broker.
#
# 🏗️ MULTI-STAGE BUILD:
# Stage 1: Build the Go application from source with local dependencies
# Stage 2: Create minimal runtime container with just the binary
#
# 🔧 KIND-SPECIFIC OPTIMIZATIONS:
# - Handles local module dependencies (common/metrics)
# - Creates minimal final image for faster Kind loading
# - Includes health check endpoint for Kubernetes probes
#
# ⚠️ PRODUCTION DIFFERENCES:
# - Production would use published modules instead of local dependencies
# - Would include security scanning and vulnerability checks
# - Would use distroless or alpine base images
# - Would implement proper user permissions (non-root)
# =============================================================================

# 🏗️ BUILD STAGE: Compile the Go application
FROM golang:1.22-alpine AS builder

# 📁 WORKING DIRECTORY
WORKDIR /services

# 📦 COPY ALL SERVICES (needed for local module dependencies)
COPY . .

# 📁 CHANGE TO QUEUE SERVICE DIRECTORY
WORKDIR /services/queue-service

# 📦 DEPENDENCY MANAGEMENT
RUN go mod download

# 🔨 BUILD THE APPLICATION
# CGO_ENABLED=0 creates a static binary
# -ldflags="-w -s" reduces binary size by removing debug info
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o queue-service ./cmd/main.go

# 🚀 RUNTIME STAGE: Create minimal container
FROM alpine:3.18

# 🔧 INSTALL RUNTIME DEPENDENCIES
# ca-certificates: For HTTPS connections
# tzdata: For timezone support
# wget: For health checks
RUN apk --no-cache add ca-certificates tzdata wget

# 👤 CREATE NON-ROOT USER (security best practice)
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 📁 WORKING DIRECTORY
WORKDIR /app

# 📋 COPY BINARY FROM BUILD STAGE
COPY --from=builder /services/queue-service/queue-service .

# 🔐 SET OWNERSHIP AND PERMISSIONS
RUN chown appuser:appgroup queue-service && \
    chmod +x queue-service

# 👤 SWITCH TO NON-ROOT USER
USER appuser

# 🔌 EXPOSE PORTS
# 8080: HTTP API port
# 9090: Metrics port
EXPOSE 8080 9090

# 🏥 HEALTH CHECK
# Kubernetes will use readiness/liveness probes, but this provides
# a fallback health check mechanism for Docker environments
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 🚀 ENTRY POINT
# Use exec form to ensure proper signal handling
ENTRYPOINT ["./queue-service"] 