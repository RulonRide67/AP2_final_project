# Build stage — implement when application code is ready
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /auth-service ./cmd/auth

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /auth-service /usr/local/bin/auth-service
EXPOSE 50051 9090
ENTRYPOINT ["auth-service"]
