# gotun

A Go application with complete Docker, development tooling, and CI/CD setup.

## Prerequisites

- [Go](https://go.dev/doc/install) 1.25.5 or later
- [Docker](https://docs.docker.com/get-docker/) 24.0+ with Compose v2.22.0+
- [Just](https://github.com/casey/just#installation) command runner (optional but recommended)

### Installing Just

```bash
# macOS
brew install just

# Linux
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# Or use Cargo
cargo install just
```

## Quick Start

### Local Development

```bash
# Build the binary
just build

# Run the application
just run

# Or combine both
go run ./cmd/tun
```

### Development with Auto-Reload

Run with Docker Compose watch mode for automatic rebuilds on file changes:

```bash
just watch
```

This will:
- Build the development Docker image
- Start the container
- Watch for file changes
- Automatically rebuild and restart on changes

## Available Commands

Run `just` to see all available commands:

```bash
just
```

### Local Development

| Command | Description |
|---------|-------------|
| `just build` | Build the binary locally |
| `just run` | Build and run the binary |
| `just test` | Run tests with race detector and coverage |
| `just fmt` | Format Go code |
| `just lint` | Run golangci-lint (requires installation) |
| `just clean` | Remove build artifacts and caches |

### Docker Commands

| Command | Description |
|---------|-------------|
| `just watch` | Run with auto-reload (recommended for development) |
| `just docker-build` | Build production Docker image |
| `just docker-run` | Run production Docker image |
| `just docker-build-run` | Build and run production image |
| `just docker-up` | Start development container with Compose |
| `just docker-up-d` | Start development container in detached mode |
| `just docker-down` | Stop and remove containers |
| `just docker-prod` | Run production container locally |
| `just docker-logs` | View container logs |

## Development Workflow

### Using Docker Compose Watch (Recommended)

The project includes Docker Compose with watch mode for the best development experience:

```bash
# Start watch mode
just watch

# Make changes to your Go files
# The container will automatically rebuild and restart
```

Press `Ctrl+C` to stop.

### Manual Docker Compose

```bash
# Start containers
docker compose up

# Run in background
docker compose up -d

# View logs
docker compose logs -f

# Stop containers
docker compose down
```

### Testing Locally

```bash
# Run tests
just test

# Or use go directly
go test -v ./...

# With coverage
go test -v -race -coverprofile=coverage.out ./...
```

## Docker Images

### Development Image

Built from `golang:1.25.5-alpine` with full development tools.

```bash
docker compose up
```

### Production Image

Optimized multi-stage build using Alpine Linux (~10-20MB):
- Static binary with stripped symbols
- Runs as non-root user
- Minimal attack surface

```bash
# Build
just docker-build

# Run
just docker-run

# Or use docker-compose with prod profile
just docker-prod
```

## CI/CD

The project includes GitHub Actions workflows for automated testing, code quality checks, and releases.

### Testing Workflow

Runs on every push to `main` and all pull requests:
- Runs tests with race detector
- Generates coverage reports
- Uploads coverage artifacts

### Lint Workflow

Runs on every push to `main` and all pull requests:
- Executes golangci-lint with 5-minute timeout
- Checks for code quality issues and best practices
- Fails build if any linting issues are found

### Format Workflow

Runs on every push to `main` and all pull requests:
- Checks code formatting with gofmt
- Ensures all Go files follow standard formatting
- Fails build if any files are not properly formatted

### Release Workflow

Triggered when pushing version tags (e.g., `v1.0.0`):

```bash
# Create and push a release tag
git tag v0.1.0
git push origin v0.1.0
```

This will:
- Build Docker images for `linux/amd64` and `linux/arm64`
- Push to GitHub Container Registry (ghcr.io)
- Tag with semantic versioning: `v0.1.0`, `v0.1`, `v0`, `latest`
- Generate build provenance attestation

### Pulling Released Images

```bash
# Pull latest release
docker pull ghcr.io/ececillo/gotun:latest

# Pull specific version
docker pull ghcr.io/ececillo/gotun:v0.1.0

# Run
docker run --rm -p 8080:8080 ghcr.io/ececillo/gotun:latest
```

## Project Structure

```
.
├── cmd/
│   └── tun/
│       └── main.go          # Application entry point
├── .github/
│   └── workflows/
│       ├── test.yml         # CI testing workflow
│       ├── lint.yml         # Linting workflow
│       ├── format.yml       # Code formatting check workflow
│       └── release.yml      # Release and publish workflow
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Compose with watch mode
├── justfile                 # Task runner recipes
├── go.mod                   # Go module definition
├── .gitignore              # Git ignore rules
└── .dockerignore           # Docker ignore rules
```

## Configuration

### Environment Variables

The application can be configured via environment variables. Add them to a `.env` file or set them in your shell:

```bash
# Example .env file
# Add your configuration here
```

### Docker Compose Override

For local customization, create a `docker-compose.override.yml`:

```yaml
services:
  app:
    environment:
      - DEBUG=true
    volumes:
      - ./data:/data
```

## Troubleshooting

### Docker Compose Watch Not Working

Ensure you have Docker Compose v2.22.0 or later:

```bash
docker compose version
```

If watch mode isn't detecting changes, try:
1. Check that files aren't in the ignore list in `docker-compose.yml`
2. Restart watch mode: `Ctrl+C` then `just watch`

### Port Already in Use

If port 8080 is already in use, change it in `docker-compose.yml`:

```yaml
ports:
  - "3000:8080"  # Use port 3000 instead
```

### Build Errors

Clean caches and rebuild:

```bash
just clean
just build
```

Or for Docker:

```bash
docker compose down
docker system prune -f
docker compose build --no-cache
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`just test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request
