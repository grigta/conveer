# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Development Environment
```bash
# Start development infrastructure (MongoDB, Redis, RabbitMQ)
make dev

# Start all services
make run

# Stop all services
make stop

# Clean up containers and volumes
make clean

# View logs from all services
make logs

# View logs from specific service
make logs-proxy    # or logs-sms, logs-warming, logs-vk, logs-telegram
```

### Testing
```bash
# Run all tests
make test

# Run specific test types
make test-unit        # Unit tests only
make test-integration # Integration tests only
make test-e2e         # End-to-end tests
make test-coverage    # Generate coverage report
make test-coverage-html # Generate HTML coverage report

# Run tests for specific service
go test -v ./services/proxy-service/...

# Run single test
go test -v -run TestProxyAllocation ./services/proxy-service/internal/service
```

### Code Quality
```bash
# Run linter (golangci-lint)
make lint

# Format code
make fmt

# Run security scan
make security

# Run all checks
make check
```

### Build Services
```bash
# Build all services with Docker
make build

# Build individual service binaries
make build-proxy    # or build-sms, build-warming, build-vk, etc.
CGO_ENABLED=0 go build -o bin/proxy-service ./services/proxy-service/cmd/main.go
```

### Code Generation
```bash
# Generate protobuf files
make proto-all

# Generate mocks
make mock-generate

# Generate Swagger documentation
make swagger-generate
```

## Architecture Overview

### Microservices Architecture
The system consists of 10+ microservices communicating via gRPC and RabbitMQ:

1. **API Gateway** (`services/api-gateway/`) - Single entry point, routes requests to appropriate services
2. **Proxy Service** (`services/proxy-service/`) - Manages proxy providers, allocations, rotations, health checks
3. **SMS Service** (`services/sms-service/`) - Integrates with SMS-Activate API for phone number verification
4. **Platform Services** - Account registration and management:
   - VK Service (`services/vk-service/`)
   - Telegram Service (`services/telegram-service/`)
   - Mail Service (`services/mail-service/`)
   - Max Service (`services/max-service/`)
5. **Warming Service** (`services/warming-service/`) - Executes account warming scenarios (14-60 days)
6. **Analytics Service** (`services/analytics-service/`) - Metrics aggregation, forecasting, recommendations
7. **Telegram Bot** (`services/telegram-bot/`) - User interface via Telegram

### Communication Patterns

**gRPC** (synchronous):
- Service-to-service direct calls
- Request/response pattern
- Proto definitions in `services/*/proto/`

**RabbitMQ** (asynchronous):
- Event-driven communication
- Task queues for warming actions
- Alert notifications

**Redis**:
- Distributed caching
- Session management
- Rate limiting
- Temporary data storage

### Data Flow

1. **Account Creation Flow**:
   ```
   Telegram Bot → API Gateway → Platform Service → Proxy Service (allocate)
                                                 → SMS Service (get number)
                                                 → MongoDB (save account)
   ```

2. **Warming Flow**:
   ```
   Warming Service → Platform Service (execute actions)
                  → Proxy Service (rotate if needed)
                  → RabbitMQ (queue actions)
                  → MongoDB (track progress)
   ```

3. **Analytics Flow**:
   ```
   Analytics Service → Prometheus (pull metrics)
                    → Platform Services (gRPC GetStatistics)
                    → MongoDB (aggregate & store)
                    → Redis (cache recommendations)
   ```

### Key Design Patterns

**Repository Pattern**: All services use repository pattern for data access
- Interface: `internal/repository/interfaces.go`
- Implementation: `internal/repository/*_repository.go`

**Service Layer**: Business logic isolated in service layer
- `internal/service/*_service.go`

**Dependency Injection**: Dependencies injected via constructors
- Example: `NewProxyService(repo, cache, logger)`

**Error Handling**: Consistent error wrapping and propagation
- Custom errors in `pkg/errors/`
- gRPC status codes for API errors

**Encryption**: AES-256 GCM for sensitive data
- Credentials encrypted in MongoDB
- Implementation in `pkg/crypto/`

## Service-Specific Notes

### Proxy Service
- Manages multiple providers via YAML config (`config/providers.yaml`)
- Health checks run every 5 minutes with IPQS fraud score validation
- Grace period of 15 minutes during proxy rotation
- Provider stats tracked for recommendations

### SMS Service
- Implements exponential backoff for SMS retrieval (max 10 retries)
- Caches active activations in Redis
- Balance monitoring with alerts

### Warming Service
- Scenarios defined in YAML (`config/warming_scenarios/`)
- Implements human-like behavior patterns (random delays, burst actions)
- State machine for task management (pending → in_progress → completed/failed)

### Analytics Service
- Aggregates metrics every 5 minutes from Prometheus
- Generates recommendations every 6 hours
- Forecasting uses ARIMA model via gonum
- Redis cache with TTL: 6h for proxy rankings, 24h for others

### Platform Services (VK, Telegram, Mail, Max)
- Share common interfaces for account management
- Implement platform-specific registration logic
- Max service depends on VK service (creates VK first, then links)

## Configuration

Services use YAML configs in `configs/` directory:
- `*_config.yaml` - service-specific configuration
- Environment variables override config values
- Required env vars: `MONGODB_URI`, `REDIS_URL`, `RABBITMQ_URL`, `ENCRYPTION_KEY`, `SMS_ACTIVATE_API_KEY`

## Database Schema

MongoDB collections:
- `accounts` - platform accounts with encrypted credentials
- `proxies` - proxy inventory and bindings
- `warming_tasks` - warming progress tracking
- `aggregated_metrics` - time-series metrics (90-day TTL)
- `forecasts` - ML predictions
- `recommendations` - generated recommendations
- `alerts` - system alerts

Indexes are created automatically on service startup (see `setupIndexes()` in main.go files).

## Testing Strategy

- **Unit tests**: Mock external dependencies, test business logic
- **Integration tests**: Use testcontainers for MongoDB/Redis/RabbitMQ
- **E2E tests**: Full flow testing in `tests/e2e/`
- Coverage target: >80% for critical components (crypto, proxy logic, warming scheduler)

## Monitoring

- Metrics exposed on `:9090/metrics` (Prometheus format)
- Custom metrics registered in `internal/service/metrics.go`
- Grafana dashboards in `docker/grafana/dashboards/`
- Health endpoints: `GET /health` on each service

## Security Considerations

- All credentials encrypted with AES-256 GCM
- JWT tokens for API authentication
- Rate limiting on API Gateway
- Telegram bot whitelist (admin IDs only)
- Proxy fraud score validation via IPQS