#!/bin/bash

# Exit on error
set -e

# Create proto directory if it doesn't exist
mkdir -p proto/profile

# Generate Go code from protobuf
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/profile/profile.proto 