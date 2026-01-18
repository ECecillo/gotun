# Variables
binary_name := "macos"

# Show available recipes
default:
    @just --list

# Build binary locally
build:
    go build -o {{binary_name}} ./cmd/tun/macos/

# Run binary locally (requires sudo for kernel operations)
run-sudo: build
    sudo ./{{binary_name}}

# Run binary with UDP forwarding to localhost (requires sudo)
run-forward host="127.0.0.1" port="8080": build
    sudo ./{{binary_name}} -forward-host={{host}} -forward-port={{port}}

# Run unit tests with race detector and coverage
test:
    go test -v -race -coverprofile=coverage.out ./...

# Run integration tests (requires sudo for kernel operations)
test-integration:
    #!/usr/bin/env bash
    if [ "$(id -u)" -ne 0 ]; then
        echo "Error: Integration tests require root privileges."
        echo "Please run: sudo -E just test-integration"
        exit 1
    fi
    go test -v -tags=integration ./pkg/tunnel/darwin/

# Run integration tests with coverage (requires sudo)
test-integration-coverage:
    #!/usr/bin/env bash
    if [ "$(id -u)" -ne 0 ]; then
        echo "Error: Integration tests require root privileges."
        echo "Please run: sudo -E just test-integration-coverage"
        exit 1
    fi
    go test -v -tags=integration -coverprofile=coverage-integration.out ./pkg/tunnel/darwin/
    go tool cover -html=coverage-integration.out -o coverage-integration.html
    echo "Coverage report generated: coverage-integration.html"

# Run all tests (unit + integration, requires sudo for integration)
test-all:
    just test
    @echo ""
    @echo "Running integration tests..."
    sudo -E just test-integration

# Clean build artifacts
clean:
    rm -f {{binary_name}}
    rm -f coverage.out coverage-integration.out
    rm -f coverage-integration.html
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

# Run UDP server (default port 8080, or pass custom port)
udp-server port="8080":
    go run ./cmd/server/main.go {{port}}

# Send UDP packet using devtool (default: 10.0.0.1:8080)
udp-send host="10.0.0.1" port="8080" msg="hello from devtool":
    go run ./cmd/devtool/main.go -host={{host}} -port={{port}} -msg="{{msg}}"

# -------------------------------
# Debugging Command Network
# -------------------------------

# MacOS command to output IPv4 table
iptable:
 netstat -nr -f inet
