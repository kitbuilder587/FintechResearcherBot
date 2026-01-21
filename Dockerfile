# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Download dependencies first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bot ./cmd/bot

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS and wget for healthcheck
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary and migrations
COPY --from=builder /bot .
COPY migrations ./migrations
COPY configs ./configs

# Use non-root user
USER appuser

EXPOSE 8080 9090

CMD ["./bot"]
