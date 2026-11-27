package goaquatemp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/emergingrobotics/goaquatemp/internal/api"
	"github.com/emergingrobotics/goaquatemp/internal/auth"
	"github.com/emergingrobotics/goaquatemp/internal/transport"
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

	// GetDeviceStatus returns whether the device is online and its active faults.
	GetDeviceStatus(ctx context.Context, deviceCode string) (*DeviceStatus, error)

	// GetHistory returns a recorded series (the data behind the app's charts).
	GetHistory(ctx context.Context, deviceCode string, query HistoryQuery) (*History, error)

	// GetParameters returns one of the device's parameter catalogs.
	GetParameters(ctx context.Context, deviceCode string, kind ParameterKind) ([]Parameter, error)

	// IsAuthenticated returns true if the client has a valid token.
	IsAuthenticated() bool

	// Config returns the client configuration.
	Config() *Config
}

// client is the default implementation of Client.
type client struct {
	config  *Config
	dialect api.Dialect
	auth    auth.AuthManager
	http    transport.HTTPClient

	mu          sync.Mutex
	protocalIDs map[string]string // productId -> protocalId
	now         func() time.Time
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

	httpClient := transport.NewClient(
		config.GetBaseURL(),
		config.Timeout,
		config.RetryCount,
		config.Debug,
	)

	authManager := auth.NewManager(
		httpClient,
		string(config.APIVersion),
		config.Username,
		config.Password,
	)

	return &client{
		config:      config,
		dialect:     api.DialectFor(string(config.APIVersion)),
		auth:        authManager,
		http:        httpClient,
		protocalIDs: make(map[string]string),
		now:         time.Now,
	}, nil
}

// Login authenticates with the Aqua Temp API. It does nothing if the client
// already holds a valid token; call Logout first to force a new login.
func (c *client) Login(ctx context.Context) error {
	if err := c.config.Validate(); err != nil {
		return err
	}
	if c.auth.IsAuthenticated() {
		return nil
	}
	if _, err := c.auth.Authenticate(ctx); err != nil {
		return wrapAuthError("login failed", err)
	}
	return nil
}

// Logout clears the authentication token.
func (c *client) Logout(ctx context.Context) error {
	c.auth.Clear()
	return nil
}

// GetDevices returns all devices owned by or shared with the account.
func (c *client) GetDevices(ctx context.Context) ([]Device, error) {
	resp, err := c.call(ctx, api.EndpointDeviceList, c.dialect.DeviceListRequest())
	if err != nil {
		return nil, fmt.Errorf("failed to get device list: %w", err)
	}
	owned, err := c.dialect.ParseDevices(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse device list: %w", err)
	}

	// Devices shared from another account (the recommended setup for
	// automation, since the app allows only one login per account). Failure
	// here is not fatal: the account may simply have no shares.
	var shared []api.DeviceInfo
	if userID, err := c.userID(ctx); err == nil && userID != "" {
		if resp, err := c.call(ctx, api.EndpointSharedDeviceList, c.dialect.SharedDeviceListRequest(userID)); err == nil {
			shared, _ = c.dialect.ParseDevices(resp.Result)
		}
	}

	seen := make(map[string]bool)
	var devices []Device
	for _, list := range [][]api.DeviceInfo{owned, shared} {
		for i := range list {
			if list[i].DeviceCode == "" || seen[list[i].DeviceCode] {
				continue
			}
			seen[list[i].DeviceCode] = true
			devices = append(devices, deviceInfoToDevice(&list[i]))
		}
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
	req := c.dialect.DeviceDataRequest(deviceCode, api.DefaultDataCodes, api.DefaultBitCodes)
	resp, err := c.call(ctx, c.dialect.DataEndpoint, req)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getDeviceData", Err: err}
	}

	params, err := api.ParseDataParams(resp.Result)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "parseDeviceData", Err: err}
	}

	data := parsedDataToDeviceData(deviceCode, api.ParseDeviceData(params))
	return &data, nil
}

// SetTemperature sets the target temperature for the device's current mode.
func (c *client) SetTemperature(ctx context.Context, deviceCode string, temp float64) error {
	return c.Control(ctx, ControlCommand{DeviceCode: deviceCode, TargetTemp: &temp})
}

// SetMode sets the operation mode. ModeOff turns the device off; any other
// mode also turns it on.
func (c *client) SetMode(ctx context.Context, deviceCode string, mode OperationMode) error {
	return c.Control(ctx, ControlCommand{DeviceCode: deviceCode, Mode: &mode})
}

