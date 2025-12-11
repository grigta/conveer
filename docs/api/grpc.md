# gRPC API Documentation

## Обзор

Conveer использует gRPC для внутренней коммуникации между сервисами. Это обеспечивает высокую производительность и строгую типизацию.

## Сервисы

### AccountService

Сервис управления аккаунтами. Используется для получения учётных данных и обновления статуса.

```protobuf
service AccountService {
  // Получить учётные данные аккаунта
  rpc GetCredentials(GetCredentialsRequest) returns (GetCredentialsResponse);
  
  // Обновить статус аккаунта
  rpc UpdateStatus(UpdateStatusRequest) returns (UpdateStatusResponse);
  
  // Получить информацию об аккаунте
  rpc GetAccount(GetAccountRequest) returns (Account);
  
  // Поиск аккаунтов по фильтру
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
}
```

#### GetCredentials

Получает расшифрованные учётные данные аккаунта.

**Request:**
```protobuf
message GetCredentialsRequest {
  string account_id = 1;
}
```

**Response:**
```protobuf
message GetCredentialsResponse {
  string login = 1;
  string password = 2;
  string phone = 3;
  map<string, string> additional = 4; // Дополнительные данные (токены и т.д.)
}
```

**Пример использования (Go):**
```go
client := pb.NewAccountServiceClient(conn)
resp, err := client.GetCredentials(ctx, &pb.GetCredentialsRequest{
    AccountId: "507f1f77bcf86cd799439011",
})
if err != nil {
    log.Fatalf("Failed to get credentials: %v", err)
}
fmt.Printf("Login: %s, Password: %s\n", resp.Login, resp.Password)
```

#### UpdateStatus

Обновляет статус аккаунта.

**Request:**
```protobuf
message UpdateStatusRequest {
  string account_id = 1;
  AccountStatus status = 2;
  string reason = 3; // Причина изменения статуса
}

enum AccountStatus {
  ACCOUNT_STATUS_UNSPECIFIED = 0;
  ACCOUNT_STATUS_CREATING = 1;
  ACCOUNT_STATUS_ACTIVE = 2;
  ACCOUNT_STATUS_WARMING = 3;
  ACCOUNT_STATUS_BANNED = 4;
  ACCOUNT_STATUS_SUSPENDED = 5;
  ACCOUNT_STATUS_DELETED = 6;
}
```

**Response:**
```protobuf
message UpdateStatusResponse {
  bool success = 1;
  string previous_status = 2;
}
```

---

### ProxyService

Сервис управления прокси.

```protobuf
service ProxyService {
  // Аллоцировать прокси для аккаунта
  rpc AllocateProxy(AllocateProxyRequest) returns (Proxy);
  
  // Освободить прокси
  rpc ReleaseProxy(ReleaseProxyRequest) returns (ReleaseProxyResponse);
  
  // Получить прокси для аккаунта
  rpc GetProxyForAccount(GetProxyForAccountRequest) returns (Proxy);
  
  // Проверить здоровье прокси
  rpc CheckHealth(CheckHealthRequest) returns (ProxyHealth);
  
  // Принудительная ротация
  rpc ForceRotate(ForceRotateRequest) returns (Proxy);
}
```

#### AllocateProxy

Выделяет прокси для указанного аккаунта.

**Request:**
```protobuf
message AllocateProxyRequest {
  string account_id = 1;
  string platform = 2;
  string preferred_country = 3; // Опционально
  string preferred_provider = 4; // Опционально
}
```

**Response:**
```protobuf
message Proxy {
  string id = 1;
  string host = 2;
  int32 port = 3;
  string protocol = 4; // http, https, socks5
  string username = 5;
  string password = 6;
  string country = 7;
  string provider = 8;
  google.protobuf.Timestamp expires_at = 9;
}
```

**Пример использования (Go):**
```go
client := pb.NewProxyServiceClient(conn)
proxy, err := client.AllocateProxy(ctx, &pb.AllocateProxyRequest{
    AccountId: "507f1f77bcf86cd799439011",
    Platform:  "vk",
    PreferredCountry: "RU",
})
if err != nil {
    log.Fatalf("Failed to allocate proxy: %v", err)
}
proxyURL := fmt.Sprintf("%s://%s:%s@%s:%d",
    proxy.Protocol, proxy.Username, proxy.Password, proxy.Host, proxy.Port)
```

---

### WarmingService

Сервис прогрева аккаунтов.

