# goaquatemp

A Go SDK and CLI tool for interacting with Aqua Temp / Linked-Go heat pump systems.

## Overview

goaquatemp provides programmatic access to Aqua Temp heat pumps via their cloud API. It allows you to:

- Monitor temperature data (current, target, inlet, outlet, ambient)
- Control heat pump settings (power, mode, target temperature)
- List and manage multiple devices
- Integrate heat pump data into your own applications

## Installation

### As a library

```bash
go get github.com/user/goaquatemp
```

### CLI tool

```bash
go install github.com/user/goaquatemp/cmd/aquatemp@latest
```

## Quick Start

### Using the SDK

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/user/goaquatemp"
)

func main() {
    client, err := goaquatemp.NewClient(
        goaquatemp.WithUsername(os.Getenv("AQUATEMP_USERNAME")),
        goaquatemp.WithPassword(os.Getenv("AQUATEMP_PASSWORD")),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    if err := client.Login(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Logout(ctx)

    // List devices
    devices, err := client.GetDevices(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Get temperature data
    data, err := client.GetDeviceData(ctx, devices[0].Code)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Current: %.1f°C, Target: %.1f°C\n", data.CurrentTemp, data.TargetTemp)
}
```

### Using the CLI

```bash
# Set credentials
export AQUATEMP_USERNAME="user@example.com"
export AQUATEMP_PASSWORD="your-password"

# Test authentication
aquatemp login

# List devices
aquatemp devices list

# Get temperature data
aquatemp temp get --device DEVICE_CODE

# Poll temperature history (1 hour, every 5 minutes)
aquatemp temp history --device DEVICE_CODE --hours 1 --interval 300

# Get full device status
aquatemp status --device DEVICE_CODE

# Control device
aquatemp control set-temp --device DEVICE_CODE --temp 28
aquatemp control power --device DEVICE_CODE --on
aquatemp control mode --device DEVICE_CODE --heat
aquatemp control fan-speed --device DEVICE_CODE --speed auto
```

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AQUATEMP_USERNAME` | Account email |
| `AQUATEMP_PASSWORD` | Account password |
| `AQUATEMP_API_VERSION` | API version: `legacy`, `hitemp`, or `modern` (default) |
| `AQUATEMP_CONFIG` | Custom config file path |
| `AQUATEMP_DEFAULT_DEVICE` | Default device code |
| `AQUATEMP_DEBUG` | Enable debug logging |

### Config File

Create `~/.config/aquatemp/config.yaml`:

```yaml
username: "user@example.com"
password: "your-password"
api_version: "modern"
default_device: "DEVICE_CODE"
```

## API Versions

Aqua Temp has multiple API versions based on when your account was created:

| Version | When to use |
|---------|-------------|
| `legacy` | Accounts created before app v1.5.8 |
| `hitemp` | HiTemp branded devices |
| `modern` | Accounts created with app v1.5.9+ (default) |

If you're unsure, try `modern` first. If authentication fails, try `legacy`.

## Single Login Limitation

The Aqua Temp API only allows one active session per account. If you log in from the mobile app, it will invalidate the SDK session and vice versa.

**Recommended solution**: Create a secondary account and share your device with it. Use the secondary account for automation. See [docs/general.md](docs/general.md) for detailed instructions.

## Examples

- `examples/temperature/` - Get temperature data for a device
- `examples/monitor/` - Continuously poll temperature data at intervals

Run an example:

```bash
go run ./examples/temperature
go run ./examples/monitor
```

## Documentation

- [Design Document](docs/DESIGN.md) - Architecture, diagrams, and detailed design
- [API Reference](docs/general.md) - API endpoints and authentication details
- [Implementation Plan](docs/plan.md) - Phased implementation plan

## Building

```bash
# Show all available targets
make

# Build CLI
make build

# Run tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage report
make test-cover

# Clean build artifacts
make clean

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o aquatemp-linux-amd64 ./cmd/aquatemp
GOOS=darwin GOARCH=arm64 go build -o aquatemp-darwin-arm64 ./cmd/aquatemp
GOOS=windows GOARCH=amd64 go build -o aquatemp-windows-amd64.exe ./cmd/aquatemp
```

## License

MIT License - see [LICENSE](LICENSE) for details.
