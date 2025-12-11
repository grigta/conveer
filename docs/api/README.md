# API Documentation

Conveer предоставляет HTTP REST API и gRPC API для взаимодействия с сервисами.

## Обзор

### Базовые URL

| Окружение | HTTP API | gRPC |
|-----------|----------|------|
| Development | `http://localhost:8080` | `localhost:9090` |
| Staging | `https://api.staging.conveer.io` | `grpc.staging.conveer.io:443` |
| Production | `https://api.conveer.io` | `grpc.conveer.io:443` |

### Аутентификация

#### JWT Token

```bash
curl -X POST /api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "secret"}'
```

Ответ:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600
}
```

Использование:
```bash
curl -X GET /api/v1/proxies/statistics \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

#### API Key (для сервисных интеграций)

```bash
curl -X GET /api/v1/proxies/statistics \
  -H "X-API-Key: your-api-key"
```

## HTTP API Endpoints

### Proxy Service

#### Выделение прокси

```http
POST /api/v1/proxies/allocate
```

**Request:**
```json
{
  "account_id": "60d5ecb54b24e1234567890a",
  "type": "mobile",
  "country": "RU",
  "protocol": "http"
}
```

**Response (200):**
```json
{
  "id": "60d5ecb54b24e1234567890b",
  "ip": "192.168.1.100",
  "port": 8080,
  "protocol": "http",
  "username": "proxy_user",
  "password": "proxy_pass",
  "type": "mobile",
  "country": "RU",
  "city": "Moscow",
  "expires_at": "2024-01-16T10:00:00Z"
}
```

#### Освобождение прокси

```http
POST /api/v1/proxies/release
```

**Request:**
```json
{
  "account_id": "60d5ecb54b24e1234567890a"
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Proxy released successfully"
}
```

#### Получение прокси для аккаунта

```http
GET /api/v1/proxies/account/:account_id
```

**Response (200):**
```json
{
  "id": "60d5ecb54b24e1234567890b",
  "ip": "192.168.1.100",
  "port": 8080,
  "protocol": "http",
  "status": "active"
}
```

#### Принудительная ротация

```http
POST /api/v1/proxies/rotate
```

**Request:**
```json
{
  "account_id": "60d5ecb54b24e1234567890a"
}
```

#### Статистика

```http
GET /api/v1/proxies/statistics
```

**Response (200):**
```json
{
  "total_proxies": 100,
  "active_proxies": 85,
  "expired_proxies": 10,
  "banned_proxies": 5,
  "total_bindings": 75,
  "proxies_by_type": {
    "mobile": 60,
    "residential": 40
  },
  "proxies_by_country": {
    "RU": 50,
    "US": 30,
    "DE": 20
  },
  "avg_fraud_score": 25.5,
  "avg_latency": 150.0
}
```

### SMS Service

#### Покупка номера

```http
POST /api/v1/sms/purchase
```

**Request:**
```json
{
  "service": "vk",
  "country": "RU",
  "user_id": "user123"
}
```

**Response (200):**
```json
{
  "activation_id": "123456789",
  "phone_number": "+79991234567",
  "expires_at": "2024-01-15T10:20:00Z"
}
```

#### Получение SMS кода

```http
GET /api/v1/sms/code/:activation_id
```

**Response (200):**
```json
{
  "activation_id": "123456789",
  "status": "STATUS_OK",
  "code": "1234"
}
```

**Response (202 - ожидание):**
```json
{
  "activation_id": "123456789",
  "status": "STATUS_WAIT_CODE",
  "code": null
}
```

#### Отмена активации

```http
POST /api/v1/sms/cancel/:activation_id
```

#### Баланс

```http
GET /api/v1/sms/balance
```

**Response (200):**
```json
{
  "balance": 1500.50,
  "currency": "RUB"
}
```

### VK Service

#### Создание аккаунта

```http
POST /api/v1/vk/accounts
```

**Request:**
```json
{
  "user_id": "user123",
  "proxy_type": "mobile",
  "country": "RU"
}
```

**Response (202):**
```json
{
  "account_id": "60d5ecb54b24e1234567890c",
  "status": "registration_started",
  "message": "Account registration in progress"
}
```

