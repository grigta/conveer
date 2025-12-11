# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN cd services/mail-service && go build -o /bin/mail-service cmd/main.go

# Runtime stage with Playwright
FROM mcr.microsoft.com/playwright/go:v1.40.0

# Install additional dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -u 1000 appuser

# Copy binary from builder
COPY --from=builder /bin/mail-service /bin/mail-service

# Copy configs
COPY services/mail-service/configs /app/configs

# Set working directory
WORKDIR /app

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 50061 8011

# Run the application
CMD ["/bin/mail-service"]
