#!/bin/bash

set -e

echo "Setting up Conveer microservices..."

echo "1. Installing Go dependencies..."
go mod download

echo "2. Installing development tools..."
make install-tools

echo "3. Creating necessary directories..."
mkdir -p logs bin tmp

echo "4. Setting up environment..."
if [ ! -f .env ]; then
    cp .env.example .env
    echo "Created .env file from .env.example"
    echo "Please update the .env file with your configuration"
fi

echo "5. Building Docker images..."
docker-compose build

echo "6. Starting infrastructure services..."
docker-compose up -d mongodb redis rabbitmq prometheus grafana loki

echo "7. Waiting for services to be ready..."
sleep 10

echo "8. Setting up MongoDB indexes..."
docker exec conveer-mongodb mongosh --eval "
    use conveer;
    db.users.createIndex({ email: 1 }, { unique: true });
    db.users.createIndex({ username: 1 }, { unique: true });
    db.products.createIndex({ name: 'text', description: 'text' });
    db.products.createIndex({ category: 1 });
    db.products.createIndex({ price: 1 });
    db.sessions.createIndex({ token: 1 });
    db.sessions.createIndex({ refresh_token: 1 });
    db.sessions.createIndex({ expires_at: 1 }, { expireAfterSeconds: 0 });
"

echo "Setup complete!"
echo ""
echo "To start all services, run: make run"
echo "To view logs, run: make logs"
echo "To stop services, run: make stop"