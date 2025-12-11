# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make gcc musl-dev

# Copy go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN cd services/telegram-service && CGO_ENABLED=0 GOOS=linux go build -o /bin/telegram-service cmd/main.go

# Runtime stage with Playwright
FROM mcr.microsoft.com/playwright:v1.40.0-focal

# Install CA certificates and curl for healthcheck
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /bin/telegram-service /bin/telegram-service

# Copy configs
COPY services/telegram-service/configs /app/configs

# Create profiles directory
RUN mkdir -p /tmp/telegram-profiles

# Set working directory
WORKDIR /app

# Expose ports
EXPOSE 8010 50060

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8010/health || exit 1

CMD ["/bin/telegram-service"]
