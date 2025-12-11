# Build stage
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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sms-service ./services/sms-service/cmd/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/sms-service .

# Copy config files
COPY services/sms-service/configs ./configs

# Create directory for logs
RUN mkdir -p /app/logs

# Expose ports
EXPOSE 8008 50058

# Run the binary
CMD ["./sms-service"]