// SetPower turns a device on or off.
func (c *client) SetPower(ctx context.Context, deviceCode string, on bool) error {
	return c.Control(ctx, ControlCommand{DeviceCode: deviceCode, Power: &on})
}

// SetFanSpeed sets the fan speed (FanAuto or FanLow).
func (c *client) SetFanSpeed(ctx context.Context, deviceCode string, speed FanSpeed) error {
	return c.Control(ctx, ControlCommand{DeviceCode: deviceCode, FanSpeed: &speed})
}

// Control sends a compound control command to a device as a single request.
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
	if command.FanSpeed != nil {
		if err := ValidateFanSpeed(*command.FanSpeed); err != nil {
			return err
		}
	}

	var values []api.ControlValue

	// Power: explicit, or implied by the mode (off => power off, else on).
	power := command.Power
	if command.Mode != nil {
		on := *command.Mode != ModeOff
		if power == nil || !on {
			power = &on
		}
	}
	if power != nil {
		v := api.APIPowerOff
		if *power {
			v = api.APIPowerOn
		}
		values = append(values, api.ControlValue{ProtocolCode: api.ParamPowerControl, Value: v})
	}

	// The API value of the mode the device will be in, needed to pick the
	// target-temperature protocol code.
	apiMode := ""
	if command.Mode != nil && *command.Mode != ModeOff {
		apiMode = modeToAPI(*command.Mode)
		values = append(values, api.ControlValue{ProtocolCode: api.ParamMode, Value: apiMode})
	}

	if command.TargetTemp != nil {
		if apiMode == "" {
			data, err := c.GetDeviceData(ctx, command.DeviceCode)
			if err != nil {
				return err
			}
			apiMode = data.RawParams[api.ParamMode]
		}
		target := api.TargetCodeForMode(apiMode)
		if target == "" {
			target = api.ParamHeatSet
		}
		values = append(values,
			api.ControlValue{ProtocolCode: target, Value: *command.TargetTemp},
			api.ControlValue{ProtocolCode: api.ParamSetTemp, Value: *command.TargetTemp},
		)
	}

	if command.FanSpeed != nil {
		values = append(values, api.ControlValue{ProtocolCode: api.ParamFanSpeed, Value: fanSpeedToAPI(*command.FanSpeed)})
	}

	if len(values) == 0 {
		return nil
	}

	if _, err := c.call(ctx, api.EndpointControl, c.dialect.ControlRequest(command.DeviceCode, values)); err != nil {
		return &DeviceError{DeviceCode: command.DeviceCode, Operation: "control", Err: err}
	}
	return nil
}

// GetDeviceStatus returns whether the device is online and its active faults.
func (c *client) GetDeviceStatus(ctx context.Context, deviceCode string) (*DeviceStatus, error) {
	req := c.dialect.DeviceStatusRequest(deviceCode)
	resp, err := c.call(ctx, api.EndpointDeviceStatus, req)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getDeviceStatus", Err: err}
	}
	st, err := c.dialect.ParseDeviceStatus(resp.Result)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "parseDeviceStatus", Err: err}
	}

	status := &DeviceStatus{
		DeviceCode: deviceCode,
		Online:     st.Status == "ONLINE",
		Status:     st.Status,
		Fault:      st.IsFault,
	}
	if st.IsFault {
		resp, err := c.call(ctx, api.EndpointDeviceFaults, req)
		if err != nil {
			return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getFaults", Err: err}
		}
		faults, err := api.ParseFaults(resp.Result)
		if err != nil {
			return nil, &DeviceError{DeviceCode: deviceCode, Operation: "parseFaults", Err: err}
		}
		for _, f := range faults {
			status.Faults = append(status.Faults, Fault{Description: f.Description, Raw: f.Raw})
		}
	}
	return status, nil
}