#### Получение аккаунта

```http
GET /api/v1/vk/accounts/:id
```

**Response (200):**
```json
{
  "id": "60d5ecb54b24e1234567890c",
  "vk_id": "123456789",
  "phone_number": "+79991234567",
  "status": "active",
  "created_at": "2024-01-15T10:00:00Z",
  "warming_status": "in_progress",
  "warming_day": 5
}
```

#### Список аккаунтов

```http
GET /api/v1/vk/accounts?status=active&page=1&limit=20
```

### Warming Service

#### Создание задачи прогрева

```http
POST /api/v1/warming/tasks
```

**Request:**
```json
{
  "account_id": "60d5ecb54b24e1234567890c",
  "platform": "vk",
  "scenario_type": "basic",
  "duration_days": 14
}
```

**Response (201):**
```json
{
  "task_id": "60d5ecb54b24e1234567890d",
  "status": "scheduled",
  "estimated_completion": "2024-01-29T10:00:00Z"
}
```

#### Статус задачи

```http
GET /api/v1/warming/tasks/:id
```

**Response (200):**
```json
{
  "id": "60d5ecb54b24e1234567890d",
  "account_id": "60d5ecb54b24e1234567890c",
  "platform": "vk",
  "scenario_type": "basic",
  "status": "in_progress",
  "current_day": 5,
  "duration_days": 14,
  "actions_completed": 45,
  "next_action_at": "2024-01-20T14:30:00Z"
}
```

#### Пауза/Возобновление/Остановка

```http
POST /api/v1/warming/tasks/:id/pause
POST /api/v1/warming/tasks/:id/resume
POST /api/v1/warming/tasks/:id/stop
```

#### Кастомный сценарий

```http
POST /api/v1/warming/scenarios
```

**Request:**
```json
{
  "name": "aggressive-vk",
  "platform": "vk",
  "description": "Агрессивный сценарий для быстрого прогрева",
  "actions": [
    {"type": "like_post", "weight": 50},
    {"type": "view_feed", "weight": 30},
    {"type": "subscribe", "weight": 15},
    {"type": "comment", "weight": 5}
  ],
  "schedule": {
    "days_1_7": {"actions_per_day": "10-15"},
    "days_8_14": {"actions_per_day": "15-25"}
  }
}
```

### Analytics Service

#### Общие метрики

```http
GET /api/v1/analytics/metrics?platform=vk&period=7d
```

**Response (200):**
```json
{
  "accounts": {
    "total": 500,
    "active": 450,
    "banned": 20,
    "warming": 30
  },
  "registrations": {
    "total": 50,
    "successful": 45,
    "failed": 5,
    "success_rate": 90.0
  },
  "warming": {
    "tasks_total": 30,
    "tasks_completed": 25,
    "avg_duration_days": 16.5
  },
  "costs": {
    "proxies": 5000.0,
    "sms": 2500.0,
    "total": 7500.0
  }
}
```

#### Прогнозы

```http
GET /api/v1/analytics/forecasts
```

**Response (200):**
```json
{
  "costs_next_week": 8000.0,
  "accounts_ready_estimate": "2024-01-25",
  "proxy_expiry_warning": 5,
  "low_balance_warning": false
}
```

## gRPC API

### Proxy Service

```protobuf
service ProxyService {
  rpc AllocateProxy(AllocateProxyRequest) returns (Proxy);
  rpc ReleaseProxy(ReleaseProxyRequest) returns (Empty);
  rpc GetProxyForAccount(GetProxyRequest) returns (Proxy);
  rpc ForceRotateProxy(RotateProxyRequest) returns (Proxy);
  rpc GetProxyStatistics(Empty) returns (ProxyStatistics);
}
```

### SMS Service

```protobuf
service SMSService {
  rpc PurchaseNumber(PurchaseNumberRequest) returns (Activation);
  rpc GetSMSCode(GetCodeRequest) returns (CodeResponse);
  rpc CancelActivation(CancelRequest) returns (Empty);
  rpc GetActivationStatus(StatusRequest) returns (Activation);
  rpc GetStatistics(Empty) returns (SMSStatistics);
  rpc GetProviderBalance(Empty) returns (BalanceResponse);
}
```

