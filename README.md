# goaquatemp

A Go SDK and CLI tool for interacting with Aqua Temp / Linked-Go heat pump systems.

## Overview

goaquatemp provides programmatic access to Aqua Temp heat pumps via their cloud API. It allows you to:

- Monitor live status: power, set mode, what the unit is actually doing
  (heating / cooling / defrosting / idle), compressor (running, Hz, % of max),
  water flow, and inlet / outlet / ambient / coil / suction / exhaust temperatures
- Read the recorded history behind the app's charts (inlet, outlet, ambient,
  compressor frequency) at 5-minute, hourly, daily or monthly resolution
- Check online status and active faults
- Browse the device's full parameter catalogs (codes, values, labels, ranges)
- Control heat pump settings (power, mode, target temperature, silent fan)
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
    "time"

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

    fmt.Printf("State: %s, inlet %.1f°C, outlet %.1f°C, ambient %.1f°C, target %.1f°C\n",
        data.State, data.InletTemp, data.OutletTemp, data.AmbientTemp, data.TargetTemp)
    fmt.Printf("Inlet in °F: %.1f\n", goaquatemp.CelsiusToFahrenheit(data.InletTemp))
    // With no water flow the water temperatures are stagnant and not valid.
    if inlet := data.ValidInletTemp(); inlet != nil {
        fmt.Printf("Pool temperature: %.1f°C\n", *inlet)
    } else {
        fmt.Printf("Pool temperature: NA (%s)\n", data.WaterTempStatus)
    }
    if data.CompressorRunning != nil && *data.CompressorRunning {
        fmt.Printf("Compressor running at %.0f Hz\n", *data.CompressorFrequency)
    }

    // Recorded history (the data behind the app's charts)
    hist, err := client.GetHistory(ctx, devices[0].Code, goaquatemp.HistoryQuery{
        Series:     goaquatemp.SeriesInletTemp,
        Start:      time.Now().Add(-24 * time.Hour),
        Resolution: goaquatemp.ResolutionHourly,
    })
    if err != nil {
        log.Fatal(err)
    }
    for _, p := range hist.Points {
        fmt.Printf("%s %.1f°C\n", p.Time.Format("15:04"), p.Value)
    }
}
```

### Using the CLI

The SDK returns temperatures in °C, as the cloud reports them. The CLI shows
every temperature in both °C and °F: side by side in tables, as extra `_f`
fields or `(°F)` columns in CSV, and as a `fahrenheit` object in
`status --json`.

```bash
# Set credentials
export AQUATEMP_USERNAME="user@example.com"
export AQUATEMP_PASSWORD="your-password"

# Test authentication
aquatemp login

# List devices
aquatemp devices list

# Full status (without --device, commands use the account's first device)
aquatemp status
aquatemp status --json

# Recorded history: last 24h of all series at 5-minute resolution
aquatemp history
aquatemp history --since 7d --series inlet,outlet --resolution hour
aquatemp history --from 2026-11-01 --to 2026-11-15 --resolution day --csv

# Parameter catalogs (status values, or writable settings)
aquatemp params --filter temp
aquatemp params --kind settings

# Current temperatures, and live polling
aquatemp temp get
aquatemp temp history --hours 1 --interval 300

# Control device (writes to the heat pump)
aquatemp control set-temp --device DEVICE_CODE --temp 28
aquatemp control set-temp --temp-f 85           # Fahrenheit (converted, rounded to 0.1°C)
aquatemp control power --device DEVICE_CODE --on
aquatemp control mode --device DEVICE_CODE --heat
aquatemp control fan-speed --device DEVICE_CODE --speed auto   # auto or low (silent)
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
| `hitemp` | HiTemp branded devices (same endpoint as modern) |
| `modern` | Accounts created with app v1.5.9+ (default) |

If you're unsure, try `modern` first. If authentication fails, try `legacy`.

## Single Login Limitation

The Aqua Temp API only allows one active session per account. If you log in from the mobile app, it will invalidate the SDK session and vice versa.

**Recommended solution**: Create a secondary account and share your device with it. Use the secondary account for automation. See [docs/general.md](docs/general.md) for detailed instructions.

## Examples

- `examples/temperature/` - Get temperature data for a device
- `examples/monitor/` - Continuously poll status (state, compressor, temperatures)
- `examples/history/` - Print the last 24 hours of recorded history

Run an example:

```bash
go run ./examples/temperature
go run ./examples/monitor
go run ./examples/history
```

## Documentation

- [Design Document](docs/DESIGN.md) - Architecture, diagrams, and detailed design
- [Cloud API Reference](docs/api-docs.md) - Every known endpoint, request/response formats, codes, history
- [Account setup](docs/general.md) - API background and the second-account setup
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
