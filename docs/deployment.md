# Deployment Guide

## Содержание

- [Требования](#требования)
- [Docker Compose (Development)](#docker-compose-development)
- [Docker Compose (Production)](#docker-compose-production)
- [Kubernetes (Helm)](#kubernetes-helm)
- [CI/CD Pipeline](#cicd-pipeline)
- [Мониторинг](#мониторинг)
- [Масштабирование](#масштабирование)

---

## Требования

### Минимальные системные требования

| Окружение | CPU | RAM | Disk |
|-----------|-----|-----|------|
| Development | 2 cores | 4 GB | 20 GB |
| Staging | 4 cores | 8 GB | 50 GB |
| Production | 8+ cores | 16+ GB | 100+ GB SSD |

### Зависимости

- Docker 20.10+
- Docker Compose 2.0+
- Kubernetes 1.24+ (для k8s)
- Helm 3.0+ (для k8s)
- Go 1.21+ (для локальной разработки)

---

## Docker Compose (Development)

### Быстрый старт

```bash
# Клонировать репозиторий
git clone https://github.com/your-org/conveer.git
cd conveer

# Скопировать пример конфигурации
cp .env.example .env

# Отредактировать .env с вашими значениями
nano .env

# Запустить все сервисы
docker-compose up -d

# Проверить статус
docker-compose ps
```

### Структура docker-compose.yml

```yaml
version: "3.8"

services:
  # ===== Infrastructure =====
  mongodb:
    image: mongo:6.0
    ports:
      - "27017:27017"
    volumes:
      - mongodb_data:/data/db
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_USER}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_PASSWORD}

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data

  rabbitmq:
    image: rabbitmq:3.12-management
    ports:
      - "5672:5672"
      - "15672:15672"
    environment:
      RABBITMQ_DEFAULT_USER: ${RABBITMQ_USER}
      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_PASSWORD}
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq

  # ===== Application Services =====
  api-gateway:
    build:
      context: .
      dockerfile: services/api-gateway/Dockerfile
    ports:
      - "8080:8080"
    environment:
      - MONGODB_URI=mongodb://${MONGO_USER}:${MONGO_PASSWORD}@mongodb:27017
      - REDIS_ADDR=redis:6379
      - RABBITMQ_URL=amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@rabbitmq:5672
    depends_on:
      - mongodb
      - redis
      - rabbitmq

  proxy-service:
    build:
      context: .
      dockerfile: services/proxy-service/Dockerfile
    ports:
      - "8081:8081"
      - "50052:50052"
    environment:
      - MONGODB_URI=mongodb://${MONGO_USER}:${MONGO_PASSWORD}@mongodb:27017
      - REDIS_ADDR=redis:6379
      - RABBITMQ_URL=amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@rabbitmq:5672
    depends_on:
      - mongodb
      - redis
      - rabbitmq

  # ... остальные сервисы аналогично

  # ===== Monitoring =====
  prometheus:
    image: prom/prometheus:v2.47.0
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus:/etc/prometheus
      - prometheus_data:/prometheus

  grafana:
    image: grafana/grafana:10.1.0
    ports:
      - "3000:3000"
    volumes:
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
      - grafana_data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}

  loki:
    image: grafana/loki:2.9.0
    ports:
      - "3100:3100"
    volumes:
      - ./monitoring/loki:/etc/loki
      - loki_data:/loki

volumes:
  mongodb_data:
  redis_data:
  rabbitmq_data:
  prometheus_data:
  grafana_data:
  loki_data:
```

### Полезные команды

```bash
# Просмотр логов конкретного сервиса
docker-compose logs -f proxy-service

# Перезапуск сервиса
docker-compose restart warming-service

# Масштабирование сервиса
docker-compose up -d --scale warming-service=3

# Остановка всех сервисов
docker-compose down

# Полная очистка (включая volumes)
docker-compose down -v
```

---

## Docker Compose (Production)

### Отличия от Development

1. **Оптимизированные образы** - multi-stage builds
2. **Health checks** - для всех сервисов
3. **Resource limits** - ограничения CPU/RAM
4. **Logging** - централизованный сбор логов
5. **Secrets** - Docker secrets вместо env vars

### docker-compose.prod.yml

```yaml
version: "3.8"

services:
  proxy-service:
    image: conveer/proxy-service:${VERSION:-latest}
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: "1.0"
          memory: 512M
        reservations:
          cpus: "0.5"
          memory: 256M
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
    secrets:
      - mongodb_password
      - redis_password
      - encryption_key

secrets:
  mongodb_password:
    external: true
  redis_password:
    external: true
  encryption_key:
    external: true
```

### Запуск Production

```bash
# Создать secrets
echo "your-mongo-password" | docker secret create mongodb_password -
echo "your-redis-password" | docker secret create redis_password -
echo "your-32-byte-key" | docker secret create encryption_key -

# Запустить в Swarm mode
docker stack deploy -c docker-compose.prod.yml conveer

# Проверить статус
docker service ls
```

---

## Kubernetes (Helm)

### Установка

```bash
# Добавить Helm repo (если есть)
helm repo add conveer https://charts.conveer.example.com
helm repo update

# Или установить из локальной директории
helm install conveer ./deploy/helm/conveer \
  --namespace conveer \
  --create-namespace \
  -f values-prod.yaml
```

### Структура Helm Chart

```
deploy/helm/conveer/
├── Chart.yaml
├── values.yaml
├── values-staging.yaml
├── values-prod.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── configmap.yaml
│   ├── secrets.yaml
│   ├── deployment-api-gateway.yaml
│   ├── deployment-proxy-service.yaml
│   ├── deployment-sms-service.yaml
│   ├── deployment-warming-service.yaml
│   ├── deployment-vk-service.yaml
│   ├── deployment-telegram-service.yaml
│   ├── deployment-telegram-bot.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   └── pdb.yaml
└── charts/
    ├── mongodb/
    ├── redis/
    └── rabbitmq/
```

### values.yaml (пример)

```yaml
global:
  imageRegistry: ""
  imagePullSecrets: []

replicaCount:
  apiGateway: 2
  proxyService: 2
  smsService: 1
  warmingService: 3
  vkService: 2
  telegramService: 2

image:
  repository: conveer
  pullPolicy: IfNotPresent
  tag: ""  # Defaults to Chart.appVersion

serviceAccount:
  create: true
  name: ""

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: api.conveer.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: conveer-tls
      hosts:
        - api.conveer.example.com

resources:
  apiGateway:
    limits:
      cpu: 500m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
  proxyService:
    limits:
      cpu: 1000m
      memory: 512Mi
    requests:
      cpu: 200m
      memory: 256Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80

mongodb:
  enabled: true
  architecture: replicaset
  replicaCount: 3
  auth:
    rootPassword: ""
    existingSecret: conveer-mongodb

redis:
  enabled: true
  architecture: replication
  auth:
    existingSecret: conveer-redis

rabbitmq:
  enabled: true
  clustering:
    enabled: true
  replicaCount: 3
  auth:
    existingPasswordSecret: conveer-rabbitmq

monitoring:
  serviceMonitor:
    enabled: true
  prometheusRule:
    enabled: true
```

### Команды управления

```bash
# Обновить релиз
helm upgrade conveer ./deploy/helm/conveer \
  --namespace conveer \
  -f values-prod.yaml

# Откатить релиз
helm rollback conveer 1 --namespace conveer

# Удалить релиз
helm uninstall conveer --namespace conveer

# Посмотреть статус
helm status conveer --namespace conveer

# Получить манифесты без установки
helm template conveer ./deploy/helm/conveer -f values-prod.yaml
```

### Kubernetes Secrets

```bash
# Создать секреты
kubectl create secret generic conveer-secrets \
  --namespace conveer \
  --from-literal=mongodb-password='your-password' \
  --from-literal=redis-password='your-password' \
  --from-literal=rabbitmq-password='your-password' \
  --from-literal=encryption-key='your-32-byte-key' \
  --from-literal=telegram-bot-token='your-token' \
  --from-literal=sms-activate-api-key='your-key'
```

---

## CI/CD Pipeline

### GitHub Actions

#### ci.yml

```yaml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      mongodb:
        image: mongo:6.0
        ports:
          - 27017:27017
      redis:
        image: redis:7
        ports:
          - 6379:6379
      rabbitmq:
        image: rabbitmq:3.12
        ports:
          - 5672:5672

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.21"

      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}

      - name: Run tests
        run: make test-coverage
        env:
          MONGODB_URI: mongodb://localhost:27017
          REDIS_ADDR: localhost:6379
          RABBITMQ_URL: amqp://guest:guest@localhost:5672

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Gosec
        uses: securego/gosec@master
        with:
          args: ./...

      - name: Run Nancy
        uses: sonatype-nexus-community/nancy-github-action@main

  build:
    needs: [test, lint, security]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./services/proxy-service/Dockerfile
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: conveer/proxy-service:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

#### cd.yml

```yaml
name: CD

on:
  push:
    tags:
      - "v*"

jobs:
  deploy-staging:
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4

      - name: Configure kubectl
        uses: azure/k8s-set-context@v4
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG_STAGING }}

      - name: Deploy to Staging
        run: |
          helm upgrade --install conveer ./deploy/helm/conveer \
            --namespace conveer-staging \
            --create-namespace \
            -f ./deploy/helm/conveer/values-staging.yaml \
            --set image.tag=${{ github.ref_name }}

  deploy-prod:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4

      - name: Configure kubectl
        uses: azure/k8s-set-context@v4
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG_PROD }}

      - name: Deploy to Production
        run: |
          helm upgrade --install conveer ./deploy/helm/conveer \
            --namespace conveer-prod \
            --create-namespace \
            -f ./deploy/helm/conveer/values-prod.yaml \
            --set image.tag=${{ github.ref_name }}
```

---

## Мониторинг

### Prometheus Endpoints

Все сервисы экспортируют метрики на `/metrics`:

| Сервис | Endpoint |
|--------|----------|
| API Gateway | http://api-gateway:8080/metrics |
| Proxy Service | http://proxy-service:8081/metrics |
| SMS Service | http://sms-service:8082/metrics |
| Warming Service | http://warming-service:8083/metrics |

### Grafana Dashboards

Преднастроенные дашборды находятся в `monitoring/grafana/dashboards/`:

- **Conveer Overview** - общая статистика
- **Proxy Service** - метрики прокси
- **Warming Service** - статистика прогрева
- **Infrastructure** - MongoDB, Redis, RabbitMQ

### Alerting

Правила алертинга в `monitoring/prometheus/alerts.yml`:

```yaml
groups:
  - name: conveer
    rules:
      - alert: HighProxyErrorRate
        expr: rate(proxy_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High proxy error rate

      - alert: LowAvailableProxies
        expr: proxies_available < 10
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: Low number of available proxies

      - alert: WarmingTasksFailing
        expr: rate(warming_tasks_failed_total[1h]) > 5
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: High warming task failure rate
```

---

## Масштабирование

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: warming-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: warming-service
  minReplicas: 2
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: warming_tasks_active
        target:
          type: AverageValue
          averageValue: "50"
```

### Рекомендации по масштабированию

| Сервис | Метрика для масштабирования | Рекомендация |
|--------|----------------------------|--------------|
| API Gateway | CPU, RPS | 1 pod на 1000 RPS |
| Proxy Service | CPU, Active proxies | 1 pod на 500 прокси |
| SMS Service | Queue depth | 1 pod на 100 активаций/мин |
| Warming Service | Active tasks | 1 pod на 100 активных задач |
| VK/Telegram Service | CPU, Memory | 1 pod на 50 параллельных действий |

### База данных

MongoDB:
- **Development**: Standalone
- **Staging**: ReplicaSet (3 nodes)
- **Production**: ReplicaSet (3 nodes) + Sharding при >100GB

Redis:
- **Development**: Standalone
- **Staging**: Sentinel (3 nodes)
- **Production**: Cluster (6+ nodes)

