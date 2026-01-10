# Dockerfile.builder
# Reusable builder image for CI/CD pipelines and artifact extraction
#
# Purpose:
# - Standalone builder that can be used in CI/CD to compile binaries
# - Allows extracting the compiled binary as an artifact
# - Useful for testing build process independently
# - Can be run directly to verify binary works: docker run gotun-builder
#
# Usage:
#   Build: docker build -f Dockerfile.builder -t gotun-builder .
#   Run:   docker run --rm gotun-builder
#   Extract binary: docker cp $(docker create gotun-builder):/build/tun ./tun

FROM golang:1.25.5-alpine AS builder

# Install build dependencies
# git: Required for go mod download of git-based dependencies
# ca-certificates: Needed for HTTPS downloads during go mod download
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Layer caching optimization: Dependencies are cached separately from source
# This means changing source code doesn't invalidate the dependency download layer
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary with optimization flags
# CGO_ENABLED=0: Creates fully static binary (no libc dependency)
# GOOS=linux: Ensures Linux binary (important for CI running on different OS)
# -ldflags="-s -w": Strips debug info for smaller binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o tun \
    ./cmd/tun

# Make builder runnable for testing
# Allows quick verification: docker run --rm gotun-builder
CMD ["/build/tun"]
