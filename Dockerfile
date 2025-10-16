# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bot ./bot
COPY env.example .env
EXPOSE 8080
CMD ["./bot"]