### VK Service

```protobuf
service VKService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc GetAccount(GetAccountRequest) returns (Account);
  rpc ListAccounts(ListAccountsRequest) returns (AccountList);
  rpc UpdateAccountStatus(UpdateStatusRequest) returns (Account);
  rpc GetAccountCredentials(CredentialsRequest) returns (Credentials);
  rpc GetStatistics(Empty) returns (VKStatistics);
}
```

### Warming Service

```protobuf
service WarmingService {
  rpc StartWarming(StartWarmingRequest) returns (WarmingTask);
  rpc PauseWarming(TaskRequest) returns (WarmingTask);
  rpc ResumeWarming(TaskRequest) returns (WarmingTask);
  rpc StopWarming(TaskRequest) returns (WarmingTask);
  rpc GetWarmingStatus(TaskRequest) returns (WarmingTask);
  rpc GetWarmingStatistics(StatisticsRequest) returns (WarmingStatistics);
  rpc CreateCustomScenario(WarmingScenario) returns (WarmingScenario);
  rpc UpdateCustomScenario(UpdateScenarioRequest) returns (WarmingScenario);
  rpc ListScenarios(ListScenariosRequest) returns (ScenarioList);
  rpc ListTasks(ListTasksRequest) returns (TaskList);
}
```

### Analytics Service

```protobuf
service AnalyticsService {
  rpc GetMetrics(MetricsRequest) returns (Metrics);
  rpc GetForecasts(ForecastsRequest) returns (Forecasts);
  rpc GetRecommendations(RecommendationsRequest) returns (Recommendations);
  rpc GetAlerts(AlertsRequest) returns (AlertList);
}
```

### Примеры вызовов grpcurl

```bash
# Выделение прокси
grpcurl -plaintext -d '{
  "account_id": "60d5ecb54b24e1234567890a",
  "type": "mobile",
  "country": "RU"
}' localhost:9090 proxy.ProxyService/AllocateProxy

# Получение статуса прогрева
grpcurl -plaintext -d '{
  "task_id": "60d5ecb54b24e1234567890d"
}' localhost:9090 warming.WarmingService/GetWarmingStatus

# Покупка SMS номера
grpcurl -plaintext -d '{
  "service": "vk",
  "country": "RU",
  "user_id": "user123"
}' localhost:9090 sms.SMSService/PurchaseNumber
```

## Коды ошибок

### HTTP

| Код | Описание |
|-----|----------|
| 200 | Успешно |
| 201 | Создано |
| 202 | Принято (асинхронная операция) |
| 400 | Некорректный запрос |
| 401 | Не авторизован |
| 403 | Доступ запрещен |
| 404 | Не найдено |
| 409 | Конфликт (дубликат) |
| 429 | Слишком много запросов |
| 500 | Внутренняя ошибка сервера |
| 503 | Сервис недоступен |

### gRPC

| Код | Описание |
|-----|----------|
| OK | Успешно |
| INVALID_ARGUMENT | Некорректные параметры |
| NOT_FOUND | Ресурс не найден |
| ALREADY_EXISTS | Ресурс уже существует |
| PERMISSION_DENIED | Доступ запрещен |
| RESOURCE_EXHAUSTED | Превышен лимит |
| INTERNAL | Внутренняя ошибка |
| UNAVAILABLE | Сервис недоступен |

## Rate Limiting

| Endpoint | Лимит |
|----------|-------|
| `/api/v1/auth/*` | 10 req/min |
| `/api/v1/proxies/*` | 100 req/min |
| `/api/v1/sms/*` | 30 req/min |
| `/api/v1/*/accounts` | 60 req/min |
| `/api/v1/warming/*` | 60 req/min |
| `/api/v1/analytics/*` | 120 req/min |

При превышении лимита возвращается `429 Too Many Requests` с заголовками:
- `X-RateLimit-Limit`: лимит
- `X-RateLimit-Remaining`: оставшиеся запросы
- `X-RateLimit-Reset`: время сброса (Unix timestamp)

## Swagger UI

Интерактивная документация доступна по адресу:
- Development: http://localhost:8080/swagger/index.html
- Staging: https://api.staging.conveer.io/swagger/index.html

