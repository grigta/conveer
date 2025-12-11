.PHONY: help
help:
	@echo "====================== Conveer Makefile ======================"
	@echo ""
	@echo "Development:"
	@echo "  make run              - Start all services with docker-compose"
	@echo "  make stop             - Stop all services"
	@echo "  make restart          - Restart all services"
	@echo "  make logs             - Show logs from all services"
	@echo "  make dev              - Start development environment"
	@echo "  make build            - Build all services"
	@echo "  make clean            - Clean up containers and volumes"
	@echo ""
	@echo "Testing:"
	@echo "  make test             - Run all tests"
	@echo "  make test-unit        - Run only unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-e2e         - Run end-to-end tests"
	@echo "  make test-coverage    - Run tests with coverage report"
	@echo "  make test-coverage-html - Generate HTML coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint             - Run linter (golangci-lint)"
	@echo "  make fmt              - Format code"
	@echo "  make vet              - Run go vet"
	@echo "  make security         - Run security scan (gosec)"
	@echo ""
	@echo "Generation:"
	@echo "  make proto            - Generate protobuf files"
	@echo "  make proto-all        - Generate protobuf for all services"
	@echo "  make mock-generate    - Generate mock objects"
	@echo "  make swagger-generate - Generate Swagger documentation"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build-all - Build all Docker images"
	@echo "  make docker-push-all  - Push all images to registry"
	@echo ""
	@echo "Deployment:"
	@echo "  make deploy-staging   - Deploy to staging environment"
	@echo "  make deploy-prod      - Deploy to production environment"
	@echo ""
	@echo "Utilities:"
	@echo "  make install-tools    - Install development tools"
	@echo "  make health           - Check service health"
	@echo "  make mod-tidy         - Tidy Go modules"

# ==================== Development ====================

.PHONY: build
build:
	@echo "Building all services..."
	docker-compose build

.PHONY: run
run:
	@echo "Starting all services..."
	docker-compose up -d

.PHONY: stop
stop:
	@echo "Stopping all services..."
	docker-compose down

.PHONY: restart
restart: stop run

.PHONY: clean
clean:
	@echo "Cleaning up containers and volumes..."
	docker-compose down -v
	docker system prune -f
	rm -rf bin/ coverage.out coverage.html

.PHONY: logs
logs:
	docker-compose logs -f

.PHONY: logs-proxy
logs-proxy:
	docker-compose logs -f proxy-service

.PHONY: logs-sms
logs-sms:
	docker-compose logs -f sms-service

.PHONY: logs-warming
logs-warming:
	docker-compose logs -f warming-service

.PHONY: logs-vk
logs-vk:
	docker-compose logs -f vk-service

.PHONY: logs-telegram
logs-telegram:
	docker-compose logs -f telegram-service

.PHONY: dev
dev:
	@echo "Starting development environment..."
	docker-compose -f docker-compose.yml up mongodb redis rabbitmq -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "Development infrastructure is ready!"

# ==================== Testing ====================

.PHONY: test
test:
	@echo "Running all tests..."
	go test -v -race ./...

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v -short -race ./...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -v -race -run Integration ./...

.PHONY: test-e2e
test-e2e:
	@echo "Running end-to-end tests..."
	go test -v -race -run E2E ./tests/e2e/...

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report generated: coverage.out"
	go tool cover -func=coverage.out | grep total

.PHONY: test-coverage-html
test-coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"

.PHONY: test-bench
test-bench:
	@echo "Running benchmark tests..."
	go test -v -bench=. -benchmem ./...

# ==================== Code Quality ====================

.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run --timeout 5m ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: security
security:
	@echo "Running security scan..."
	gosec -quiet ./...
	@echo "Running dependency vulnerability scan..."
	nancy sleuth -p go.sum

.PHONY: check
check: lint vet security
	@echo "All checks passed!"

# ==================== Generation ====================

.PHONY: proto
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto

.PHONY: proto-all
proto-all:
	@echo "Generating protobuf files for all services..."
	./scripts/generate_proto.sh

.PHONY: mock-generate
mock-generate:
	@echo "Generating mocks..."
	mockery --all --dir pkg --output pkg/mocks --case underscore
	mockery --all --dir services/proxy-service/internal --output services/proxy-service/internal/mocks --case underscore
	mockery --all --dir services/sms-service/internal --output services/sms-service/internal/mocks --case underscore
	mockery --all --dir services/warming-service/internal --output services/warming-service/internal/mocks --case underscore

.PHONY: swagger-generate
swagger-generate:
	@echo "Generating Swagger documentation..."
	swag init -g services/api-gateway/cmd/main.go -o docs/swagger
	@echo "Swagger documentation generated in docs/swagger/"

# ==================== Docker ====================

