# gotun

A macOS userspace tunnel (UTUN) implementation in Go that creates and configures network interfaces using kernel control sockets.

## Platform Support

**macOS only** - This project uses macOS-specific kernel control interfaces (`com.apple.net.utun_control`) and will not compile or run on Linux or Windows.

## Prerequisites

- macOS (tested on macOS 10.15+)
- [Go](https://go.dev/doc/install) 1.25.5 or later
- [Just](https://github.com/casey/just#installation) command runner (optional but recommended)
- Root/sudo privileges (required for kernel network operations)

## Quick Start

### Build and Run

```bash
# Build the binary
just build

# Run with sudo (required for kernel operations)
just run-sudo

# Or manually
go build -o tun ./cmd/tun
sudo ./tun
```

## What This Does

The program:

1. Opens a kernel control socket to communicate with the macOS kernel
2. Retrieves the UTUN (User Tunnel) controller ID via IOCTL calls
3. Connects to the UTUN controller
4. Creates a new virtual network interface (utun0, utun1, etc.)
5. Configures the interface with IP addresses:
   - Source: 10.0.0.2
   - Destination: 10.0.0.1
6. Brings the interface UP and marks it as RUNNING

This is equivalent to running:

```bash
ifconfig utun0 10.0.0.2 10.0.0.1 up
```

But done programmatically through direct kernel syscalls.

## Available Commands

Run `just` to see all available commands:

```bash
just
```

### Development Commands

| Command | Description |
|---------|-------------|
| `just build` | Build the binary locally |
| `just run-sudo` | Build and run with sudo |
| `just test` | Run unit tests with race detector and coverage |
| `sudo -E just test-integration` | Run integration tests (requires root) |
| `just test-all` | Run all tests (unit + integration) |
| `just fmt` | Format Go code |
| `just lint` | Run golangci-lint (requires installation) |
| `just clean` | Remove build artifacts and caches |
| `just security` | Run security checks with gosec |

## Configuration

All commands are configured via environment variables using the [env](https://github.com/caarlos0/env) package. Each command has its own `.env` file:

| File | Command | Example |
|------|---------|---------|
| `.env.tunnel` | `just run-forward` | `.env.tunnel.example` |
| `.env.server` | `just udp-server` | `.env.server.example` |
| `.env.devtool` | `just devtool-send` | `.env.devtool.example` |

### Setup

Copy the example files to create your env files:

```bash
cp .env.tunnel.example .env.tunnel
cp .env.server.example .env.server
cp .env.devtool.example .env.devtool

# Edit as needed
vim .env.tunnel
```

See the `.env.*.example` files for available variables and their descriptions.

## Local Testing

This section explains how to test the tunnel locally by sending UDP or TCP requests through it.

### Testing Tools

| Command | Description |
|---------|-------------|
| `just udp-server` | Start UDP server (config: `.env.server`) |
| `just run-forward` | Run tunnel with forwarding (config: `.env.tunnel`) |
| `just devtool-send` | Send UDP/TCP request (config: `.env.devtool`) |

### Full Workflow (3 Terminals)

**Terminal 1 - Start the UDP server:**

```bash
just udp-server
# UDP server listening on :8080
```

**Terminal 2 - Start the tunnel with forwarding:**

```bash
just run-forward
# UDP forwarding enabled to 127.0.0.1:8080
# Tunnel created successfully: utun0 (fd: X)
# Waiting for packets...
```

**Terminal 3 - Send a UDP request through the tunnel:**

```bash
just devtool-send
# Sent 18 bytes to 10.0.0.1:8080: hello from devtool
# Received 23 bytes: ACK: hello from devtool
```

To customize the message, edit `.env.devtool`:

```bash
DEVTOOL_MSG=hello tunnel
DEVTOOL_PROTOCOL=udp # tcp
```

### Direct Testing (without tunnel)

To test the UDP server directly without the tunnel, edit `.env.devtool`:

```bash
DEVTOOL_HOST=127.0.0.1
```

Then run:

```bash
# Terminal 1
just udp-server

# Terminal 2
just devtool-send
```

## Project Structure

```
.
├── cmd/
│   ├── devtool/
│   │   └── main.go              # UDP/TCP request sender for testing
│   ├── server/
│   │   └── main.go              # Simple UDP server
│   └── tun/
│       └── macos/
│           └── main.go          # UTUN implementation
├── .github/
│   └── workflows/
│       ├── lint.yml         # Linting workflow
│       └── format.yml       # Code formatting check workflow
├── justfile                 # Task runner recipes
├── go.mod                   # Go module definition
└── README.md               # This file
```

## How It Works

### Kernel Control Sockets

The program uses macOS kernel control sockets to communicate directly with the kernel's network subsystem:

1. **AF_SYSTEM + AF_SYS_CONTROL**: Creates a socket for kernel-level communication
2. **IOCTL calls**: Retrieves controller information and configures network interfaces
3. **Direct syscalls**: Uses `unix.Syscall` for low-level IOCTL operations not exposed by the Go standard library

### Key Components

- `openKernelControlSocket()`: Establishes connection to kernel control subsystem
- `retrieveUTUNControllerIDFromKernelController()`: Gets UTUN controller ID via IOCTL
- `connectToUTUNController()`: Connects to the UTUN controller
- `askForUTUNNetworkInterfaceName()`: Retrieves assigned interface name (utun0, etc.)
- `configureUTUNIPInterface()`: Sets IP addresses and brings interface UP

## Security Considerations

This program requires root privileges because it:

- Creates kernel control sockets
- Modifies network interfaces
- Performs low-level kernel operations

Always review code before running with sudo.

## CI/CD

The project includes GitHub Actions workflows for automated code quality checks:

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

**Note**: The test workflow has been removed as tests cannot run in Linux CI environments (this is macOS-only code).

## Troubleshooting

### Permission Denied

If you get permission errors, make sure to run with sudo:

```bash
sudo ./tun
```

### Build Errors on Non-macOS

This is expected. The code uses `//go:build darwin` build constraints and will not compile on other platforms.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linting (`just test && just lint && just fmt`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request
