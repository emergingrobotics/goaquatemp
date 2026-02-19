# CLAUDE.md

This file provides guidance for Claude Code when working on the goaquatemp project.

## Project Overview

goaquatemp is a Go module that provides an SDK and CLI tool for interacting with the Aqua Temp / Linked-Go cloud API for heat pump control and monitoring.

## Key Documentation

- `docs/DESIGN.md` - Detailed design document with architecture diagrams, data models, and sequence diagrams
- `docs/general.md` - API documentation including endpoints, authentication, and account setup

## Project Structure

```
goaquatemp/
├── client.go           # Main client interface and implementation
├── config.go           # Configuration types and defaults
├── device.go           # Device types and methods
├── errors.go           # Custom error types
├── options.go          # Functional options for client
├── internal/           # Private implementation details
│   ├── auth/           # Authentication and token management
│   ├── api/            # API endpoint definitions and request/response handling
│   └── transport/      # HTTP client wrapper
├── cmd/aquatemp/       # CLI tool
└── examples/           # Example programs
```

## Code Style

- Follow standard Go conventions and `gofmt`
- Use functional options pattern for client configuration (e.g., `WithUsername()`, `WithTimeout()`)
- Keep public API in root package, implementation details in `internal/`
- Use `context.Context` for all API calls
- Return concrete error types that implement `error` interface
- Use interfaces for testability (e.g., `HTTPClient`, `AuthManager`)

## API Versions

The Aqua Temp API has three versions - code must handle all:

| Version | Base URL | Password |
|---------|----------|----------|
| Legacy (pre-v1.5.8) | `cloud.linked-go.com/cloudservice/api` | Plain text |
| HiTemp | `cloud.linked-go.com/cloudservice/api` | Plain text |
| Modern (v1.5.9+) | `cloud.linked-go.com:449/crmservice/api` | MD5 hashed |

## Key Endpoints

- `POST /app/user/login.json` - Authentication, returns x-token
- `POST /app/device/deviceList.json` - List devices
- `POST /app/device/getDataByCode.json` - Get device sensor data
- `POST /app/device/control.json` - Control device settings

## Testing

- Unit tests should mock HTTP responses
- Use `httptest` package for integration tests
- E2E tests require real credentials (skip in CI)

## Build Commands

```bash
# Show all available targets
make

# Build the CLI
make build

# Run tests
make test

# Run with race detector
make test-race

# Clean build artifacts
make clean

# Run example
go run ./examples/temperature/main.go
```

## Environment Variables

For testing and development:

```bash
export AQUATEMP_USERNAME="user@example.com"
export AQUATEMP_PASSWORD="password"
export AQUATEMP_API_VERSION="modern"  # legacy, hitemp, or modern
export AQUATEMP_DEBUG="true"
```

## Common Tasks

### Adding a new API endpoint

1. Add endpoint constant in `internal/api/endpoints.go`
2. Add request/response types in `internal/api/requests.go` and `responses.go`
3. Add method to `Client` interface in `client.go`
4. Implement method in client implementation
5. Add CLI command if user-facing in `cmd/aquatemp/`

### Adding a new device parameter

1. Add field to `DeviceData` struct in `device.go`
2. Map the parameter code in the response parser
3. Update CLI output formatters if needed
