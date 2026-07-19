# syntax=docker/dockerfile:1
# ntfy-relay — Alertmanager webhook -> ntfy bridge (ADR-003.4). Same
# build/runtime pattern as the lab's Go services; stdlib only, no go.sum.

FROM golang:1.24-alpine AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod ./
COPY *.go ./

# BuildKit populates TARGETOS/TARGETARCH on cross-platform builds; empty
# GOARCH falls back to the build host's native architecture.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /bin/ntfy-relay .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates && \
    addgroup -S -g 10001 app && adduser -S -G app -H -u 10001 app

COPY --from=build /bin/ntfy-relay /usr/local/bin/ntfy-relay

USER 10001:10001
WORKDIR /

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/ntfy-relay"]
