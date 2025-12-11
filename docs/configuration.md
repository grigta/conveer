# Руководство по конфигурации

## Переменные окружения

### Application

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `APP_ENV` | Окружение (dev/staging/prod) | string | `dev` | Нет |
| `APP_DEBUG` | Режим отладки | bool | `false` | Нет |
| `LOG_LEVEL` | Уровень логирования | string | `info` | Нет |
| `LOG_FORMAT` | Формат логов (json/text) | string | `json` | Нет |

### Database (MongoDB)

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `MONGODB_URI` | MongoDB connection string | string | — | Да |
| `MONGODB_DATABASE` | Имя базы данных | string | `conveer` | Нет |
| `MONGODB_MAX_POOL_SIZE` | Максимальный размер пула | int | `100` | Нет |
| `MONGODB_MIN_POOL_SIZE` | Минимальный размер пула | int | `10` | Нет |

### Redis

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `REDIS_URL` | Redis connection URL | string | — | Да |
| `REDIS_PASSWORD` | Пароль Redis | string | — | Нет |
| `REDIS_DB` | Номер базы данных | int | `0` | Нет |
| `REDIS_POOL_SIZE` | Размер пула соединений | int | `10` | Нет |

### RabbitMQ

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `RABBITMQ_URL` | AMQP connection URL | string | — | Да |
| `RABBITMQ_PREFETCH_COUNT` | Prefetch count | int | `10` | Нет |

### Шифрование

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `ENCRYPTION_KEY` | 32-байтный ключ AES-256 | string | — | Да |

### JWT

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `JWT_SECRET` | Секретный ключ JWT | string | — | Да |
| `JWT_ACCESS_TTL` | Время жизни access token | duration | `1h` | Нет |
| `JWT_REFRESH_TTL` | Время жизни refresh token | duration | `168h` | Нет |

### Proxy Service

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `PROXY_PROVIDERS_CONFIG` | Путь к конфигурации провайдеров | string | `./config/providers.yaml` | Нет |
| `PROXY_HEALTH_CHECK_INTERVAL` | Интервал проверки прокси | duration | `15m` | Нет |
| `PROXY_MAX_FAILED_CHECKS` | Максимум неудачных проверок | int | `3` | Нет |
| `PROXY_ROTATION_CHECK_INTERVAL` | Интервал проверки ротации | duration | `5m` | Нет |
| `IPQS_API_KEY` | API ключ IPQualityScore | string | — | Нет |

### SMS Service

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `SMS_ACTIVATE_API_KEY` | API ключ SMS-Activate | string | — | Да |
| `SMS_ACTIVATE_BASE_URL` | Базовый URL API | string | `https://sms-activate.org/stubs/handler_api.php` | Нет |
| `SMS_MAX_RETRIES` | Максимум повторных попыток | int | `4` | Нет |
| `SMS_RETRY_DELAY` | Базовая задержка retry | duration | `1m` | Нет |
| `SMS_CODE_TIMEOUT` | Таймаут ожидания кода | duration | `15m` | Нет |

### Warming Service

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `WARMING_CONFIG_PATH` | Путь к конфигурации прогрева | string | `./config/warming_config.yaml` | Нет |
| `WARMING_SCHEDULER_INTERVAL` | Интервал планировщика | duration | `1m` | Нет |
| `WARMING_MAX_CONCURRENT_TASKS` | Максимум параллельных задач | int | `50` | Нет |
| `WARMING_ENABLE_AUTO_START` | Автозапуск прогрева | bool | `true` | Нет |

### Telegram Bot

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота | string | — | Да |
| `ADMIN_TELEGRAM_IDS` | ID администраторов (через запятую) | string | — | Да |
| `TELEGRAM_WEBHOOK_URL` | URL для webhook | string | — | Нет |

### Мониторинг

| Переменная | Описание | Тип | По умолчанию | Обязательно |
|------------|----------|-----|--------------|-------------|
| `PROMETHEUS_PORT` | Порт для метрик | int | `9090` | Нет |
| `METRICS_PATH` | Путь к метрикам | string | `/metrics` | Нет |

## YAML конфигурации

### Провайдеры прокси (`config/providers.yaml`)

