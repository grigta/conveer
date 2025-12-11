# Developer Guide

## Содержание

- [Начало работы](#начало-работы)
- [Архитектура](#архитектура)
- [Разработка сервисов](#разработка-сервисов)
- [Тестирование](#тестирование)
- [Отладка](#отладка)
- [Code Style](#code-style)
- [Работа с Git](#работа-с-git)

---

## Начало работы

### Требования

- Go 1.21+
- Docker & Docker Compose
- Make
- protoc (для gRPC)
- golangci-lint

### Установка инструментов

```bash
# Установить все необходимые инструменты
make install-tools

# Или вручную:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/vektra/mockery/v2@latest
```

### Настройка окружения

```bash
# Клонировать репозиторий
git clone https://github.com/your-org/conveer.git
cd conveer

# Скопировать конфигурацию
cp .env.example .env

# Запустить инфраструктуру
make dev

# Проверить что всё работает
make health
```

### Структура проекта

```
conveer/
├── cmd/                    # Entry points (устаревшее)
├── services/               # Микросервисы
│   ├── api-gateway/
│   ├── proxy-service/
│   ├── sms-service/
│   ├── warming-service/
│   ├── vk-service/
│   ├── telegram-service/
│   ├── mail-service/
│   ├── max-service/
│   ├── analytics-service/
│   └── telegram-bot/
├── pkg/                    # Общие пакеты
│   ├── crypto/            # Шифрование
│   ├── messaging/         # RabbitMQ клиент
│   ├── database/          # MongoDB helpers
│   ├── cache/             # Redis клиент
│   └── testutil/          # Тестовые утилиты
├── proto/                  # Protobuf definitions
├── docs/                   # Документация
├── deploy/                 # Deployment configs
├── monitoring/             # Prometheus, Grafana
├── migrations/             # Database migrations
└── tests/                  # Integration & E2E tests
```

---

## Архитектура

### Микросервисы

```
┌─────────────────┐     ┌─────────────────┐
│  Telegram Bot   │────▶│   API Gateway   │
└─────────────────┘     └────────┬────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
        ▼                        ▼                        ▼
┌───────────────┐      ┌─────────────────┐      ┌─────────────────┐
│ Proxy Service │      │  SMS Service    │      │ Warming Service │
└───────┬───────┘      └────────┬────────┘      └────────┬────────┘
        │                       │                        │
        └───────────────────────┼────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│  VK Service   │      │Telegram Svc   │      │  Mail Service │
└───────────────┘      └───────────────┘      └───────────────┘
```

### Коммуникация

- **HTTP/REST** - Внешние API (Telegram Bot → API Gateway)
- **gRPC** - Внутренняя синхронная коммуникация
- **RabbitMQ** - Асинхронные события и команды

### Шаблоны сообщений RabbitMQ

| Exchange | Routing Key | Описание |
|----------|-------------|----------|
| `conveer.events` | `account.created` | Аккаунт создан |
| `conveer.events` | `account.banned` | Аккаунт забанен |
| `conveer.events` | `proxy.allocated` | Прокси выделен |
| `conveer.events` | `proxy.released` | Прокси освобождён |
| `conveer.events` | `proxy.rotated` | Прокси заменён |
| `conveer.events` | `warming.started` | Прогрев начат |
| `conveer.events` | `warming.completed` | Прогрев завершён |
| `conveer.commands` | `proxy.allocate` | Команда выделить прокси |
| `conveer.commands` | `proxy.release` | Команда освободить прокси |
| `conveer.commands` | `warming.execute_action` | Выполнить действие прогрева |

---

## Разработка сервисов

### Создание нового сервиса

1. **Создать структуру директорий:**

```bash
mkdir -p services/my-service/{cmd,internal/{handler,service,repository,model}}
```

2. **Создать main.go:**

```go
// services/my-service/cmd/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "conveer/pkg/config"
    "conveer/pkg/database"
    "conveer/pkg/messaging"
    "conveer/services/my-service/internal/handler"
    "conveer/services/my-service/internal/service"
)

func main() {
    cfg := config.Load()
    
    // Initialize dependencies
    db := database.NewMongoDB(cfg.MongoURI)
    rabbit := messaging.NewRabbitMQ(cfg.RabbitURL)
    
    // Create service
    svc := service.NewMyService(db, rabbit)
    
    // Create handlers
    httpHandler := handler.NewHTTPHandler(svc)
    grpcHandler := handler.NewGRPCHandler(svc)
    
    // Start servers
    go httpHandler.Start(cfg.HTTPPort)
    go grpcHandler.Start(cfg.GRPCPort)
    
    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    httpHandler.Shutdown(ctx)
    grpcHandler.Stop()
    db.Close()
    rabbit.Close()
}
```

3. **Реализовать интерфейсы:**

```go
// services/my-service/internal/service/service.go
package service

type MyService interface {
    DoSomething(ctx context.Context, req *DoSomethingRequest) (*DoSomethingResponse, error)
}

type myService struct {
    repo   Repository
    rabbit *messaging.RabbitMQ
    cache  *cache.Redis
}

func NewMyService(repo Repository, rabbit *messaging.RabbitMQ, cache *cache.Redis) MyService {
    return &myService{
        repo:   repo,
        rabbit: rabbit,
        cache:  cache,
    }
}
```

4. **Добавить Dockerfile:**

```dockerfile
# services/my-service/Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /my-service ./services/my-service/cmd

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
COPY --from=builder /my-service /my-service
EXPOSE 8080 50051
CMD ["/my-service"]
```

### Dependency Injection

Используем constructor injection:

```go
type ProxyService struct {
    repo           ProxyRepository
    providerMgr    *ProviderManager
    healthChecker  *HealthChecker
    rotationMgr    *RotationManager
    rabbit         *messaging.RabbitMQ
    cache          *cache.Redis
    metrics        *ProxyMetrics
}

func NewProxyService(
    repo ProxyRepository,
    providerMgr *ProviderManager,
    healthChecker *HealthChecker,
    rotationMgr *RotationManager,
    rabbit *messaging.RabbitMQ,
    cache *cache.Redis,
) *ProxyService {
    return &ProxyService{
        repo:          repo,
        providerMgr:   providerMgr,
        healthChecker: healthChecker,
        rotationMgr:   rotationMgr,
        rabbit:        rabbit,
        cache:         cache,
        metrics:       NewProxyMetrics(),
    }
}
```

### Работа с RabbitMQ

**Публикация события:**

```go
func (s *ProxyService) AllocateProxy(ctx context.Context, accountID string) (*Proxy, error) {
    proxy, err := s.allocateInternal(ctx, accountID)
    if err != nil {
        return nil, err
    }
    
    // Публикуем событие
    event := map[string]interface{}{
        "proxy_id":   proxy.ID,
        "account_id": accountID,
        "timestamp":  time.Now(),
    }
    if err := s.rabbit.Publish("conveer.events", "proxy.allocated", event); err != nil {
        log.Printf("Failed to publish event: %v", err)
    }
    
    return proxy, nil
}
```

**Подписка на события:**

```go
func (s *WarmingService) StartWorkers(ctx context.Context) {
    s.rabbit.ConsumeWithHandler(
        "warming.execute_action",
        func(msg *messaging.Message) error {
            var cmd ExecuteActionCommand
            if err := json.Unmarshal(msg.Body, &cmd); err != nil {
                return err
            }
            return s.executeAction(ctx, &cmd)
        },
    )
}
```

---

## Тестирование

### Unit Tests

```go
// services/proxy-service/internal/service/proxy_service_test.go
package service

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestAllocateProxy_Success(t *testing.T) {
    // Arrange
    mockRepo := new(MockProxyRepository)
    mockRabbit := new(MockRabbitMQ)
    mockCache := new(MockRedisCache)
    
    svc := NewProxyService(mockRepo, nil, nil, nil, mockRabbit, mockCache)
    
    expectedProxy := &Proxy{ID: "proxy-1", Host: "1.2.3.4", Port: 8080}
    mockRepo.On("FindAvailable", mock.Anything).Return(expectedProxy, nil)
    mockRepo.On("UpdateStatus", mock.Anything, "proxy-1", "allocated").Return(nil)
    mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything).Return(nil)
    mockRabbit.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
    
    // Act
    proxy, err := svc.AllocateProxy(context.Background(), "account-1")
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expectedProxy.ID, proxy.ID)
    mockRepo.AssertExpectations(t)
}
```

### Integration Tests

```go
// tests/integration/proxy_service_test.go
package integration

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/suite"
    "conveer/pkg/testutil"
)

type ProxyServiceIntegrationSuite struct {
    suite.Suite
    mongoContainer *testutil.MongoContainer
    redisContainer *testutil.RedisContainer
    svc            *service.ProxyService
}

func (s *ProxyServiceIntegrationSuite) SetupSuite() {
    ctx := context.Background()
    
    var err error
    s.mongoContainer, err = testutil.NewMongoContainer(ctx)
    s.Require().NoError(err)
    
    s.redisContainer, err = testutil.NewRedisContainer(ctx)
    s.Require().NoError(err)
    
    // Initialize service with real connections
    // ...
}

func (s *ProxyServiceIntegrationSuite) TearDownSuite() {
    s.mongoContainer.Terminate(context.Background())
    s.redisContainer.Terminate(context.Background())
}

func (s *ProxyServiceIntegrationSuite) TestAllocateProxy_Integration() {
    // Test with real MongoDB and Redis
}

func TestProxyServiceIntegration(t *testing.T) {
    suite.Run(t, new(ProxyServiceIntegrationSuite))
}
```

### Запуск тестов

```bash
# Все тесты
make test

# Только unit тесты
make test-unit

# Integration тесты
make test-integration

# С покрытием
make test-coverage

# HTML отчёт покрытия
make test-coverage-html
open coverage.html
```

### Генерация моков

```bash
# Сгенерировать все моки
make mock-generate

# Или для конкретного интерфейса
mockery --name=ProxyRepository --dir=services/proxy-service/internal/repository --output=services/proxy-service/internal/mocks
```

---

## Отладка

### Локальная отладка

1. **Запустить инфраструктуру:**
```bash
make dev
```

2. **Запустить сервис в режиме отладки:**
```bash
cd services/proxy-service
go run cmd/main.go
```

3. **Или с air (hot reload):**
```bash
air -c .air.toml
```

### Просмотр логов

```bash
# Docker Compose
docker-compose logs -f proxy-service

# Kubernetes
kubectl logs -f deployment/proxy-service -n conveer

# Loki (через Grafana)
# Explore → Loki → {app="proxy-service"}
```

### Метрики

```bash
# Prometheus UI
open http://localhost:9090

# Запросы:
# rate(http_requests_total{service="proxy-service"}[5m])
# histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### RabbitMQ Management

```bash
open http://localhost:15672
# guest / guest (dev only)
```

### MongoDB queries

```bash
# Подключиться к MongoDB
docker exec -it conveer-mongodb mongosh -u admin -p password

# Примеры запросов
use conveer
db.proxies.find({status: "available"}).limit(10)
db.accounts.countDocuments({platform: "vk", status: "active"})
```

---

## Code Style

### Go Style Guide

Следуем [Effective Go](https://golang.org/doc/effective_go) и [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

### Linting

```bash
# Запустить линтер
make lint

# Конфигурация в .golangci.yml
```

### Именование

```go
// Интерфейсы - существительные или с суффиксом -er
type ProxyRepository interface {}
type HealthChecker interface {}

// Структуры - существительные
type ProxyService struct {}

// Методы - глаголы
func (s *ProxyService) AllocateProxy() {}
func (s *ProxyService) GetProxyByID() {}

// Константы - CamelCase для экспортируемых
const MaxRetries = 3
const defaultTimeout = 30 * time.Second
```

### Обработка ошибок

```go
// Оборачивать ошибки с контекстом
if err := s.repo.Save(ctx, proxy); err != nil {
    return fmt.Errorf("failed to save proxy %s: %w", proxy.ID, err)
}

// Определять sentinel errors
var (
    ErrProxyNotFound = errors.New("proxy not found")
    ErrNoAvailableProxies = errors.New("no available proxies")
)

// Проверять типы ошибок
if errors.Is(err, ErrProxyNotFound) {
    // handle not found
}
```

---

## Работа с Git

### Branching Strategy

```
main          ─────●─────●─────●─────●───▶
                   │     │     │
feature/*     ─────●─────┘     │
                               │
hotfix/*      ─────────────────●
```

### Commit Messages

Формат: `<type>(<scope>): <description>`

```
feat(proxy): add automatic proxy rotation
fix(warming): fix race condition in scheduler
docs(api): update OpenAPI specification
test(sms): add integration tests for retry logic
refactor(crypto): extract encryption interface
chore(deps): update go.mod dependencies
```

### Pull Request Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Security scan passes (`make security`)
- [ ] Documentation updated (if needed)
- [ ] CHANGELOG updated (for features/fixes)
- [ ] PR description explains the change

