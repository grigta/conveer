FROM golang:1.24-alpine AS builder

RUN apk update && apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o analytics-service ./services/analytics/cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /build/analytics-service .
COPY --from=builder /build/config ./config

EXPOSE 50056

CMD ["./analytics-service"]
