FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o vk-service ./services/vk-service/cmd

# Final stage with Playwright
FROM mcr.microsoft.com/playwright:v1.40.0-focal

WORKDIR /app

# Install required dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Playwright browsers (only Chromium needed)
RUN npx playwright install chromium
RUN npx playwright install-deps chromium

# Copy the binary from builder
COPY --from=builder /app/vk-service .
COPY --from=builder /app/services/vk-service/configs ./configs

# Create directory for browser profiles
RUN mkdir -p /tmp/vk-profiles

# Expose ports
EXPOSE 50059 8009

# Run the application
CMD ["./vk-service"]