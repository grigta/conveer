# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o warming-service ./services/warming-service/cmd

# Final stage
FROM mcr.microsoft.com/playwright:v1.40.0-focal

# Install additional dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Create app user
RUN useradd -m -u 1001 appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/warming-service /app/
COPY --from=builder /app/services/warming-service/configs /app/configs

# Create directories for browser profiles
RUN mkdir -p /tmp/warming-profiles && \
    chown -R appuser:appuser /tmp/warming-profiles

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 50063 8013

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8013/health || exit 1

# Run the application
CMD ["./warming-service"]
