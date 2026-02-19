package goaquatemp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/user/goaquatemp/internal/api"
	"github.com/user/goaquatemp/internal/auth"
	"github.com/user/goaquatemp/internal/transport"
)

// Client provides methods for interacting with the Aqua Temp API.
type Client interface {
	// Login authenticates with the Aqua Temp API.
	Login(ctx context.Context) error

	// Logout clears the authentication token.
	Logout(ctx context.Context) error

	// GetDevices returns a list of all devices.
	GetDevices(ctx context.Context) ([]Device, error)

	// GetDevice returns a specific device by code.
	GetDevice(ctx context.Context, deviceCode string) (*Device, error)

	// GetDeviceData returns the current sensor data for a device.
	GetDeviceData(ctx context.Context, deviceCode string) (*DeviceData, error)

	// SetTemperature sets the target temperature for a device.
	SetTemperature(ctx context.Context, deviceCode string, temp float64) error

	// SetMode sets the operation mode for a device.
	SetMode(ctx context.Context, deviceCode string, mode OperationMode) error

	// SetPower turns a device on or off.
	SetPower(ctx context.Context, deviceCode string, on bool) error

	// SetFanSpeed sets the fan speed for a device.
	SetFanSpeed(ctx context.Context, deviceCode string, speed FanSpeed) error

	// Control sends a compound control command to a device.
	Control(ctx context.Context, command ControlCommand) error

	// IsAuthenticated returns true if the client has a valid token.
	IsAuthenticated() bool

	// Config returns the client configuration.
	Config() *Config
}

// client is the default implementation of Client.
type client struct {
	config *Config
	auth   auth.AuthManager
	http   transport.HTTPClient
}

// NewClient creates a new Aqua Temp client with the given options.
func NewClient(opts ...Option) (Client, error) {
	// Start with default config
	config := DefaultConfig()

	// Apply environment config first
	envConfig := LoadConfigFromEnv()
	WithConfig(envConfig)(config)

	// Load config file (AQUATEMP_CONFIG env var or default path)
	cfgPath := config.ConfigPath
	if cfgPath == "" {
		cfgPath = DefaultConfigPath()
	}
	if cfgPath != "" {
		if fileCfg, err := LoadConfigFile(cfgPath); err == nil {
			WithConfig(fileCfg)(config)
		}
	}

	// Apply provided options (highest priority)
	for _, opt := range opts {
		opt(config)
	}

	// Create HTTP transport
	httpClient := transport.NewClient(
		config.GetBaseURL(),
		config.Timeout,
		config.RetryCount,
		config.Debug,
	)

	// Create auth manager
	authManager := auth.NewManager(
		httpClient,
		string(config.APIVersion),
		config.Username,
		config.Password,
	)

	return &client{
		config: config,
		auth:   authManager,
		http:   httpClient,
	}, nil
}

// Login authenticates with the Aqua Temp API.
func (c *client) Login(ctx context.Context) error {
	if err := c.config.Validate(); err != nil {
		return err
	}

	_, err := c.auth.Authenticate(ctx)
	if err != nil {
		return &AuthError{
			Code:      -1,
			Message:   "login failed",
			Retryable: false,
			Err:       ErrInvalidCredentials,
		}
	}
	return nil
}

// Logout clears the authentication token.
func (c *client) Logout(ctx context.Context) error {
	c.auth.Clear()
	return nil
}

// GetDevices returns a list of all devices.
func (c *client) GetDevices(ctx context.Context) ([]Device, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	req := api.NewDeviceListRequest()
	body, err := c.http.Post(ctx, api.EndpointDeviceList, req, c.auth.FormatTokenHeader())
	if err != nil {
		return nil, fmt.Errorf("failed to get device list: %w", err)
	}

	var resp api.DeviceListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse device list response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, &APIError{
			ErrorCode: resp.ErrorCode,
			Message:   resp.ErrorMessage,
		}
	}

	devices := make([]Device, len(resp.ObjectResult))
	for i, d := range resp.ObjectResult {
		devices[i] = deviceInfoToDevice(&d)
	}

	return devices, nil
}