// GetHistory returns a recorded series.
//
// Times in the result are in the location of query.Start. The API takes a
// whole-hour UTC offset, so in zones with a fractional offset the buckets are
// aligned to the offset truncated to whole hours.
func (c *client) GetHistory(ctx context.Context, deviceCode string, query HistoryQuery) (*History, error) {
	if query.Series == "" {
		return nil, &ValidationError{Field: "series", Value: "", Message: "series is required"}
	}
	end := query.End
	if end.IsZero() {
		end = c.now().In(query.Start.Location())
	}
	if query.Start.IsZero() || !query.Start.Before(end) {
		return nil, &ValidationError{Field: "start", Value: query.Start, Message: "start must be set and before end"}
	}

	res := query.Resolution
	if res == ResolutionAuto {
		res = autoResolution(end.Sub(query.Start))
	}
	frequency, layout, err := historyFrequency(res)
	if err != nil {
		return nil, &ValidationError{Field: "resolution", Value: res, Message: err.Error()}
	}

	// The 5-minute series is stored in UTC+8 and only lines up with the
	// returned slots when the window is sent in UTC+8 with timeZone 8
	// (probed 2026-11-17). The other resolutions honour timeZone.
	tzHours, zone := requestZone(query.Start)
	if res == Resolution5Min {
		tzHours, zone = 8, chinaTime
	}
	// Slots are laid out from the start time as sent, so align it to a
	// bucket boundary or the slots miss the stored samples.
	reqStart := alignToBucket(res, query.Start.In(zone))
	const reqLayout = "2006-01-02 15:04:05"
	req := c.dialect.HistoryRequest(deviceCode, string(query.Series),
		reqStart.Format(reqLayout), end.In(zone).Format(reqLayout), frequency, tzHours)

	resp, err := c.call(ctx, api.EndpointHistory, req)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getHistory", Err: err}
	}
	raw, err := api.ParseHistory(resp.Result)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "parseHistory", Err: err}
	}
	if raw.ConfigID == "" && len(raw.Points) == 0 {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getHistory",
			Err: fmt.Errorf("series %s: %w", query.Series, ErrHistoryUnavailable)}
	}

	labelZone := zone

	h := &History{
		DeviceCode: deviceCode,
		Series:     query.Series,
		Name:       query.Series.Name(),
		Title:      raw.Title,
		Unit:       query.Series.Unit(),
		Resolution: res,
	}

	type sample struct {
		t time.Time
		v float64
	}
	var samples []sample
	for _, p := range raw.Points {
		t, err := time.ParseInLocation(layout, p.DateTime, labelZone)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(p.Value, 64)
		if err != nil {
			continue
		}
		samples = append(samples, sample{t, v})
	}

	now := c.now()
	loc := query.Start.Location()
	for i, s := range samples {
		// Keep buckets overlapping [Start, End].
		if !s.t.Add(bucketLength(res, s.t)).After(query.Start) || s.t.After(end) {
			continue
		}
		if !query.KeepGaps {
			if s.t.After(now) {
				continue // future buckets read 0
			}
			if res == Resolution5Min && s.v == 0 && isGap(query.Series, i, func(j int) (float64, bool) {
				if j < 0 || j >= len(samples) {
					return 0, false
				}
				return samples[j].v, true
			}) {
				continue
			}
		}
		h.Points = append(h.Points, HistoryPoint{Time: s.t.In(loc), Value: s.v})
	}
	return h, nil
}

// isGap reports whether a 0 in the 5-minute series is a missing sample. For
// temperatures every 0 is; for the compressor frequency, where 0 also means
// stopped, only a 0 between two running samples is.
func isGap(series HistorySeries, i int, at func(int) (float64, bool)) bool {
	if series != SeriesCompressorFrequency {
		return true
	}
	prev, okPrev := at(i - 1)
	next, okNext := at(i + 1)
	return okPrev && okNext && prev != 0 && next != 0
}

// GetParameters returns one of the device's parameter catalogs.
func (c *client) GetParameters(ctx context.Context, deviceCode string, kind ParameterKind) ([]Parameter, error) {
	var (
		endpoint string
		req      map[string]any
	)
	switch kind {
	case ParametersStatus:
		protocalID, err := c.protocalID(ctx, deviceCode)
		if err != nil {
			return nil, err
		}
		endpoint = api.EndpointStatusParameters
		req = c.dialect.StatusParametersRequest(deviceCode, protocalID)
	case ParametersSettings:
		endpoint = api.EndpointControlParameters
		req = c.dialect.ControlParametersRequest(deviceCode)
	default:
		return nil, &ValidationError{Field: "kind", Value: kind, Message: "kind must be status or settings"}
	}

	resp, err := c.call(ctx, endpoint+"?lang=en", req)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "getParameters", Err: err}
	}
	infos, err := api.ParseParameters(resp.Result)
	if err != nil {
		return nil, &DeviceError{DeviceCode: deviceCode, Operation: "parseParameters", Err: err}
	}
	params := make([]Parameter, len(infos))
	for i, p := range infos {
		params[i] = Parameter(p)
	}
	return params, nil
}