```protobuf
service WarmingService {
  // Запустить прогрев
  rpc StartWarming(StartWarmingRequest) returns (WarmingTask);
  
  // Приостановить прогрев
  rpc PauseWarming(PauseWarmingRequest) returns (WarmingTask);
  
  // Возобновить прогрев
  rpc ResumeWarming(ResumeWarmingRequest) returns (WarmingTask);
  
  // Остановить прогрев
  rpc StopWarming(StopWarmingRequest) returns (WarmingTask);
  
  // Получить статус прогрева
  rpc GetWarmingStatus(GetWarmingStatusRequest) returns (WarmingTask);
  
  // Получить статистику
  rpc GetStatistics(GetStatisticsRequest) returns (WarmingStatistics);
  
  // Streaming: Подписка на события прогрева
  rpc StreamEvents(StreamEventsRequest) returns (stream WarmingEvent);
}
```

#### StartWarming

Запускает задачу прогрева для аккаунта.

**Request:**
```protobuf
message StartWarmingRequest {
  string account_id = 1;
  string scenario_id = 2; // Опционально, по умолчанию basic
  int32 duration_days = 3; // Опционально, переопределяет сценарий
}
```

**Response:**
```protobuf
message WarmingTask {
  string id = 1;
  string account_id = 2;
  string scenario_id = 3;
  WarmingStatus status = 4;
  int32 progress = 5; // 0-100
  int32 current_day = 6;
  int32 total_days = 7;
  int32 actions_completed = 8;
  int32 actions_failed = 9;
  google.protobuf.Timestamp next_action_at = 10;
  google.protobuf.Timestamp started_at = 11;
  google.protobuf.Timestamp completed_at = 12;
  string error = 13;
}

enum WarmingStatus {
  WARMING_STATUS_UNSPECIFIED = 0;
  WARMING_STATUS_PENDING = 1;
  WARMING_STATUS_RUNNING = 2;
  WARMING_STATUS_PAUSED = 3;
  WARMING_STATUS_COMPLETED = 4;
  WARMING_STATUS_FAILED = 5;
}
```

#### StreamEvents

Стриминг событий прогрева в реальном времени.

**Request:**
```protobuf
message StreamEventsRequest {
  string task_id = 1; // Опционально, если не указан - все события
  repeated string event_types = 2; // Фильтр по типам событий
}
```

**Response (stream):**
```protobuf
message WarmingEvent {
  string task_id = 1;
  string event_type = 2; // action_started, action_completed, action_failed, status_changed
  google.protobuf.Timestamp timestamp = 3;
  map<string, string> data = 4;
}
```

**Пример использования (Go):**
```go
client := pb.NewWarmingServiceClient(conn)
stream, err := client.StreamEvents(ctx, &pb.StreamEventsRequest{
    TaskId: "507f1f77bcf86cd799439011",
})
if err != nil {
    log.Fatalf("Failed to stream events: %v", err)
}

for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatalf("Error receiving event: %v", err)
    }
    fmt.Printf("Event: %s at %v\n", event.EventType, event.Timestamp)
}
```

---

### SMSService

Сервис SMS-активации.

```protobuf
service SMSService {
  // Получить номер для активации
  rpc GetNumber(GetNumberRequest) returns (SMSActivation);
  
  // Получить SMS-код
  rpc GetCode(GetCodeRequest) returns (GetCodeResponse);
  
  // Отменить активацию
  rpc CancelActivation(CancelActivationRequest) returns (CancelActivationResponse);
  
  // Получить баланс
  rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
}
```

#### GetNumber

Получает виртуальный номер для SMS-активации.

**Request:**
```protobuf
message GetNumberRequest {
  string service = 1; // vk, telegram, mail, max
  string country = 2; // По умолчанию "ru"
  string operator = 3; // Опционально
}
```

**Response:**
```protobuf
message SMSActivation {
  string id = 1;
  string phone = 2;
  string service = 3;
  string country = 4;
  SMSStatus status = 5;
  google.protobuf.Timestamp expires_at = 6;
}

enum SMSStatus {
  SMS_STATUS_UNSPECIFIED = 0;
  SMS_STATUS_PENDING = 1;
  SMS_STATUS_WAITING_CODE = 2;
  SMS_STATUS_CODE_RECEIVED = 3;
  SMS_STATUS_COMPLETED = 4;
  SMS_STATUS_CANCELLED = 5;
  SMS_STATUS_FAILED = 6;
}
```

#### GetCode

Получает SMS-код для активации. Метод блокирующий с таймаутом.

**Request:**
```protobuf
message GetCodeRequest {
  string activation_id = 1;
  int32 timeout_seconds = 2; // По умолчанию 120
}
```

**Response:**
```protobuf
message GetCodeResponse {
  string code = 1;
  bool success = 2;
  string error = 3;
}
```

---

### VKService

Сервис автоматизации ВКонтакте.

