#!/bin/bash

# Generate Go code from proto files
protoc --go_out=. --go-grpc_out=. telegram.proto

echo "Proto files generated successfully"