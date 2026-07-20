FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o profile-worker ./cmd/profile-worker

FROM alpine:3.19

RUN apk --no-cache add ca-certificates && \
    addgroup -S -g 10001 worker && adduser -S -G worker -u 10001 worker

WORKDIR /app

COPY --from=builder --chown=worker:worker /app/profile-worker .

USER 10001:10001

EXPOSE 8080

CMD ["./profile-worker"]
