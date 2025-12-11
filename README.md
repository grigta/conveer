# Conveer — Автоматизированный конвейер создания и прогрева аккаунтов

Conveer — это система автоматизированного создания и прогрева аккаунтов в социальных сетях (VK, Telegram, Mail.ru, Max) с управлением через Telegram бота. Построена на микросервисной архитектуре с использованием Go.

## 🏗 Архитектура

```mermaid
graph TD
    TGBot[Telegram Bot] --> APIGateway[API Gateway]
    APIGateway --> ProxyService[Proxy Service]
    APIGateway --> SMSService[SMS Service]
    APIGateway --> VKService[VK Service]
    APIGateway --> TelegramService[Telegram Service]
    APIGateway --> MailService[Mail Service]
    APIGateway --> MaxService[Max Service]
    APIGateway --> WarmingService[Warming Service]
    APIGateway --> AnalyticsService[Analytics Service]
    
    VKService --> ProxyService
    VKService --> SMSService
    TelegramService --> ProxyService
    TelegramService --> SMSService
    MailService --> ProxyService
    MailService --> SMSService
    MaxService --> VKService
    
    WarmingService --> VKService
    WarmingService --> TelegramService
    WarmingService --> MailService
    WarmingService --> MaxService
    
    AnalyticsService --> ProxyService
    AnalyticsService --> SMSService
    AnalyticsService --> VKService
    AnalyticsService --> TelegramService
    AnalyticsService --> WarmingService
    
    ProxyService --> MongoDB[(MongoDB)]
    SMSService --> MongoDB
    VKService --> MongoDB
    TelegramService --> MongoDB
    MailService --> MongoDB
    MaxService --> MongoDB
    WarmingService --> MongoDB
    AnalyticsService --> MongoDB
    TGBot --> MongoDB
    
    ProxyService --> Redis[(Redis)]
    SMSService --> Redis
    WarmingService --> Redis
    AnalyticsService --> Redis
    TGBot --> Redis
    
    ProxyService --> RabbitMQ[RabbitMQ]
    SMSService --> RabbitMQ
    VKService --> RabbitMQ
    TelegramService --> RabbitMQ
    MailService --> RabbitMQ
    MaxService --> RabbitMQ
    WarmingService --> RabbitMQ
    AnalyticsService --> RabbitMQ
    TGBot --> RabbitMQ
    
    ProxyService --> Prometheus[Prometheus]
    SMSService --> Prometheus
    VKService --> Prometheus
    TelegramService --> Prometheus
    WarmingService --> Prometheus
    AnalyticsService --> Prometheus
    
    Prometheus --> Grafana[Grafana]
    
    ProxyService --> ProxyProviders[Proxy Providers API]
    SMSService --> SMSActivate[SMS-Activate API]
```

### Микросервисы

| Сервис | Описание |
|--------|----------|
| **API Gateway** | Единая точка входа, маршрутизация, rate limiting |
| **Proxy Service** | Управление мобильными/резидентными прокси, ротация, health checks |
| **SMS Service** | Интеграция с SMS-Activate, получение кодов верификации |
| **VK Service** | Регистрация и управление аккаунтами VK |
| **Telegram Service** | Регистрация и управление аккаунтами Telegram |
| **Mail Service** | Регистрация и управление аккаунтами Mail.ru |
| **Max Service** | Регистрация и управление аккаунтами Max |
| **Warming Service** | Прогрев аккаунтов по сценариям (14-60 дней) |
| **Analytics Service** | Метрики, прогнозы, рекомендации |
| **Telegram Bot** | Управление системой через Telegram |

### Инфраструктура

- **MongoDB** — основная база данных
- **Redis** — кеширование и очереди
- **RabbitMQ** — асинхронная коммуникация между сервисами
- **Prometheus** — сбор метрик
- **Grafana** — визуализация метрик
- **Loki** — централизованное логирование

## 🚀 Быстрый старт

### Требования

- Go 1.21+
- Docker и Docker Compose
- Make

### Установка

1. Клонируйте репозиторий:
```bash
git clone https://github.com/conveer/conveer.git
cd conveer
```

2. Создайте файл окружения:
```bash
cp .env.example .env
```

3. Настройте переменные в `.env`:
```env
# Database
MONGODB_URI=mongodb://localhost:27017
REDIS_URL=redis://localhost:6379

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Encryption
ENCRYPTION_KEY=your-32-byte-encryption-key-here

# SMS-Activate
SMS_ACTIVATE_API_KEY=your-sms-activate-api-key

# Telegram Bot
TELEGRAM_BOT_TOKEN=your-bot-token
ADMIN_TELEGRAM_IDS=123456789,987654321

# Proxy Providers
PROXY_PROVIDERS_CONFIG=./config/providers.yaml
```

4. Запустите инфраструктуру:
```bash
docker-compose up -d mongodb redis rabbitmq
```

5. Запустите все сервисы:
```bash
make run
```

## 📝 Команды Makefile

