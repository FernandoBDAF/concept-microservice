#!/bin/bash

# Protocol Buffer and gRPC Code Generation Script
# Generates Go code from protobuf definitions

set -e

echo "🔧 Generating protobuf and gRPC code..."

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "❌ protoc (Protocol Buffer Compiler) is not installed"
    echo "   Install with: brew install protobuf (macOS) or apt-get install protobuf-compiler (Ubuntu)"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "❌ protoc-gen-go is not installed"
    echo "   Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "❌ protoc-gen-go-grpc is not installed"
    echo "   Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

# Create output directory
mkdir -p api/proto

# Generate Go code from protobuf definitions
echo "📦 Generating Go code from protobuf..."
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    api/proto/cache.proto

echo "✅ Protobuf code generation completed"
echo "   Generated files:"
echo "   - api/proto/cache.pb.go"
echo "   - api/proto/cache_grpc.pb.go"

# Verify generated files exist
if [ -f "api/proto/cache.pb.go" ] && [ -f "api/proto/cache_grpc.pb.go" ]; then
    echo "✅ All generated files are present"
else
    echo "❌ Generation failed - missing output files"
    exit 1
fi

echo "🎉 gRPC code generation successful!" 