// GetDevice returns a specific device by code.
func (c *client) GetDevice(ctx context.Context, deviceCode string) (*Device, error) {
	devices, err := c.GetDevices(ctx)
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		if d.Code == deviceCode {
			return &d, nil
		}
	}

	return nil, &DeviceError{
		DeviceCode: deviceCode,
		Operation:  "get",
		Err:        ErrDeviceNotFound,
	}
}

// GetDeviceData returns the current sensor data for a device.
func (c *client) GetDeviceData(ctx context.Context, deviceCode string) (*DeviceData, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	req := api.NewDeviceDataRequest(deviceCode)
	body, err := c.http.Post(ctx, api.EndpointDeviceData, req, c.auth.FormatTokenHeader())
	if err != nil {
		return nil, &DeviceError{
			DeviceCode: deviceCode,
			Operation:  "getDeviceData",
			Err:        err,
		}
	}

	var resp api.DeviceDataResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &DeviceError{
			DeviceCode: deviceCode,
			Operation:  "parseDeviceData",
			Err:        err,
		}
	}

	if !resp.IsSuccess() {
		return nil, &APIError{
			ErrorCode: resp.ErrorCode,
			Message:   resp.ErrorMessage,
		}
	}

	parsed := resp.ParseDeviceData()
	data := parsedDataToDeviceData(deviceCode, parsed)
	return &data, nil
}

// SetTemperature sets the target temperature for a device.
func (c *client) SetTemperature(ctx context.Context, deviceCode string, temp float64) error {
	if err := ValidateTemperature(temp); err != nil {
		return err
	}

	return c.sendControl(ctx, deviceCode, api.ParamTargetTemp, strconv.FormatFloat(temp, 'f', 1, 64))
}

// SetMode sets the operation mode for a device.
func (c *client) SetMode(ctx context.Context, deviceCode string, mode OperationMode) error {
	if err := ValidateMode(mode); err != nil {
		return err
	}

	return c.sendControl(ctx, deviceCode, api.ParamMode, modeToAPI(mode))
}

// SetPower turns a device on or off.
func (c *client) SetPower(ctx context.Context, deviceCode string, on bool) error {
	value := api.APIPowerOff
	if on {
		value = api.APIPowerOn
	}

	return c.sendControl(ctx, deviceCode, api.ParamPower, value)
}

// SetFanSpeed sets the fan speed for a device.
func (c *client) SetFanSpeed(ctx context.Context, deviceCode string, speed FanSpeed) error {
	return c.sendControl(ctx, deviceCode, api.ParamFanSpeed, fanSpeedToAPI(speed))
}

// Control sends a compound control command to a device.
func (c *client) Control(ctx context.Context, command ControlCommand) error {
	if command.DeviceCode == "" {
		return &ValidationError{
			Field:   "deviceCode",
			Value:   command.DeviceCode,
			Message: "device code is required",
		}
	}

	if command.TargetTemp != nil {
		if err := ValidateTemperature(*command.TargetTemp); err != nil {
			return err
		}
	}

	if command.Mode != nil {
		if err := ValidateMode(*command.Mode); err != nil {
			return err
		}
	}

	var values []api.ControlValue

	if command.Power != nil {
		value := api.APIPowerOff
		if *command.Power {
			value = api.APIPowerOn
		}
		values = append(values, api.ControlValue{ProtocolCode: api.ParamPower, Value: value})
	}

	if command.Mode != nil {
		values = append(values, api.ControlValue{ProtocolCode: api.ParamMode, Value: modeToAPI(*command.Mode)})
	}

	if command.TargetTemp != nil {
		values = append(values, api.ControlValue{ProtocolCode: api.ParamTargetTemp, Value: strconv.FormatFloat(*command.TargetTemp, 'f', 1, 64)})
	}

	if command.FanSpeed != nil {
		values = append(values, api.ControlValue{ProtocolCode: api.ParamFanSpeed, Value: fanSpeedToAPI(*command.FanSpeed)})
	}

	if len(values) == 0 {
		return nil
	}

	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	for _, v := range values {
		req := api.NewControlRequest(command.DeviceCode, v.ProtocolCode, v.Value)
		body, err := c.http.Post(ctx, api.EndpointControl, req, c.auth.FormatTokenHeader())
		if err != nil {
			return &DeviceError{
				DeviceCode: command.DeviceCode,
				Operation:  "control",
				Err:        err,
			}
		}

		var resp api.ControlResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return &DeviceError{
				DeviceCode: command.DeviceCode,
				Operation:  "parseControlResponse",
				Err:        err,
			}
		}

		if !resp.IsSuccess() {
			return &APIError{
				ErrorCode: resp.ErrorCode,
				Message:   resp.ErrorMessage,
			}
		}
	}

	return nil
}

