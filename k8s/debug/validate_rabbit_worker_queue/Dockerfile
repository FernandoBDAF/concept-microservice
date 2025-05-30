# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build publisher
RUN CGO_ENABLED=0 GOOS=linux go build -o publisher ./cmd/publisher

# Build consumer
RUN CGO_ENABLED=0 GOOS=linux go build -o consumer ./cmd/consumer

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/publisher /app/publisher

# Set entrypoint to run both services
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"] 