# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# BuildKit populates TARGETOS/TARGETARCH on cross-platform builds; empty
# GOARCH falls back to the build host's native architecture.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /bin/api-service ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates && \
    addgroup -S -g 10001 app && adduser -S -G app -H -u 10001 app

COPY --from=build /bin/api-service /usr/local/bin/api-service

USER 10001:10001
WORKDIR /

EXPOSE 8080 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/api-service"]
