# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files from root
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN cd services/telegram-bot && CGO_ENABLED=0 GOOS=linux go build -o /bin/telegram-bot ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and configs
COPY --from=builder /bin/telegram-bot .
COPY services/telegram-bot/configs ./configs

EXPOSE 8080

CMD ["./telegram-bot"]
