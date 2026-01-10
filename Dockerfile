# Stage 1: Builder
FROM golang:1.25.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go module files
COPY go.mod go.sum* ./

# Download dependencies (cached layer)
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimization flags
# CGO_ENABLED=0 for static binary, -s -w to strip debug symbols
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o tun \
    ./cmd/tun

# Stage 2: Development (for watch mode with docker compose)
FROM golang:1.25.5-alpine AS development

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go module files
COPY go.mod go.sum* ./

# Download dependencies (cached layer)
RUN go mod download

# Copy source code
COPY . .

# Build binary without optimization flags for faster rebuilds
RUN CGO_ENABLED=0 go build -o /app/tun ./cmd/tun

# Expose application port
EXPOSE 8080

# Run the application
CMD ["/app/tun"]

# Stage 3: Production
FROM alpine:3.21 AS production

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder stage
COPY --from=builder /build/tun /usr/local/bin/tun

# Change ownership of the binary
RUN chown appuser:appuser /usr/local/bin/tun

# Switch to non-root user
USER appuser

# Expose application port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/tun"]
