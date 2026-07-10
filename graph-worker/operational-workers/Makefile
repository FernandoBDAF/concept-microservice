.PHONY: all build test clean docker docker-email docker-image docker-profile mod

all: build

build:
	go build -o bin/email-worker ./cmd/email-worker
	go build -o bin/image-worker ./cmd/image-worker
	go build -o bin/profile-worker ./cmd/profile-worker

test:
	go test ./...

clean:
	rm -rf bin/

docker:
	docker build -f Dockerfile.email -t email-worker:latest .
	docker build -f Dockerfile.image -t image-worker:latest .
	docker build -f Dockerfile.profile -t profile-worker:latest .

docker-email:
	docker build -f Dockerfile.email -t email-worker:latest .

docker-image:
	docker build -f Dockerfile.image -t image-worker:latest .

docker-profile:
	docker build -f Dockerfile.profile -t profile-worker:latest .

mod:
	go mod tidy