// IsAuthenticated returns true if the client has a valid token.
func (c *client) IsAuthenticated() bool {
	return c.auth.IsAuthenticated()
}

// Config returns the client configuration.
func (c *client) Config() *Config {
	return c.config
}

// sendControl sends a control command to a device.
func (c *client) sendControl(ctx context.Context, deviceCode, protocolCode, value string) error {
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	req := api.NewControlRequest(deviceCode, protocolCode, value)
	body, err := c.http.Post(ctx, api.EndpointControl, req, c.auth.FormatTokenHeader())
	if err != nil {
		return &DeviceError{
			DeviceCode: deviceCode,
			Operation:  "control",
			Err:        err,
		}
	}

	var resp api.ControlResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return &DeviceError{
			DeviceCode: deviceCode,
			Operation:  "parseControlResponse",
			Err:        err,
		}
	}

	if !resp.IsSuccess() {
		return &APIError{
			ErrorCode: resp.ErrorCode,
			Message:   resp.ErrorMessage,
		}
	}

	return nil
}

// ensureAuth ensures we have a valid authentication token.
func (c *client) ensureAuth(ctx context.Context) error {
	if c.auth.IsAuthenticated() {
		return nil
	}

	if err := c.auth.EnsureAuthenticated(ctx); err != nil {
		return &AuthError{
			Code:      -1,
			Message:   "not authenticated",
			Retryable: true,
			Err:       ErrNotAuthenticated,
		}
	}
	return nil
}

// Conversion functions

func deviceInfoToDevice(info *api.DeviceInfo) Device {
	return Device{
		Code:        info.DeviceCode,
		Name:        info.DeviceNick,
		Type:        info.DeviceType,
		Online:      info.IsOnline(),
		ProductCode: info.ProductCode,
		CreatedAt:   info.ParsedCreateTime(),
	}
}

func parsedDataToDeviceData(deviceCode string, parsed *api.ParsedDeviceData) DeviceData {
	return DeviceData{
		DeviceCode:  deviceCode,
		Timestamp:   time.Now(),
		Power:       parsed.Power,
		Mode:        parseMode(parsed.Mode),
		TargetTemp:  parsed.TargetTemp,
		CurrentTemp: parsed.CurrentTemp,
		InletTemp:   parsed.InletTemp,
		OutletTemp:  parsed.OutletTemp,
		AmbientTemp: parsed.AmbientTemp,
		CoilTemp:    parsed.CoilTemp,
		FanSpeed:    parseFanSpeed(parsed.FanSpeed),
		ErrorCode:   parsed.ErrorCode,
		RawParams:   parsed.RawParams,
	}
}

func parseMode(value string) OperationMode {
	switch value {
	case api.APIModeOff:
		return ModeOff
	case api.APIModeHeat:
		return ModeHeat
	case api.APIModeCool:
		return ModeCool
	case api.APIModeAuto:
		return ModeAuto
	default:
		return ModeOff
	}
}

func parseFanSpeed(value string) FanSpeed {
	switch value {
	case api.APIFanLow:
		return FanLow
	case api.APIFanMedium:
		return FanMedium
	case api.APIFanHigh:
		return FanHigh
	case api.APIFanAuto:
		return FanAuto
	default:
		return FanAuto
	}
}

func modeToAPI(mode OperationMode) string {
	switch mode {
	case ModeOff:
		return api.APIModeOff
	case ModeHeat:
		return api.APIModeHeat
	case ModeCool:
		return api.APIModeCool
	case ModeAuto:
		return api.APIModeAuto
	default:
		return api.APIModeOff
	}
}

func fanSpeedToAPI(speed FanSpeed) string {
	switch speed {
	case FanLow:
		return api.APIFanLow
	case FanMedium:
		return api.APIFanMedium
	case FanHigh:
		return api.APIFanHigh
	case FanAuto:
		return api.APIFanAuto
	default:
		return api.APIFanAuto
	}
}