// protocalID returns the protocol id of the device's product, which the
// status catalog needs. Cached per product.
func (c *client) protocalID(ctx context.Context, deviceCode string) (string, error) {
	device, err := c.GetDevice(ctx, deviceCode)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	id, ok := c.protocalIDs[device.ProductCode]
	c.mu.Unlock()
	if ok {
		return id, nil
	}

	resp, err := c.call(ctx, api.EndpointProductInfo+"?lang=en", c.dialect.ProductInfoRequest(device.ProductCode))
	if err != nil {
		return "", &DeviceError{DeviceCode: deviceCode, Operation: "getProductInfo", Err: err}
	}
	id, err = api.ParseProtocalID(resp.Result)
	if err != nil {
		return "", &DeviceError{DeviceCode: deviceCode, Operation: "parseProductInfo", Err: err}
	}
	if id == "" {
		return "", &DeviceError{DeviceCode: deviceCode, Operation: "getProductInfo",
			Err: errors.New("product has no protocol id")}
	}

	c.mu.Lock()
	c.protocalIDs[device.ProductCode] = id
	c.mu.Unlock()
	return id, nil
}

// IsAuthenticated returns true if the client has a valid token.
func (c *client) IsAuthenticated() bool {
	return c.auth.IsAuthenticated()
}

// Config returns the client configuration.
func (c *client) Config() *Config {
	return c.config
}

// call posts an authenticated request and returns the successful response.
// If the server reports the token invalid (-100), it logs in again once and
// retries.
func (c *client) call(ctx context.Context, endpoint string, req any) (*api.Response, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	resp, err := c.post(ctx, endpoint, req)
	if err != nil {
		return nil, err
	}

	if resp.ErrorCode == api.ErrCodeTokenInvalid {
		c.auth.Clear()
		if err := c.ensureAuth(ctx); err != nil {
			return nil, err
		}
		if resp, err = c.post(ctx, endpoint, req); err != nil {
			return nil, err
		}
	}

	if !resp.IsSuccess() {
		return nil, &APIError{ErrorCode: resp.ErrorCode, Message: resp.ErrorMessage}
	}
	return resp, nil
}

func (c *client) post(ctx context.Context, endpoint string, req any) (*api.Response, error) {
	body, err := c.http.Post(ctx, c.dialect.Path(endpoint), req, c.auth.FormatTokenHeader())
	if err != nil {
		return nil, err
	}
	return c.dialect.ParseResponse(body)
}

// userID returns the logged-in user's id, from the login response or, if it
// was absent, from getUserInfo.
func (c *client) userID(ctx context.Context) (string, error) {
	if tok := c.auth.GetToken(); tok != nil && tok.UserID != "" {
		return tok.UserID, nil
	}
	resp, err := c.call(ctx, api.EndpointUserInfo, nil)
	if err != nil {
		return "", err
	}
	return c.dialect.ParseUserID(resp.Result)
}

// ensureAuth ensures we have a valid authentication token.
func (c *client) ensureAuth(ctx context.Context) error {
	if c.auth.IsAuthenticated() {
		return nil
	}
	if err := c.config.Validate(); err != nil {
		return err
	}
	if err := c.auth.EnsureAuthenticated(ctx); err != nil {
		return wrapAuthError("not authenticated", err)
	}
	return nil
}

// wrapAuthError converts an internal auth error into the public AuthError,
// preserving the server's code and message.
func wrapAuthError(msg string, err error) error {
	var ae *auth.AuthError
	if !errors.As(err, &ae) {
		return &AuthError{Code: -1, Message: msg, Err: err}
	}
	out := &AuthError{Code: ae.Code, Message: ae.Message, Retryable: ae.Retryable, Err: ae.Err}
	if ae.Rejected {
		out.Err = ErrInvalidCredentials
	}
	return out
}

// Conversion functions

func deviceInfoToDevice(info *api.DeviceInfo) Device {
	return Device{
		Code:        info.DeviceCode,
		Name:        info.Nickname,
		Online:      info.IsOnline(),
		Status:      info.Status,
		ProductCode: info.ProductID,
	}
}