```bash
# Разработка
make run              # Запуск всех сервисов
make stop             # Остановка всех сервисов
make build            # Сборка всех сервисов
make clean            # Очистка контейнеров и томов

# Тестирование
make test             # Запуск всех тестов
make test-unit        # Только unit-тесты
make test-integration # Только integration-тесты
make test-e2e         # End-to-end тесты
make test-coverage    # Тесты с отчетом покрытия

# Кодовая база
make lint             # Проверка линтером
make proto            # Генерация protobuf файлов
make mock-generate    # Генерация mock-объектов
make swagger-generate # Генерация Swagger документации

# Deployment
make docker-build-all # Сборка всех Docker образов
make docker-push-all  # Push образов в registry
```

## 🔑 Основные возможности

### Управление прокси

- Автоматическая покупка и ротация прокси
- Health checks с проверкой fraud score (IPQS)
- Поддержка множественных провайдеров через YAML конфигурацию
- Привязка прокси к аккаунтам с grace period при ротации

### SMS-верификация

- Интеграция с SMS-Activate API
- Retry-механизм с экспоненциальным backoff
- Автоматическое получение кодов
- Мониторинг баланса

### Прогрев аккаунтов

- Базовый сценарий (14 дней): лайки, просмотры, подписки
- Продвинутый сценарий (30 дней): + комментарии, сообщения
- Кастомные сценарии через YAML
- Симуляция человеческого поведения (задержки, паузы, burst patterns)

### Безопасность

- Шифрование AES-256 GCM для credentials
- Rate limiting
- JWT-аутентификация
- Whitelist для Telegram бота

## 📊 Мониторинг

| Интерфейс | URL | Credentials |
|-----------|-----|-------------|
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | admin/admin |
| RabbitMQ Management | http://localhost:15672 | guest/guest |

### Готовые дашборды Grafana

- **Proxy Overview** — статистика прокси, ротации, health checks
- **SMS Statistics** — активации, успешность, затраты
- **Warming Progress** — прогресс прогрева, действия, ошибки
- **System Health** — общее состояние системы

## 📂 Структура проекта

```
conveer/
├── .github/workflows/     # CI/CD конфигурации
├── config/                # Общие конфигурации
├── deploy/helm/           # Kubernetes Helm charts
├── docker/                # Docker конфигурации
│   ├── grafana/dashboards/
│   └── prometheus/
├── docs/                  # Документация
│   ├── api/              # API документация
│   └── ...
├── examples/configs/      # Примеры конфигураций
│   ├── dev/
│   ├── staging/
│   └── prod/
├── pkg/                   # Общие пакеты
│   ├── cache/
│   ├── config/
│   ├── crypto/
│   ├── database/
│   ├── logger/
│   ├── messaging/
│   ├── middleware/
│   └── testutil/
├── services/              # Микросервисы
│   ├── api-gateway/
│   ├── proxy-service/
│   ├── sms-service/
│   ├── vk-service/
│   ├── telegram-service/
│   ├── mail-service/
│   ├── max-service/
│   ├── warming-service/
│   ├── analytics-service/
│   └── telegram-bot/
├── tests/e2e/            # End-to-end тесты
├── docker-compose.yml
├── go.mod
├── go.work
├── Makefile
└── README.md
```

## 🧪 Тестирование

### Unit-тесты

```bash
make test-unit
```

Покрытие: >80% для критичных компонентов (crypto, proxy logic, warming scheduler)

### Integration-тесты

```bash
make test-integration
```

Используют testcontainers для MongoDB, Redis, RabbitMQ.

### End-to-end тесты

```bash
make test-e2e
```

Полный цикл: регистрация → прогрев → экспорт аккаунта.

### Отчет покрытия

```bash
make test-coverage
```

Генерирует HTML-отчет в `coverage.html`.

## 🔧 Конфигурация

### Основные переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `MONGODB_URI` | MongoDB connection string | `mongodb://localhost:27017` |
| `REDIS_URL` | Redis URL | `redis://localhost:6379` |
| `RABBITMQ_URL` | RabbitMQ URL | `amqp://guest:guest@localhost:5672/` |
| `ENCRYPTION_KEY` | 32-byte ключ шифрования | — (обязательно) |
| `SMS_ACTIVATE_API_KEY` | API ключ SMS-Activate | — (обязательно) |
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота | — (обязательно) |
| `ADMIN_TELEGRAM_IDS` | ID администраторов (через запятую) | — (обязательно) |

Подробная документация: [docs/configuration.md](docs/configuration.md)

## 📚 Документация

- [API документация](docs/api/README.md)
- [Руководство по конфигурации](docs/configuration.md)
- [Deployment инструкции](docs/deployment.md)
- [Руководство разработчика](docs/developer-guide.md)
- [Troubleshooting](docs/troubleshooting.md)
- [FAQ](docs/faq.md)

## 🚢 Deployment

### Docker Compose (dev/staging)

```bash
docker-compose up -d
```

### Kubernetes (production)

```bash
helm install conveer ./deploy/helm/conveer -f values-prod.yaml
```

Подробнее: [docs/deployment.md](docs/deployment.md)

## 📈 CI/CD

### GitHub Actions

- **CI** (`ci.yml`): тесты, линтер, security scan, сборка образов
- **CD** (`cd.yml`): деплой в staging (develop) / production (main)

### Pipeline

1. Push → lint + test
2. Build Docker images
3. Push to registry
4. Deploy to Kubernetes

## 📝 Лицензия

MIT

## 👥 Авторы

Conveer Team

## 🤝 Вклад

См. [CONTRIBUTING.md](CONTRIBUTING.md)
