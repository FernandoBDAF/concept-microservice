# Multi-stage build for the example worker. Rename the build target and the
# binary when you adapt the template (cmd/<your>-worker).
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/example-worker

FROM alpine:3.19

RUN apk --no-cache add ca-certificates && \
    addgroup -S -g 10001 worker && adduser -S -G worker -u 10001 worker

WORKDIR /app

COPY --from=builder --chown=worker:worker /app/worker .

USER 10001:10001

EXPOSE 8080

CMD ["./worker"]