func parsedDataToDeviceData(deviceCode string, parsed *api.ParsedDeviceData) DeviceData {
	mode := parseMode(parsed.Mode)
	if !parsed.Power {
		mode = ModeOff
	}

	running := parsed.CompressorRunning
	if running == nil && parsed.CompressorFreq != nil {
		r := *parsed.CompressorFreq > 0
		running = &r
	}

	return DeviceData{
		DeviceCode:             deviceCode,
		Timestamp:              time.Now(),
		Power:                  parsed.Power,
		Mode:                   mode,
		State:                  operatingState(parsed, running),
		FanSpeed:               parseFanSpeed(parsed.FanSpeed),
		TargetTemp:             parsed.TargetTemp,
		TargetMin:              parsed.TargetMin,
		TargetMax:              parsed.TargetMax,
		CurrentTemp:            parsed.InletTemp,
		InletTemp:              parsed.InletTemp,
		OutletTemp:             parsed.OutletTemp,
		AmbientTemp:            parsed.AmbientTemp,
		CoilTemp:               parsed.CoilTemp,
		SuctionTemp:            parsed.SuctionTemp,
		ExhaustTemp:            parsed.ExhaustTemp,
		CompressorRunning:      running,
		CompressorFrequency:    parsed.CompressorFreq,
		CompressorMaxFrequency: parsed.CompressorMaxFreq,
		CompressorSpeed:        compressorSpeed(parsed),
		CompressorCurrent:      parsed.CompressorCurrent,
		InverterInputVoltage:   parsed.InverterInputVoltage,
		RealPowerRaw:           parsed.RealPowerRaw,
		EnergyTotal:            parsed.EnergyTotal,
		WaterFlowing:           parsed.WaterFlowing,
		WaterTempStatus:        WaterTempStatusFromFlow(parsed.WaterFlowing),
		ExternalStop:           parsed.ExternalStop,
		SensorFaults:           parsed.SensorFaults,
		RawParams:              parsed.RawParams,
	}
}

// operatingState works out what the unit is doing from power, the operating
// state code (ModeState), the compressor and, failing those, the set mode.
func operatingState(p *api.ParsedDeviceData, running *bool) OperatingState {
	if !p.Power {
		return StateOff
	}
	if p.ModeState == api.APIStateDefrosting {
		return StateDefrosting
	}
	if running != nil && !*running {
		return StateIdle
	}
	switch p.ModeState {
	case api.APIStateHeating:
		return StateHeating
	case api.APIStateCooling:
		return StateCooling
	}
	if running == nil {
		return StateUnknown
	}
	switch p.Mode {
	case api.APIModeHeat:
		return StateHeating
	case api.APIModeCool:
		return StateCooling
	case api.APIModeAuto:
		// The app's rule: heating while the auto target is above the inlet.
		if p.TargetTemp > p.InletTemp {
			return StateHeating
		}
		return StateCooling
	}
	return StateUnknown
}

// compressorSpeed is the compressor frequency as a percentage of its maximum
// for the current direction, as the app shows it. Nil for fixed-speed units
// (compressor type 0) or when a value is missing.
func compressorSpeed(p *api.ParsedDeviceData) *float64 {
	if p.CompressorFreq == nil || (p.CompressorType != nil && *p.CompressorType == 0) {
		return nil
	}
	if *p.CompressorFreq == 0 {
		zero := 0.0
		return &zero
	}
	if p.CompressorMaxFreq == nil || *p.CompressorMaxFreq <= 0 {
		return nil
	}
	pct := math.Round(*p.CompressorFreq / *p.CompressorMaxFreq * 1000) / 10
	return &pct
}

func parseMode(value string) OperationMode {
	switch value {
	case api.APIModeHeat:
		return ModeHeat
	case api.APIModeCool:
		return ModeCool
	case api.APIModeAuto:
		return ModeAuto
	default:
		return OperationMode("unknown(" + value + ")")
	}
}

func parseFanSpeed(value string) FanSpeed {
	if value == api.APIFanLow {
		return FanLow
	}
	return FanAuto
}

func modeToAPI(mode OperationMode) string {
	switch mode {
	case ModeHeat:
		return api.APIModeHeat
	case ModeCool:
		return api.APIModeCool
	case ModeAuto:
		return api.APIModeAuto
	default:
		return ""
	}
}

func fanSpeedToAPI(speed FanSpeed) string {
	if speed == FanLow {
		return api.APIFanLow
	}
	return api.APIFanAuto
}