```yaml
providers:
  - name: "proxyseller"
    enabled: true
    priority: 1
    api:
      base_url: "https://api.proxyseller.com/v1"
      auth_type: "bearer"  # bearer, basic, api_key
      auth_key: "${PROXYSELLER_API_KEY}"
    endpoints:
      list: "/proxies"
      purchase: "/proxies/purchase"
      release: "/proxies/{id}/release"
      rotate: "/proxies/{id}/rotate"
      check: "/proxies/{id}/status"
    limits:
      max_proxies: 100
      requests_per_minute: 60
    supported_types:
      - mobile
      - residential
    supported_countries:
      - RU
      - US
      - DE

  - name: "proxyline"
    enabled: true
    priority: 2
    api:
      base_url: "https://panel.proxyline.net/api"
      auth_type: "api_key"
      auth_key: "${PROXYLINE_API_KEY}"
    endpoints:
      list: "/orders"
      purchase: "/orders/new"
      release: "/orders/{id}/cancel"
      rotate: "/orders/{id}/changeip"
      check: "/orders/{id}"
    limits:
      max_proxies: 50
      requests_per_minute: 30
    supported_types:
      - mobile
    supported_countries:
      - RU
```

### Конфигурация прогрева (`config/warming_config.yaml`)

```yaml
scenarios:
  basic:
    vk:
      duration_14_30:
        days_1_7:
          actions_per_day: "5-10"
          actions:
            - type: view_feed
              weight: 40
              params:
                max_per_day: 20
            - type: like_post
              weight: 35
              params:
                max_per_day: 10
            - type: subscribe_group
              weight: 25
              params:
                max_per_day: 5
        days_8_14:
          actions_per_day: "10-15"
          actions:
            - type: view_feed
              weight: 30
            - type: like_post
              weight: 35
            - type: subscribe_group
              weight: 20
            - type: view_profile
              weight: 15
        days_15_30:
          actions_per_day: "15-25"
          actions:
            - type: view_feed
              weight: 25
            - type: like_post
              weight: 30
            - type: subscribe_group
              weight: 15
            - type: comment_post
              weight: 15
              params:
                max_per_day: 5
            - type: view_profile
              weight: 15

  advanced:
    vk:
      duration_30_60:
        days_1_7:
          actions_per_day: "5-10"
          # ... similar structure
        days_8_14:
          actions_per_day: "10-20"
          # ...
        days_15_30:
          actions_per_day: "20-35"
          actions:
            - type: view_feed
              weight: 20
            - type: like_post
              weight: 25
            - type: subscribe_group
              weight: 15
            - type: comment_post
              weight: 15
            - type: send_message
              weight: 10
              params:
                max_per_day: 3
            - type: view_profile
              weight: 15
        days_31_60:
          actions_per_day: "25-40"
          # ... full activity

behavior_simulation:
  enable_random_delays: true
  delay_min_seconds: 30
  delay_max_seconds: 300
  active_hours_start: 8
  active_hours_end: 22
  night_pause_probability: 0.9
  weekend_activity_reduction: 0.7
  enable_burst_patterns: true
  burst_probability: 0.15

scheduler:
  check_interval: 1m
  max_concurrent_tasks: 50
  stuck_task_timeout: 2h
```

### Конфигурация Telegram бота (`config/bot_config.yaml`)

```yaml
bot:
  whitelist_enabled: true
  default_language: "ru"

roles:
  admin:
    telegram_ids:
      - 123456789
      - 987654321
    permissions:
      - register_accounts
      - view_statistics
      - manage_warming
      - export_accounts
      - manage_users
      - view_logs
      
  operator:
    telegram_ids:
      - 111111111
    permissions:
      - register_accounts
      - view_statistics
      - manage_warming
      - export_accounts

notifications:
  alerts:
    enabled: true
    channels:
      - type: telegram
        chat_ids:
          - -1001234567890  # Group chat
    conditions:
      - type: ban_rate
        threshold: 15
        period: 1h
      - type: error_rate
        threshold: 20
        period: 1h
      - type: low_balance
        threshold: 500
        
export:
  formats:
    telegram:
      - tdata
      - session_telethon
      - session_pyrogram
      - json
    vk:
      - json
      - csv
    mail:
      - json
      - csv
```

## Примеры конфигураций

