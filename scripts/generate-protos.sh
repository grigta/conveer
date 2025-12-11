#!/bin/bash

# Generate all proto files for the project

set -e

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "protoc not found. Installing..."
    apt-get update && apt-get install -y protobuf-compiler
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

export PATH="$PATH:$(go env GOPATH)/bin"

# Generate protos for each service
SERVICES=(
    "proxy-service"
    "sms-service"
    "vk-service"
    "telegram-service"
    "mail-service"
    "max-service"
    "warming-service"
    "analytics-service"
)

for SERVICE in "${SERVICES[@]}"; do
    PROTO_DIR="services/${SERVICE}/proto"
    if [ -d "$PROTO_DIR" ]; then
        echo "Generating protos for ${SERVICE}..."
        for PROTO_FILE in ${PROTO_DIR}/*.proto; do
            if [ -f "$PROTO_FILE" ]; then
                protoc --go_out=. --go_opt=paths=source_relative \
                       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
                       "$PROTO_FILE" 2>/dev/null || true
            fi
        done
    fi
done

echo "Proto generation complete!"

