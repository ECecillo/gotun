# Variables
binary_name := "tun"

# Show available recipes
default:
    @just --list

# Build binary locally
build:
    go build -o {{binary_name}} ./cmd/tun

# Run binary locally (requires sudo for kernel operations)
run-sudo: build
    sudo ./{{binary_name}}

# Run tests with race detector and coverage
test:
    go test -v -race -coverprofile=coverage.out ./...

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