### Development (`.env.dev`)

```env
APP_ENV=dev
APP_DEBUG=true
LOG_LEVEL=debug
LOG_FORMAT=text

MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=conveer_dev

REDIS_URL=redis://localhost:6379
REDIS_DB=0

RABBITMQ_URL=amqp://guest:guest@localhost:5672/

ENCRYPTION_KEY=dev-key-32-bytes-here-123456789
JWT_SECRET=dev-jwt-secret-key

# Use mock providers in dev
PROXY_PROVIDERS_CONFIG=./config/providers.dev.yaml
SMS_ACTIVATE_API_KEY=test-api-key

TELEGRAM_BOT_TOKEN=your-dev-bot-token
ADMIN_TELEGRAM_IDS=123456789

# Disable headless for debugging
PLAYWRIGHT_HEADLESS=false
```

### Staging (`.env.staging`)

```env
APP_ENV=staging
APP_DEBUG=false
LOG_LEVEL=info
LOG_FORMAT=json

MONGODB_URI=mongodb://staging-mongo:27017
MONGODB_DATABASE=conveer_staging
MONGODB_MAX_POOL_SIZE=50

REDIS_URL=redis://staging-redis:6379
REDIS_PASSWORD=staging-password

RABBITMQ_URL=amqp://user:password@staging-rabbitmq:5672/

ENCRYPTION_KEY=${STAGING_ENCRYPTION_KEY}
JWT_SECRET=${STAGING_JWT_SECRET}

PROXY_PROVIDERS_CONFIG=./config/providers.yaml
SMS_ACTIVATE_API_KEY=${STAGING_SMS_API_KEY}

TELEGRAM_BOT_TOKEN=${STAGING_BOT_TOKEN}
ADMIN_TELEGRAM_IDS=123456789,987654321

WARMING_MAX_CONCURRENT_TASKS=10
PLAYWRIGHT_HEADLESS=true
```

### Production (`.env.prod`)

```env
APP_ENV=prod
APP_DEBUG=false
LOG_LEVEL=warn
LOG_FORMAT=json

MONGODB_URI=${PROD_MONGODB_URI}
MONGODB_DATABASE=conveer
MONGODB_MAX_POOL_SIZE=100

REDIS_URL=${PROD_REDIS_URL}
REDIS_PASSWORD=${PROD_REDIS_PASSWORD}

RABBITMQ_URL=${PROD_RABBITMQ_URL}

ENCRYPTION_KEY=${PROD_ENCRYPTION_KEY}
JWT_SECRET=${PROD_JWT_SECRET}

PROXY_PROVIDERS_CONFIG=/etc/conveer/providers.yaml
IPQS_API_KEY=${PROD_IPQS_API_KEY}

SMS_ACTIVATE_API_KEY=${PROD_SMS_API_KEY}

TELEGRAM_BOT_TOKEN=${PROD_BOT_TOKEN}
ADMIN_TELEGRAM_IDS=${PROD_ADMIN_IDS}

WARMING_MAX_CONCURRENT_TASKS=50
PLAYWRIGHT_HEADLESS=true

# Monitoring
PROMETHEUS_PORT=9090
```

## Секреты

### Рекомендации по хранению

1. **Локальная разработка**: `.env` файл (добавлен в `.gitignore`)

2. **Staging/Production**: 
   - HashiCorp Vault
   - Kubernetes Secrets
   - AWS Secrets Manager
   - Google Secret Manager

### Пример использования Vault

```bash
# Сохранение секрета
vault kv put secret/conveer/prod \
  encryption_key="your-32-byte-key" \
  jwt_secret="your-jwt-secret" \
  mongodb_uri="mongodb+srv://..."

# Чтение в приложении
ENCRYPTION_KEY=$(vault kv get -field=encryption_key secret/conveer/prod)
```

### Ротация ключей

Для ротации ключа шифрования:

1. Сгенерируйте новый ключ:
```bash
openssl rand -hex 32
```

2. Добавьте новый ключ параллельно со старым:
```env
ENCRYPTION_KEY=new-key
ENCRYPTION_KEY_OLD=old-key
```

3. Запустите миграцию данных:
```bash
make migrate-encryption
```

4. Удалите старый ключ после миграции.

