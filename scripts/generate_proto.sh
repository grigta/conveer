#!/bin/bash

# Generate protobuf files for all services
services=("vk-service" "telegram-service" "mail-service" "max-service" "warming-service" "sms-service" "proxy-service")

for service in "${services[@]}"; do
    echo "Generating proto for $service..."
    if [ -f "services/$service/proto/*.proto" ]; then
        protoc --go_out=. --go_opt=paths=source_relative \
            --go-grpc_out=. --go-grpc_opt=paths=source_relative \
            services/$service/proto/*.proto
    fi
done

echo "Proto generation complete!"