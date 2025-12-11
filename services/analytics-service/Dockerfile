# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o analytics-service ./services/analytics-service/cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /app

# Copy binary
COPY --from=builder /app/analytics-service .

# Copy configs
COPY --from=builder /app/services/analytics-service/configs ./configs

EXPOSE 50056 8014

CMD ["./analytics-service"]
