# Troubleshooting Guide

## Содержание

- [Общие проблемы](#общие-проблемы)
- [Proxy Service](#proxy-service)
- [SMS Service](#sms-service)
- [Warming Service](#warming-service)
- [Platform Services (VK, Telegram)](#platform-services)
- [Infrastructure](#infrastructure)
- [Performance](#performance)

---

## Общие проблемы

### Сервис не запускается

**Симптомы:** Контейнер перезапускается, логи показывают ошибки подключения.

**Проверить:**

```bash
# Статус контейнеров
docker-compose ps

# Логи проблемного сервиса
docker-compose logs proxy-service

# Проверить подключение к MongoDB
docker exec conveer-mongodb mongosh --eval "db.adminCommand('ping')"

# Проверить подключение к Redis
docker exec conveer-redis redis-cli ping

# Проверить RabbitMQ
curl -u guest:guest http://localhost:15672/api/overview
```

**Решения:**

1. Убедитесь, что все переменные окружения заданы в `.env`
2. Проверьте, что инфраструктурные сервисы запущены: `make dev`
3. Проверьте сетевое подключение между контейнерами

### Ошибки аутентификации

**Симптомы:** `401 Unauthorized`, `Invalid token`

**Проверить:**

```bash
# Проверить токен
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/health
```

**Решения:**

1. Убедитесь, что токен не истёк
2. Проверьте, что `JWT_SECRET` одинаковый на всех сервисах
3. Обновите токен через Telegram бота

### Медленные запросы

**Симптомы:** Запросы выполняются дольше 5 секунд.

**Диагностика:**

```bash
# Проверить метрики latency
curl http://localhost:8080/metrics | grep http_request_duration

# Проверить MongoDB slow queries
docker exec conveer-mongodb mongosh --eval "db.setProfilingLevel(1, { slowms: 100 })"
```

**Решения:**

1. Добавить индексы в MongoDB
2. Увеличить размер пула соединений
3. Включить кэширование в Redis

---

## Proxy Service

### "No available proxies"

**Причина:** Пул прокси исчерпан.

**Диагностика:**

```bash
# Проверить количество доступных прокси
curl http://localhost:8081/api/v1/proxies?status=available

# Проверить метрики
curl http://localhost:8081/metrics | grep proxies_available
```

**Решения:**

1. Обновить пул: `POST /api/v1/proxies/pool/refresh`
2. Проверить баланс у провайдеров прокси
3. Увеличить `PROXY_POOL_MIN_SIZE` в конфигурации
4. Проверить, не забанены ли все прокси

### Прокси помечается как banned

**Причина:** Высокий fraud score или недоступность.

**Диагностика:**

```bash
# Проверить здоровье прокси
curl http://localhost:8081/api/v1/proxies/{id}/health

# Посмотреть логи health checker
docker-compose logs proxy-service | grep "health check"
```

**Решения:**

1. Проверить настройки IPQS API (ключ, лимиты)
2. Уменьшить порог `PROXY_MAX_FRAUD_SCORE`
3. Сменить провайдера прокси
4. Проверить, не заблокирован ли IP провайдера целевой платформой

### Ротация прокси не работает

**Причина:** Проблемы с RabbitMQ delayed messages.

**Диагностика:**

```bash
# Проверить очередь ротации
curl -u guest:guest http://localhost:15672/api/queues/%2F/proxy.rotation

# Проверить delayed exchange
curl -u guest:guest http://localhost:15672/api/exchanges/%2F/conveer.delayed
```

**Решения:**

1. Убедитесь, что установлен плагин `rabbitmq_delayed_message_exchange`
2. Проверьте TTL сообщений
3. Перезапустите proxy-service

---

## SMS Service

### "No numbers available"

**Причина:** У SMS-провайдера закончились номера.

**Диагностика:**

```bash
# Проверить баланс
curl http://localhost:8082/api/v1/sms/balance

# Проверить логи
docker-compose logs sms-service | grep "get number"
```

**Решения:**

1. Пополнить баланс у провайдера
2. Сменить страну/оператора
3. Попробовать альтернативного провайдера
4. Увеличить интервал между запросами

### SMS код не приходит

**Причина:** Таймаут ожидания, неверный номер.

**Диагностика:**

```bash
# Проверить статус активации
curl http://localhost:8082/api/v1/sms/activations/{id}

# Логи retry
docker-compose logs sms-service | grep "retry"
```

**Решения:**

1. Увеличить таймаут `SMS_CODE_TIMEOUT`
2. Проверить, поддерживает ли провайдер данный сервис
3. Отменить активацию и запросить новый номер

### Высокий расход SMS

**Причина:** Много отмен, баны аккаунтов на этапе регистрации.

**Решения:**

1. Проверить качество прокси
2. Добавить задержки между регистрациями
3. Сменить провайдера SMS

---

## Warming Service

### Задача прогрева зависла в статусе "running"

**Причина:** Воркер упал, сообщение потеряно.

**Диагностика:**

```bash
# Проверить активные задачи
curl "http://localhost:8083/api/v1/warming/tasks?status=running"

# Проверить consumers в RabbitMQ
curl -u guest:guest http://localhost:15672/api/consumers
```

**Решения:**

1. Перезапустить warming-service
2. Вручную обновить статус задачи в MongoDB
3. Проверить prefetch count и acknowledgment

### "Account banned during warming"

**Причина:** Слишком агрессивный сценарий прогрева.

**Решения:**

1. Использовать более консервативный сценарий (`basic` вместо `advanced`)
2. Увеличить задержки между действиями
3. Уменьшить количество действий в день
4. Добавить ночные перерывы
5. Разнообразить типы действий

### Действия выполняются слишком медленно

**Причина:** Высокая задержка прокси, медленный Playwright.

**Диагностика:**

```bash
# Метрики времени выполнения
curl http://localhost:8083/metrics | grep warming_action_duration
```

**Решения:**

1. Использовать прокси с меньшей latency
2. Увеличить количество параллельных воркеров
3. Оптимизировать Playwright скрипты

---

## Platform Services

### VK: "Captcha required"

**Причина:** VK требует капчу при подозрительной активности.

**Решения:**

1. Уменьшить частоту действий
2. Сменить прокси
3. Интегрировать сервис распознавания капчи (Anti-Captcha, RuCaptcha)
4. Добавить больше "человечности" в поведение

### VK: "Rate limit exceeded"

**Причина:** Превышен лимит API VK.

**Решения:**

1. Добавить задержки между запросами
2. Использовать несколько токенов
3. Переключиться на browser automation вместо API

### Telegram: "Phone number banned"

**Причина:** Номер заблокирован Telegram.

**Решения:**

1. Использовать другого SMS провайдера
2. Использовать номера других стран
3. Увеличить интервал между регистрациями

### Telegram: "FloodWait"

**Причина:** Слишком много запросов к Telegram API.

**Решения:**

```python
# В клиенте Telethon/Pyrogram автоматически обрабатывается
# Убедитесь, что обработка включена
```

1. Соблюдать sleep time из ошибки
2. Уменьшить параллелизм
3. Использовать разные аккаунты для разных операций

---

## Infrastructure

### MongoDB: "Connection refused"

**Диагностика:**

```bash
docker-compose logs mongodb
docker exec conveer-mongodb mongosh --eval "db.serverStatus()"
```

**Решения:**

1. Проверить, что контейнер запущен
2. Проверить credentials в `.env`
3. Проверить disk space на хосте

### MongoDB: Высокое использование памяти

**Диагностика:**

```bash
docker stats conveer-mongodb
docker exec conveer-mongodb mongosh --eval "db.serverStatus().mem"
```

**Решения:**

1. Настроить `wiredTigerCacheSizeGB`
2. Добавить индексы для частых запросов
3. Архивировать старые данные

### Redis: "OOM command not allowed"

**Причина:** Redis исчерпал память.

**Решения:**

1. Увеличить `maxmemory` в redis.conf
2. Настроить политику вытеснения: `maxmemory-policy allkeys-lru`
3. Уменьшить TTL кэшированных данных
4. Проверить утечки ключей

### RabbitMQ: Messages piling up

**Диагностика:**

```bash
# Проверить очереди
curl -u guest:guest http://localhost:15672/api/queues

# Проверить consumers
curl -u guest:guest http://localhost:15672/api/consumers
```

**Решения:**

1. Увеличить количество consumers
2. Проверить, что consumers работают (нет deadlock)
3. Увеличить prefetch count
4. Масштабировать сервис-consumer

### RabbitMQ: Delayed messages not working

**Проверить:**

```bash
# Проверить установлен ли плагин
docker exec conveer-rabbitmq rabbitmq-plugins list | grep delayed
```

**Решения:**

1. Включить плагин: `rabbitmq-plugins enable rabbitmq_delayed_message_exchange`
2. Перезапустить RabbitMQ

---

## Performance

### Высокая latency API

**Диагностика:**

```bash
# p99 latency
curl http://localhost:8080/metrics | grep "http_request_duration.*quantile=\"0.99\""

# Трейсинг (если включен)
# Jaeger UI: http://localhost:16686
```

**Решения:**

1. Включить кэширование Redis для частых запросов
2. Оптимизировать MongoDB запросы
3. Добавить индексы
4. Увеличить connection pool size

### Высокое потребление CPU

**Диагностика:**

```bash
docker stats
# или в Kubernetes
kubectl top pods -n conveer
```

**Решения:**

1. Профилирование с pprof: `go tool pprof http://localhost:8081/debug/pprof/profile`
2. Оптимизировать горячие пути
3. Увеличить ресурсы или масштабировать горизонтально

### Memory leaks

**Диагностика:**

```bash
# Heap profile
curl http://localhost:8081/debug/pprof/heap > heap.out
go tool pprof heap.out

# Goroutine leaks
curl http://localhost:8081/debug/pprof/goroutine?debug=2
```

**Решения:**

1. Проверить утечки goroutine (незакрытые каналы)
2. Проверить незакрытые HTTP/gRPC соединения
3. Проверить кэши без ограничения размера

---

## Команды для диагностики

```bash
# Общий статус системы
make health

# Логи всех сервисов
docker-compose logs -f

# Метрики
curl http://localhost:9090/api/v1/query?query=up

# MongoDB статус
docker exec conveer-mongodb mongosh --eval "db.adminCommand({serverStatus:1})"

# Redis info
docker exec conveer-redis redis-cli INFO

# RabbitMQ overview
curl -u guest:guest http://localhost:15672/api/overview
```

