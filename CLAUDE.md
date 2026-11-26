# CLAUDE.md

This file provides guidance for Claude Code when working on the goaquatemp project.

## Project Overview

goaquatemp is a Go module that provides an SDK and CLI tool for interacting with the Aqua Temp / Linked-Go cloud API for heat pump control and monitoring.

## Key Documentation

- `docs/DESIGN.md` - Detailed design document with architecture diagrams, data models, and sequence diagrams
- `docs/api-docs.md` - **Cloud API reference**: endpoints, wire formats, protocol codes, status bits, history (probed live)
- `docs/general.md` - API background and account setup

## Project Structure

```
goaquatemp/
├── client.go           # Main client interface and implementation
├── config.go           # Configuration types and defaults
├── device.go           # Device, DeviceData, DeviceStatus types
├── history.go          # History types (series, resolutions)
├── parameters.go       # Parameter catalog types
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

| Version | Base URL | Fields / paths |
|---------|----------|----------------|
| Legacy (pre-v1.5.8) | `cloud.linked-go.com/cloudservice/api` | snake_case, `.json` suffix |
| HiTemp | `cloud.linked-go.com:449/crmservice/api` | same as modern |
| Modern (v1.5.9+) | `cloud.linked-go.com:449/crmservice/api` | camelCase, no suffix |

All versions send the MD5 hex of the password. The per-version differences live in
`api.Dialect` (`internal/api/requests.go`).

## Key Endpoints

Modern paths (legacy appends `.json`); see `docs/api-docs.md` for formats.

- `POST /app/user/login` - Authentication, returns `x-token` (sent back bare in the `x-token` header)
- `POST /app/device/deviceList`, `/app/device/getMyAppectDeviceShareDataList` - List devices
- `POST /app/device/v5/getDataByCode` - Device values (`codes`) and status bits (`bitCodes`)
- `POST /app/device/getDeviceStatus`, `/app/device/getFaultDataByDeviceCode` - Online / faults
- `POST /app/device/control` - Control (`{"param":[{deviceCode, protocolCode, value}]}`)
- `POST /device/snapshot/listCollectData` - History (not under `/app`)
- `POST /app/device/getControlDetailStatusByDeviceCode`, `/app/device/getControlParamConfigByDeviceCode` - Parameter catalogs

Responses: `error_code` is a string; `"-100"` means the token is invalid (re-login and retry once).
The account allows one session at a time.

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
5. Add a handler to the fake server in `internal/testutil/server.go` (mirror the real wire format) and tests
6. Add the method to `mockClient` in `cmd/aquatemp/main_test.go`
7. Add CLI command if user-facing in `cmd/aquatemp/`
8. Document the endpoint in `docs/api-docs.md`

### Adding a new device parameter

1. Add the code to `DefaultDataCodes` (or a bit to `DefaultBitCodes`) in `internal/api/endpoints.go`
2. Parse it in `ParseDeviceData` (`internal/api/responses.go`)
3. Add the field to `DeviceData` in `device.go` and map it in `parsedDataToDeviceData` (`client.go`)
4. Update CLI output formatters if needed (`cmd/aquatemp/features.go`)

Use `aquatemp params` against a real device to discover codes, labels and history addresses.
