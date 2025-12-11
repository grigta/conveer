FROM golang:1.21-alpine AS builder

RUN apk update && apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o auth-service ./services/auth/cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /build/auth-service .
COPY --from=builder /build/config ./config

EXPOSE 50051

CMD ["./auth-service"]