.PHONY: docker-build-all
docker-build-all:
	@echo "Building all Docker images..."
	docker build -f services/api-gateway/Dockerfile -t conveer/api-gateway:latest .
	docker build -f services/proxy-service/Dockerfile -t conveer/proxy-service:latest .
	docker build -f services/sms-service/Dockerfile -t conveer/sms-service:latest .
	docker build -f services/vk-service/Dockerfile -t conveer/vk-service:latest .
	docker build -f services/telegram-service/Dockerfile -t conveer/telegram-service:latest .
	docker build -f services/mail-service/Dockerfile -t conveer/mail-service:latest .
	docker build -f services/max-service/Dockerfile -t conveer/max-service:latest .
	docker build -f services/warming-service/Dockerfile -t conveer/warming-service:latest .
	docker build -f services/analytics-service/Dockerfile -t conveer/analytics-service:latest .
	docker build -f services/telegram-bot/Dockerfile -t conveer/telegram-bot:latest .

.PHONY: docker-push-all
docker-push-all:
	@echo "Pushing all Docker images..."
	docker push conveer/api-gateway:latest
	docker push conveer/proxy-service:latest
	docker push conveer/sms-service:latest
	docker push conveer/vk-service:latest
	docker push conveer/telegram-service:latest
	docker push conveer/mail-service:latest
	docker push conveer/max-service:latest
	docker push conveer/warming-service:latest
	docker push conveer/analytics-service:latest
	docker push conveer/telegram-bot:latest

# ==================== Deployment ====================

.PHONY: deploy-staging
deploy-staging:
	@echo "Deploying to staging..."
	helm upgrade --install conveer ./deploy/helm/conveer \
		--namespace conveer-staging \
		--create-namespace \
		-f ./deploy/helm/conveer/values-staging.yaml

.PHONY: deploy-prod
deploy-prod:
	@echo "Deploying to production..."
	@read -p "Are you sure you want to deploy to production? [y/N] " confirm && \
		[ "$$confirm" = "y" ] || exit 1
	helm upgrade --install conveer ./deploy/helm/conveer \
		--namespace conveer-prod \
		--create-namespace \
		-f ./deploy/helm/conveer/values-prod.yaml

# ==================== Utilities ====================

.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/vektra/mockery/v2@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/sonatype-nexus-community/nancy@latest

.PHONY: mod-download
mod-download:
	@echo "Downloading Go modules..."
	go mod download

.PHONY: mod-tidy
mod-tidy:
	@echo "Tidying Go modules..."
	go mod tidy

.PHONY: health
health:
	@echo "Checking service health..."
	@echo "API Gateway:"
	@curl -sf http://localhost:8080/health && echo " OK" || echo " FAIL"
	@echo "Proxy Service:"
	@curl -sf http://localhost:8081/health && echo " OK" || echo " FAIL"
	@echo "SMS Service:"
	@curl -sf http://localhost:8082/health && echo " OK" || echo " FAIL"
	@echo "Warming Service:"
	@curl -sf http://localhost:8083/health && echo " OK" || echo " FAIL"
	@echo "Prometheus:"
	@curl -sf http://localhost:9090/-/healthy && echo " OK" || echo " FAIL"
	@echo "Grafana:"
	@curl -sf http://localhost:3000/api/health && echo " OK" || echo " FAIL"
	@echo "RabbitMQ:"
	@curl -sf http://localhost:15672 && echo " OK" || echo " FAIL"

.PHONY: migrate-up
migrate-up:
	@echo "Running migrations up..."
	migrate -path ./migrations -database "$(MONGODB_URI)" up

.PHONY: migrate-down
migrate-down:
	@echo "Running migrations down..."
	migrate -path ./migrations -database "$(MONGODB_URI)" down

.PHONY: migrate-create
migrate-create:
	@read -p "Migration name: " name && \
		migrate create -ext json -dir ./migrations -seq $$name

# ==================== Build Individual Services ====================

.PHONY: build-proxy
build-proxy:
	@echo "Building Proxy Service..."
	CGO_ENABLED=0 go build -o bin/proxy-service ./services/proxy-service/cmd/main.go

.PHONY: build-sms
build-sms:
	@echo "Building SMS Service..."
	CGO_ENABLED=0 go build -o bin/sms-service ./services/sms-service/cmd/main.go

.PHONY: build-warming
build-warming:
	@echo "Building Warming Service..."
	CGO_ENABLED=0 go build -o bin/warming-service ./services/warming-service/cmd/main.go

.PHONY: build-vk
build-vk:
	@echo "Building VK Service..."
	CGO_ENABLED=0 go build -o bin/vk-service ./services/vk-service/cmd/main.go

.PHONY: build-telegram
build-telegram:
	@echo "Building Telegram Service..."
	CGO_ENABLED=0 go build -o bin/telegram-service ./services/telegram-service/cmd/main.go

.PHONY: build-gateway
build-gateway:
	@echo "Building API Gateway..."
	CGO_ENABLED=0 go build -o bin/api-gateway ./services/api-gateway/cmd/main.go

.PHONY: build-bot
build-bot:
	@echo "Building Telegram Bot..."
	CGO_ENABLED=0 go build -o bin/telegram-bot ./services/telegram-bot/cmd/main.go

.PHONY: build-binaries
build-binaries: build-gateway build-proxy build-sms build-warming build-vk build-telegram build-bot
	@echo "All binaries built successfully!"
