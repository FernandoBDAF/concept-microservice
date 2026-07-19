# syntax=docker/dockerfile:1
# One Dockerfile, two images (HOST_CONTRACT.md §1.1):
#   docker build --build-arg CMD=web    -t localhost:5001/hello-guest-web:dev .
#   docker build --build-arg CMD=worker -t localhost:5001/hello-guest-worker:dev .

FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod ./
# Zero external deps by design — nothing to download, but keep the layer so
# the pattern matches the lab's other services if deps ever appear.
RUN go mod download

COPY . .

ARG CMD=web
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /bin/hello-guest ./cmd/${CMD}

FROM alpine:3.19

RUN addgroup -S -g 10001 app && adduser -S -G app -H -u 10001 app

COPY --from=build /bin/hello-guest /usr/local/bin/hello-guest

USER 10001:10001
WORKDIR /

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/hello-guest"]
