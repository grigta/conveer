# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy-service ./services/proxy-service/cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/proxy-service .
COPY --from=builder /app/services/proxy-service/configs ./configs

EXPOSE 50057 8007

CMD ["./proxy-service"]
