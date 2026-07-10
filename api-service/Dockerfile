FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/api-service ./cmd/server

FROM alpine:3.19
WORKDIR /app
COPY --from=build /bin/api-service /usr/local/bin/api-service
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api-service"]

