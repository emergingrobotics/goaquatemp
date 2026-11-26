# goaquatemp - Design Document

A Go module for interacting with the Aqua Temp / Linked-Go cloud API for heat pump control and monitoring.

## Table of Contents

1. [Overview](#overview)
2. [Goals](#goals)
3. [Architecture](#architecture)
4. [Module Structure](#module-structure)
5. [Core Components](#core-components)
6. [Data Models](#data-models)
7. [Sequence Diagrams](#sequence-diagrams)
8. [CLI Tool Design](#cli-tool-design)
9. [Error Handling](#error-handling)
10. [Configuration](#configuration)
11. [Example Usage](#example-usage)

---

## Overview

The `goaquatemp` module provides a Go SDK for interacting with the Aqua Temp cloud API (powered by Linked-Go). It enables developers to:

- Authenticate with the Aqua Temp cloud service
- Retrieve device lists and status
- Read temperature and sensor data
- Control heat pump settings (temperature, mode, power)

The module also includes a CLI tool (`aquatemp`) for immediate command-line access to heat pump data.

---

## Goals

| Goal | Description |
|------|-------------|
| **Simplicity** | Easy-to-use API that abstracts away HTTP details |
| **Flexibility** | Support both legacy and new API endpoints |
| **Reliability** | Automatic token refresh and error recovery |
| **Extensibility** | Clean interfaces for testing and customization |
| **Immediate Value** | CLI tool for quick access to heat pump data |

---

## Architecture

### High-Level Architecture

```mermaid
block-beta
    columns 3

    block:UserApps:3
        CLI["CLI Tool\n(aquatemp)"]
        UserCode["User Applications"]
        Automation["Home Automation"]
    end

    space:3

    block:SDK:3
        Client["Client\n(goaquatemp)"]
    end

    space:3

    block:ClientInternals:3
        Auth["Auth Manager"]
        DeviceAPI["Device API"]
        ControlAPI["Control API"]
    end

    space:3

    block:Transport:3
        HTTP["HTTP Transport"]
    end

    space:3

    block:Cloud:3
        CloudAPI["Aqua Temp Cloud API\n(Linked-Go)"]
    end

    space:3

    block:Devices:3
        HP1["Heat Pump 1"]
        HP2["Heat Pump 2"]
        HPN["Heat Pump N"]
    end

    CLI --> Client
    UserCode --> Client
    Automation --> Client
    Client --> Auth
    Client --> DeviceAPI
    Client --> ControlAPI
    Auth --> HTTP
    DeviceAPI --> HTTP
    ControlAPI --> HTTP
    HTTP --> CloudAPI
    CloudAPI --> HP1
    CloudAPI --> HP2
    CloudAPI --> HPN
```

### Component Block Diagram

```mermaid
block-beta
    columns 4

    block:goaquatemp:4
        columns 4

        block:public:4
            columns 4
            A["Client"]
            B["Config"]
            C["Device"]
            D["Options"]
        end

        block:internal:4
            columns 4
            E["auth"]
            F["api"]
            G["transport"]
            H["models"]
        end
    end

    block:cmd:4
        columns 2
        CLI["cmd/aquatemp"]
        space
    end
```

---

## Module Structure

```
goaquatemp/
├── client.go           # Main client interface and implementation
├── config.go           # Configuration types and defaults
├── device.go           # Device types and methods
├── errors.go           # Custom error types
├── options.go          # Functional options for client
├── doc.go              # Package documentation
│
├── internal/
│   ├── auth/
│   │   ├── auth.go         # Authentication manager
│   │   └── token.go        # Token storage and refresh
│   │
│   ├── api/
│   │   ├── endpoints.go    # API endpoint definitions
│   │   ├── requests.go     # Request builders
│   │   └── responses.go    # Response parsers
│   │
│   └── transport/
│       └── http.go         # HTTP client wrapper
│
├── cmd/
│   └── aquatemp/
│       ├── main.go         # CLI entry point
│       ├── commands.go     # Command definitions
│       └── output.go       # Output formatting
│
├── examples/
│   └── temperature/
│       └── main.go         # Example: get temperature data
│
└── docs/
    ├── general.md          # General API documentation
    └── DESIGN.md           # This document
```

---

## Core Components

### Client Interface

```mermaid
classDiagram
    class Client {
        <<interface>>
        +Login(ctx) error
        +Logout(ctx) error
        +GetDevices(ctx) []Device, error
        +GetDevice(ctx, deviceCode) Device, error
        +GetDeviceData(ctx, deviceCode) DeviceData, error
        +SetTemperature(ctx, deviceCode, temp) error
        +SetMode(ctx, deviceCode, mode) error
        +SetPower(ctx, deviceCode, on) error
    }

    class client {
        -config Config
        -auth AuthManager
        -http HTTPClient
        +Login(ctx) error
        +Logout(ctx) error
        +GetDevices(ctx) []Device, error
        +GetDevice(ctx, deviceCode) Device, error
        +GetDeviceData(ctx, deviceCode) DeviceData, error
        +SetTemperature(ctx, deviceCode, temp) error
        +SetMode(ctx, deviceCode, mode) error
        +SetPower(ctx, deviceCode, on) error
    }

    class AuthManager {
        <<interface>>
        +Authenticate(ctx, username, password) Token, error
        +RefreshToken(ctx) Token, error
        +GetToken() Token
        +IsAuthenticated() bool
    }

    class HTTPClient {
        <<interface>>
        +Do(ctx, req) Response, error
        +SetToken(token)
    }

    Client <|.. client
    client --> AuthManager
    client --> HTTPClient
```

### Configuration

```mermaid
classDiagram
    class Config {
        +Username string
        +Password string
        +APIVersion APIVersion
        +BaseURL string
        +Timeout time.Duration
        +RetryCount int
        +Debug bool
    }

    class APIVersion {
        <<enumeration>>
        Legacy
        HiTemp
        Modern
    }

    Config --> APIVersion
```

---

## Data Models

### Device and Sensor Data

```mermaid
classDiagram
    class Device {
        +Code string
        +Name string
        +Type string
        +Online bool
        +ProductCode string
        +CreatedAt time.Time
    }

    class DeviceData {
        +DeviceCode string
        +Timestamp time.Time
        +Power bool
        +Mode OperationMode
        +TargetTemp float64
        +CurrentTemp float64
        +InletTemp float64
        +OutletTemp float64
        +AmbientTemp float64
        +CoilTemp float64
        +FanSpeed FanSpeed
        +ErrorCode int
        +RawParams map[string]string
    }

    class OperationMode {
        <<enumeration>>
        Off
        Heat
        Cool
        Auto
    }

    class FanSpeed {
        <<enumeration>>
        Low
        Medium
        High
        Auto
    }

    class ControlCommand {
        +DeviceCode string
        +Power *bool
        +Mode *OperationMode
        +TargetTemp *float64
        +FanSpeed *FanSpeed
    }

    DeviceData --> OperationMode
    DeviceData --> FanSpeed
    ControlCommand --> OperationMode
    ControlCommand --> FanSpeed
```

### API Response Models

```mermaid
classDiagram
    class APIResponse {
        +Error int
        +ErrorMessage string
        +Data interface{}
    }

    class LoginResponse {
        +Token string
        +UserID string
        +ExpiresIn int
    }

    class DeviceListResponse {
        +Devices []DeviceInfo
    }

    class DeviceDataResponse {
        +ObjectResult []Parameter
    }

    class Parameter {
        +Code string
        +Value string
    }

    APIResponse <|-- LoginResponse
    APIResponse <|-- DeviceListResponse
    APIResponse <|-- DeviceDataResponse
    DeviceDataResponse --> Parameter
```

---

## Sequence Diagrams

### Authentication Flow

```mermaid
sequenceDiagram
    autonumber
    participant App as Application
    participant Client as goaquatemp.Client
    participant Auth as AuthManager
    participant HTTP as HTTPClient
    participant API as Aqua Temp API

    App->>Client: NewClient(config)
    Client->>Auth: NewAuthManager()
    Client->>HTTP: NewHTTPClient()
    Client-->>App: client

    App->>Client: Login(ctx)
    Client->>Auth: Authenticate(ctx, user, pass)

    alt Modern API (v1.5.9+)
        Auth->>Auth: MD5 hash password
    end

    Auth->>HTTP: POST /app/user/login.json
    HTTP->>API: HTTP Request
    API-->>HTTP: {token, userId, expiresIn}
    HTTP-->>Auth: LoginResponse
    Auth->>Auth: Store token
    Auth-->>Client: Token
    Client-->>App: nil (success)
```

### Get Device Temperature Data

```mermaid
sequenceDiagram
    autonumber
    participant App as Application
    participant Client as goaquatemp.Client
    participant Auth as AuthManager
    participant HTTP as HTTPClient
    participant API as Aqua Temp API

    App->>Client: GetDeviceData(ctx, deviceCode)
    Client->>Auth: IsAuthenticated()

    alt Not Authenticated
        Auth-->>Client: false
        Client->>Auth: RefreshToken(ctx)
        Auth->>HTTP: POST /app/user/login.json
        HTTP->>API: HTTP Request
        API-->>HTTP: {token}
        HTTP-->>Auth: Token
    end

    Auth-->>Client: true
    Client->>HTTP: POST /app/device/getDataByCode.json
    Note right of HTTP: Headers: x-token
    HTTP->>API: HTTP Request
    API-->>HTTP: {objectResult: [...params]}
    HTTP-->>Client: DeviceDataResponse

    Client->>Client: Parse parameters
    Note right of Client: Map codes to fields:<br/>T02 → CurrentTemp<br/>T03 → TargetTemp<br/>etc.

    Client-->>App: DeviceData
```

### Control Device (Set Temperature)

```mermaid
sequenceDiagram
    autonumber
    participant App as Application
    participant Client as goaquatemp.Client
    participant HTTP as HTTPClient
    participant API as Aqua Temp API
    participant HP as Heat Pump

    App->>Client: SetTemperature(ctx, deviceCode, 28.0)
    Client->>Client: Validate temperature range

    alt Invalid Temperature
        Client-->>App: ErrInvalidTemperature
    end

    Client->>HTTP: POST /app/device/control.json
    Note right of HTTP: Body: {deviceCode, value: [...]}
    HTTP->>API: HTTP Request
    API->>HP: Send command via cloud
    HP-->>API: ACK
    API-->>HTTP: {error: 0}
    HTTP-->>Client: ControlResponse
    Client-->>App: nil (success)
```

### Complete CLI Workflow

```mermaid
sequenceDiagram
    autonumber
    participant User
    participant CLI as aquatemp CLI
    participant Config as Config Loader
    participant Client as goaquatemp.Client
    participant API as Aqua Temp API

    User->>CLI: aquatemp temp --device DEVICE001
    CLI->>Config: Load config file / env vars
    Config-->>CLI: Config{user, pass, apiVersion}

    CLI->>Client: NewClient(config)
    Client-->>CLI: client

    CLI->>Client: Login(ctx)
    Client->>API: POST /app/user/login.json
    API-->>Client: {token}
    Client-->>CLI: nil

    CLI->>Client: GetDeviceData(ctx, "DEVICE001")
    Client->>API: POST /app/device/getDataByCode.json
    API-->>Client: {params...}
    Client-->>CLI: DeviceData

    CLI->>CLI: Format output (table/json)
    CLI->>User: Display temperature data
```

---

## CLI Tool Design

### Command Structure

```mermaid
flowchart TD
    aquatemp[aquatemp]

    aquatemp --> login[login]
    aquatemp --> devices[devices]
    aquatemp --> temp[temp]
    aquatemp --> status[status]
    aquatemp --> control[control]
    aquatemp --> version[version]

    login --> login_desc["Authenticate and cache token"]

    devices --> devices_list["list - List all devices"]
    devices --> devices_info["info - Device details"]

    temp --> temp_get["get - Get current temps"]
    temp --> temp_history["history - Temperature history"]

    status --> status_desc["Show device status"]

    control --> ctrl_power["power - Turn on/off"]
    control --> ctrl_temp["set-temp - Set target temp"]
    control --> ctrl_mode["mode - Set operation mode"]

    version --> version_desc["Show version info"]
```

### CLI Commands

| Command | Subcommand | Description | Example |
|---------|------------|-------------|---------|
| `login` | - | Authenticate with API | `aquatemp login` |
| `devices` | `list` | List all devices | `aquatemp devices list` |
| `devices` | `info` | Show device details | `aquatemp devices info -d CODE` |
| `temp` | `get` | Get temperature data | `aquatemp temp get -d CODE` |
| `temp` | `history` | Get temperature history | `aquatemp temp history -d CODE --hours 24` |
| `status` | - | Show full device status | `aquatemp status -d CODE` |
| `control` | `power` | Turn device on/off | `aquatemp control power -d CODE --on` |
| `control` | `set-temp` | Set target temperature | `aquatemp control set-temp -d CODE -t 28` |
| `control` | `mode` | Set operation mode | `aquatemp control mode -d CODE --heat` |
| `version` | - | Show version | `aquatemp version` |

### CLI Output Formats

```mermaid
flowchart LR
    Output[Output Formatter]

    Output --> Table[Table Format<br/>Default, human-readable]
    Output --> JSON[JSON Format<br/>--json flag]
    Output --> CSV[CSV Format<br/>--csv flag]

    Table --> T1["Device: Pool Heater<br/>──────────────────<br/>Current:  26.5C<br/>Target:   28.0C"]

    JSON --> J1["{ current: 26.5,<br/>  target: 28.0 }"]

    CSV --> C1["metric,value<br/>current,26.5<br/>target,28.0"]
```

---

## Error Handling

### Error Types

```mermaid
classDiagram
    class Error {
        <<interface>>
        +Error() string
    }

    class AuthError {
        +Code int
        +Message string
        +Retryable bool
        +Error() string
        +Unwrap() error
    }

    class APIError {
        +StatusCode int
        +ErrorCode int
        +Message string
        +Error() string
    }

    class DeviceError {
        +DeviceCode string
        +Operation string
        +Cause error
        +Error() string
        +Unwrap() error
    }

    class ValidationError {
        +Field string
        +Value interface{}
        +Message string
        +Error() string
    }

    Error <|.. AuthError
    Error <|.. APIError
    Error <|.. DeviceError
    Error <|.. ValidationError
```

### Error Handling Flow

```mermaid
flowchart TD
    Request[API Request]

    Request --> HTTPError{HTTP Error?}
    HTTPError -->|Yes| Retry{Retryable?}
    Retry -->|Yes| Backoff[Exponential Backoff]
    Backoff --> Request
    Retry -->|No| ReturnHTTP[Return HTTP Error]

    HTTPError -->|No| ParseResponse[Parse Response]
    ParseResponse --> APIError{API Error Code?}

    APIError -->|Auth Error| TokenExpired{Token Expired?}
    TokenExpired -->|Yes| RefreshToken[Refresh Token]
    RefreshToken --> Request
    TokenExpired -->|No| ReturnAuth[Return AuthError]

    APIError -->|Device Error| ReturnDevice[Return DeviceError]
    APIError -->|No Error| ReturnSuccess[Return Success]
```

---

## Configuration

### Configuration Sources (Priority Order)

```mermaid
flowchart TD
    subgraph Priority["Configuration Priority (highest to lowest)"]
        direction TB
        P1["1. Function Options<br/>WithUsername(), WithPassword()"]
        P2["2. Environment Variables<br/>AQUATEMP_USERNAME, AQUATEMP_PASSWORD"]
        P3["3. Config File<br/>~/.config/aquatemp/config.yaml"]
        P4["4. Default Values"]

        P1 --> P2 --> P3 --> P4
    end
```

### Configuration File Format

```yaml
# ~/.config/aquatemp/config.yaml
username: "user@example.com"
password: "secret"           # Or use AQUATEMP_PASSWORD env var
api_version: "modern"        # legacy, hitemp, or modern
base_url: ""                 # Override auto-detected URL
timeout: "30s"
retry_count: 3
debug: false

# Default device (optional)
default_device: "DEVICE001"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AQUATEMP_USERNAME` | Account username/email | - |
| `AQUATEMP_PASSWORD` | Account password | - |
| `AQUATEMP_API_VERSION` | API version (legacy/hitemp/modern) | `modern` |
| `AQUATEMP_BASE_URL` | Override base URL | Auto-detected |
| `AQUATEMP_TIMEOUT` | Request timeout | `30s` |
| `AQUATEMP_DEBUG` | Enable debug logging | `false` |
| `AQUATEMP_CONFIG` | Config file path | `~/.config/aquatemp/config.yaml` |

---

## Example Usage

### Basic Example: Get Temperature Data

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/emergingrobotics/goaquatemp"
)

func main() {
    // Create client with configuration
    client, err := goaquatemp.NewClient(
        goaquatemp.WithUsername(os.Getenv("AQUATEMP_USERNAME")),
        goaquatemp.WithPassword(os.Getenv("AQUATEMP_PASSWORD")),
        goaquatemp.WithAPIVersion(goaquatemp.APIModern),
        goaquatemp.WithTimeout(30*time.Second),
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    ctx := context.Background()

    // Authenticate
    if err := client.Login(ctx); err != nil {
        log.Fatalf("Login failed: %v", err)
    }
    defer client.Logout(ctx)

    // Get list of devices
    devices, err := client.GetDevices(ctx)
    if err != nil {
        log.Fatalf("Failed to get devices: %v", err)
    }

    if len(devices) == 0 {
        log.Fatal("No devices found")
    }

    // Get temperature data for first device
    device := devices[0]
    data, err := client.GetDeviceData(ctx, device.Code)
    if err != nil {
        log.Fatalf("Failed to get device data: %v", err)
    }

    // Display temperature data
    fmt.Printf("Device: %s (%s)\n", device.Name, device.Code)
    fmt.Printf("Timestamp: %s\n", data.Timestamp.Format(time.RFC3339))
    fmt.Println("─────────────────────────────")
    fmt.Printf("Power:       %s\n", formatPower(data.Power))
    fmt.Printf("Mode:        %s\n", data.Mode)
    fmt.Printf("Target Temp: %.1f°C\n", data.TargetTemp)
    fmt.Printf("Current:     %.1f°C\n", data.CurrentTemp)
    fmt.Printf("Inlet:       %.1f°C\n", data.InletTemp)
    fmt.Printf("Outlet:      %.1f°C\n", data.OutletTemp)
    fmt.Printf("Ambient:     %.1f°C\n", data.AmbientTemp)
    fmt.Printf("Fan Speed:   %s\n", data.FanSpeed)
}

func formatPower(on bool) string {
    if on {
        return "ON"
    }
    return "OFF"
}
```

### Advanced Example: Monitor Temperature with Callbacks

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/emergingrobotics/goaquatemp"
)

func main() {
    client, _ := goaquatemp.NewClient(
        goaquatemp.WithConfigFile("~/.config/aquatemp/config.yaml"),
    )

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    client.Login(ctx)
    defer client.Logout(ctx)

    devices, _ := client.GetDevices(ctx)
    deviceCode := devices[0].Code

    // Poll every 5 minutes
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            data, err := client.GetDeviceData(ctx, deviceCode)
            if err != nil {
                log.Printf("Error: %v", err)
                continue
            }

            fmt.Printf("[%s] Current: %.1f°C, Target: %.1f°C\n",
                time.Now().Format("15:04"),
                data.CurrentTemp,
                data.TargetTemp,
            )
        }
    }
}
```

---

## API Version Differences

```mermaid
flowchart TD
    subgraph Legacy["Legacy API (pre-v1.5.8)"]
        L1[Base URL: cloud.linked-go.com/cloudservice/api]
        L2[Password: Plain text]
        L3[Token: x-token header]
    end

    subgraph HiTemp["HiTemp API"]
        H1[Base URL: cloud.linked-go.com/cloudservice/api]
        H2[Password: Plain text]
        H3[Token: x-token header]
        H4[Additional device types]
    end

    subgraph Modern["Modern API (v1.5.9+)"]
        M1[Base URL: cloud.linked-go.com:449/crmservice/api]
        M2[Password: MD5 hashed]
        M3[Token: x-token header]
        M4[Single login enforcement]
    end
```

---

## Security Considerations

1. **Credential Storage**: Never store passwords in plain text; use environment variables or encrypted config files
2. **Token Management**: Tokens should be stored securely and refreshed before expiration
3. **HTTPS Only**: All API communication uses HTTPS
4. **Single Login**: Be aware that logging in will invalidate other sessions; use a dedicated account for automation

---

## Testing Strategy

```mermaid
flowchart TD
    subgraph Unit["Unit Tests"]
        U1[Auth Manager Tests]
        U2[Response Parser Tests]
        U3[Config Loader Tests]
        U4[Validation Tests]
    end

    subgraph Integration["Integration Tests"]
        I1[Mock HTTP Server]
        I2[Full Client Flow]
        I3[Error Scenarios]
    end

    subgraph E2E["E2E Tests (Optional)"]
        E1[Real API Tests]
        E2[Requires Test Account]
    end

    Unit --> Integration --> E2E
```

---

## Future Enhancements

1. **Local API Support**: Direct communication with heat pump (if supported)
2. **Prometheus Metrics**: Export metrics for monitoring
3. **Webhook Support**: Push notifications for status changes
4. **Multi-device Dashboard**: TUI for monitoring multiple devices
5. **Historical Data Export**: Export temperature history to CSV/InfluxDB
