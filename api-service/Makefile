BIN=api-service

.PHONY: build run test

build:
	go build -o bin/$(BIN) ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

