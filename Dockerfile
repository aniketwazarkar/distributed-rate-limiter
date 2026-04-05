# Build Stage
FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o rate_limiter_server ./cmd/server/main.go

# Run Stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/rate_limiter_server .

EXPOSE 8080
CMD ["./rate_limiter_server"]