```protobuf
service VKService {
  // Регистрация нового аккаунта
  rpc Register(RegisterRequest) returns (RegisterResponse);
  
  // Выполнить действие
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteActionResponse);
  
  // Проверить статус аккаунта
  rpc CheckAccountStatus(CheckAccountStatusRequest) returns (CheckAccountStatusResponse);
}
```

#### ExecuteAction

Выполняет действие от имени аккаунта (для прогрева).

**Request:**
```protobuf
message ExecuteActionRequest {
  string account_id = 1;
  ActionType action_type = 2;
  map<string, string> params = 3;
}

enum ActionType {
  ACTION_TYPE_UNSPECIFIED = 0;
  ACTION_TYPE_VIEW_PROFILE = 1;
  ACTION_TYPE_LIKE_POST = 2;
  ACTION_TYPE_SUBSCRIBE = 3;
  ACTION_TYPE_COMMENT = 4;
  ACTION_TYPE_SEND_MESSAGE = 5;
  ACTION_TYPE_REPOST = 6;
  ACTION_TYPE_VIEW_STORIES = 7;
  ACTION_TYPE_ADD_FRIEND = 8;
}
```

**Response:**
```protobuf
message ExecuteActionResponse {
  bool success = 1;
  string error = 2;
  ActionResult result = 3;
}

message ActionResult {
  string action_type = 1;
  int64 duration_ms = 2;
  map<string, string> data = 3;
}
```

---

## Коды ошибок

gRPC использует стандартные коды статуса. Дополнительная информация передаётся в metadata.

| Код | Описание | Когда используется |
|-----|----------|-------------------|
| `OK` | Успех | Операция выполнена успешно |
| `INVALID_ARGUMENT` | Неверные аргументы | Некорректные входные данные |
| `NOT_FOUND` | Не найдено | Ресурс не существует |
| `ALREADY_EXISTS` | Уже существует | Попытка создать дубликат |
| `PERMISSION_DENIED` | Доступ запрещён | Нет прав на операцию |
| `RESOURCE_EXHAUSTED` | Ресурсы исчерпаны | Rate limit, нет свободных прокси |
| `UNAVAILABLE` | Недоступен | Сервис временно недоступен |
| `INTERNAL` | Внутренняя ошибка | Ошибка на стороне сервера |

### Обработка ошибок

```go
resp, err := client.AllocateProxy(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.NotFound:
            log.Printf("Account not found")
        case codes.ResourceExhausted:
            log.Printf("No available proxies")
        case codes.Unavailable:
            log.Printf("Service unavailable, retry later")
        default:
            log.Printf("RPC error: %v", st.Message())
        }
    }
    return err
}
```

---

## Подключение

### Создание клиента

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "conveer/proto"
)

func NewClient(address string) (*grpc.ClientConn, pb.ProxyServiceClient, error) {
    conn, err := grpc.Dial(address,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
        ),
    )
    if err != nil {
        return nil, nil, err
    }
    
    client := pb.NewProxyServiceClient(conn)
    return conn, client, nil
}
```

### Конфигурация сервисов

| Сервис | Порт | Переменная окружения |
|--------|------|---------------------|
| AccountService | 50051 | `ACCOUNT_SERVICE_GRPC_ADDR` |
| ProxyService | 50052 | `PROXY_SERVICE_GRPC_ADDR` |
| WarmingService | 50053 | `WARMING_SERVICE_GRPC_ADDR` |
| SMSService | 50054 | `SMS_SERVICE_GRPC_ADDR` |
| VKService | 50055 | `VK_SERVICE_GRPC_ADDR` |
| TelegramService | 50056 | `TELEGRAM_SERVICE_GRPC_ADDR` |
| MailService | 50057 | `MAIL_SERVICE_GRPC_ADDR` |
| MaxService | 50058 | `MAX_SERVICE_GRPC_ADDR` |

---

## Interceptors

### Logging Interceptor

```go
func LoggingInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    start := time.Now()
    resp, err := handler(ctx, req)
    duration := time.Since(start)
    
    log.Printf("method=%s duration=%v error=%v",
        info.FullMethod, duration, err)
    
    return resp, err
}
```

### Recovery Interceptor

```go
func RecoveryInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = status.Errorf(codes.Internal, "panic: %v", r)
        }
    }()
    return handler(ctx, req)
}
```

---

## Health Checks

Все сервисы реализуют стандартный gRPC Health Checking Protocol.

```protobuf
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}
```

**Пример:**
```go
import "google.golang.org/grpc/health/grpc_health_v1"

healthClient := grpc_health_v1.NewHealthClient(conn)
resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
    Service: "conveer.ProxyService",
})
if err != nil || resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
    log.Printf("Service unhealthy: %v", resp.Status)
}
```

