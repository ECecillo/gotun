# Variables
binary_name := "tun"
docker_image := "ghcr.io/ececillo/gotun"
docker_tag := "latest"

# Show available recipes
default:
    @just --list

# Build binary locally
build:
    go build -o {{binary_name}} ./cmd/tun

# Run binary locally
run: build
    ./{{binary_name}}

# Run tests with race detector and coverage
test:
    go test -v -race -coverprofile=coverage.out ./...

# Run in watch mode with Docker Compose (auto-rebuilds on file changes)
watch:
    docker compose up --watch

# Build production Docker image
docker-build:
    docker build -f Dockerfile -t {{docker_image}}:{{docker_tag}} .

# Build builder image
docker-build-builder:
    docker build -f Dockerfile.builder -t {{docker_image}}-builder:{{docker_tag}} .

# Build dev image
docker-build-dev:
    docker build -f Dockerfile.dev -t {{docker_image}}-dev:{{docker_tag}} .

# Run production Docker image
docker-run:
    docker run --rm -p 8080:8080 {{docker_image}}:{{docker_tag}}

# Build and run production Docker image (standalone)
docker-build-run: docker-build docker-run

# Run development container with Docker Compose
docker-up:
    docker compose up

# Run development container in detached mode
docker-up-d:
    docker compose up -d

# Stop and remove containers
docker-down:
    docker compose down

# Run production container with Docker Compose
docker-prod:
    docker compose --profile prod up app-prod

# View logs from running containers
docker-logs:
    docker compose logs -f

# Clean build artifacts
clean:
    rm -f {{binary_name}}
    rm -rf tmp/
    go clean -cache -testcache -modcache

# Format code
fmt:
    go fmt ./...

# Run linter (requires golangci-lint)
lint:
    golangci-lint run ./...

# Run security check (requires gosec)
security:
    gosec ./...
