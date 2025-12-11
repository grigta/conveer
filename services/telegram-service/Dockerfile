# Build stage
FROM docker.io/conveer/base:latest AS builder

WORKDIR /app

# Copy go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main ./services/telegram-service/cmd

# Runtime stage
FROM docker.io/conveer/base:latest

# Install Playwright dependencies and browsers
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcups2 \
    libdrm2 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libxss1 \
    libxtst6 \
    xdg-utils \
    && rm -rf /var/lib/apt/lists/*

# Install Playwright
RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest
RUN playwright install chromium
RUN playwright install-deps chromium

# Set up non-root user
RUN useradd --create-home --shell /bin/bash app

WORKDIR /app

# Copy binary and configs
COPY --from=builder /app/main .
COPY services/telegram-service/configs/ ./configs/

# Create necessary directories
RUN mkdir -p /tmp/telegram-profiles && \
    chown -R app:app /app /tmp/telegram-profiles

# Set shared memory size for Chromium
RUN echo 'kernel.unprivileged_userns_clone=1' >> /etc/sysctl.conf

USER app

EXPOSE 8010 50060

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8010/health || exit 1

CMD ["./main"]