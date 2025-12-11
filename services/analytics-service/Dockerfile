FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копирование go.mod и go.sum из корня проекта
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -o analytics-service ./services/analytics-service/cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Копирование бинарника
COPY --from=builder /app/analytics-service .

# Копирование конфигурации
COPY --from=builder /app/services/analytics-service/configs ./configs

EXPOSE 50056 8014

CMD ["./analytics-service"]