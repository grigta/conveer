#!/bin/bash

set -e

echo "Running tests for Conveer microservices..."

echo "1. Running unit tests..."
go test -v -race ./pkg/...

echo "2. Running integration tests..."
go test -v -race ./services/...

echo "3. Running test coverage..."
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

echo "4. Running linter..."
golangci-lint run --timeout 5m

echo "5. Running security checks..."
go list -json -deps ./... | nancy sleuth

echo "All tests passed successfully!"