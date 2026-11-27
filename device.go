package goaquatemp

import "time"

// OperationMode represents the heat pump operation mode.
type OperationMode string

const (
	ModeOff  OperationMode = "off"
	ModeHeat OperationMode = "heat"
	ModeCool OperationMode = "cool"
	ModeAuto OperationMode = "auto"
)

// OperatingState is what the heat pump is actually doing, as opposed to the
// mode it is set to.
type OperatingState string

const (
	StateOff        OperatingState = "off"
	StateIdle       OperatingState = "idle" // on, compressor stopped
	StateHeating    OperatingState = "heating"
	StateCooling    OperatingState = "cooling"
	StateDefrosting OperatingState = "defrosting"
	StateUnknown    OperatingState = "unknown"
)

// WaterTempStatus says whether the water temperatures (CurrentTemp,
// InletTemp, OutletTemp) can be trusted.
//
// The water sensors sit in the heat pump's own plumbing. With no flow (the
// circulation pump is off) they read water standing in the pipe, which drifts
// toward the air temperature and says nothing about the pool.
type WaterTempStatus string

const (
	// WaterTempValid: the flow switch reports water flowing.
	WaterTempValid WaterTempStatus = "valid"
	// WaterTempNoFlow: the flow switch reports no flow. The readings are of
	// stagnant water and must not be used as the pool temperature.
	WaterTempNoFlow WaterTempStatus = "no_flow"
	// WaterTempUnknown: the flow switch was not reported (legacy API), so
	// the readings can't be checked.
	WaterTempUnknown WaterTempStatus = "unknown"
)

// FanSpeed represents the fan setting (the API's "Manual-mute" parameter).
// Devices support only auto and low (silent).
type FanSpeed string

const (
	FanLow  FanSpeed = "low"
	FanAuto FanSpeed = "auto"
)

// Device represents an Aqua Temp heat pump device.
type Device struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Online      bool   `json:"online"`
	Status      string `json:"status"`
	ProductCode string `json:"productCode"`
}

// DeviceData contains the current sensor data and status for a device.
//
// Pointer fields are nil when the device (or the API version) does not report
// the value. Temperatures are in °C.
type DeviceData struct {
	DeviceCode string    `json:"deviceCode"`
	Timestamp  time.Time `json:"timestamp"`

	Power bool `json:"power"`
	// Mode is the mode the unit is set to (ModeOff when powered off).
	Mode OperationMode `json:"mode"`
	// State is what the unit is doing right now.
	State    OperatingState `json:"state"`
	FanSpeed FanSpeed       `json:"fanSpeed"`

	TargetTemp float64  `json:"targetTemp"`
	TargetMin  *float64 `json:"targetMin,omitempty"`
	TargetMax  *float64 `json:"targetMax,omitempty"`

	// CurrentTemp is the inlet water temperature, which the app shows as the
	// current temperature.
	//
	// CurrentTemp, InletTemp and OutletTemp always hold the value the unit
	// reported, even when there is no flow. Check WaterTempStatus (or use
	// ValidCurrentTemp, ValidInletTemp, ValidOutletTemp) before trusting them.
	CurrentTemp float64  `json:"currentTemp"`
	InletTemp   float64  `json:"inletTemp"`
	OutletTemp  float64  `json:"outletTemp"`
	AmbientTemp float64  `json:"ambientTemp"`
	CoilTemp    float64  `json:"coilTemp"`
	SuctionTemp *float64 `json:"suctionTemp,omitempty"`
	ExhaustTemp *float64 `json:"exhaustTemp,omitempty"`

	// CompressorRunning comes from the compressor status bit (modern API), or
	// from the compressor frequency when the bit is unavailable.
	CompressorRunning *bool `json:"compressorRunning,omitempty"`
	// CompressorFrequency is the compressor output frequency in Hz.
	CompressorFrequency *float64 `json:"compressorFrequency,omitempty"`
	// CompressorMaxFrequency is the configured maximum for the current
	// direction (heating or cooling), in Hz.
	CompressorMaxFrequency *float64 `json:"compressorMaxFrequency,omitempty"`
	// CompressorSpeed is CompressorFrequency as a percentage of
	// CompressorMaxFrequency, as the app shows it. Nil for fixed-speed units.
	CompressorSpeed   *float64 `json:"compressorSpeed,omitempty"`
	CompressorCurrent *float64 `json:"compressorCurrent,omitempty"`

	// InverterInputVoltage is the inverter board's AC input voltage (T14),
	// in V. The parameter catalog labels it "A"; the values (around 220-230)
	// are volts. With CompressorCurrent it gives a rough compressor power
	// estimate (V x A); that leaves out the fan, controls and power factor.
	InverterInputVoltage *float64 `json:"inverterInputVoltage,omitempty"`
	// RealPowerRaw is Z04 ("Real power value", catalog range 0-100 kW) as
	// reported. Its unit is unverified: a probe read 24.3 while V x A was about
	// 1,525 W. Calibrate against a real meter before using it.
	RealPowerRaw *float64 `json:"realPowerRaw,omitempty"`
	// EnergyTotal is Z06, the unit's accumulated electricity counter. Probably
	// kWh (it rose 460.6 -> 471.1 over ten days of light use); unverified.
	// Differences between readings give energy used.
	EnergyTotal *float64 `json:"energyTotal,omitempty"`

	// WaterFlowing reports the water flow switch (modern API only).
	WaterFlowing *bool `json:"waterFlowing,omitempty"`
	// WaterTempStatus is derived from WaterFlowing. When it is
	// WaterTempNoFlow the water temperatures are stagnant and not valid.
	WaterTempStatus WaterTempStatus `json:"waterTempStatus"`
	// ExternalStop is true when a wired input is holding the unit off; it
	// then can't be turned on remotely (modern API only).
	ExternalStop *bool `json:"externalStop,omitempty"`
	// SensorFaults lists the temperature sensors reporting a fault.
	SensorFaults []string `json:"sensorFaults,omitempty"`

	// RawParams holds every value returned, by protocol code.
	RawParams map[string]string `json:"rawParams,omitempty"`
}

// DeviceStatus is a device's connectivity and fault status.
type DeviceStatus struct {
	DeviceCode string  `json:"deviceCode"`
	Online     bool    `json:"online"`
	Status     string  `json:"status"` // ONLINE or OFFLINE
	Fault      bool    `json:"fault"`
	Faults     []Fault `json:"faults,omitempty"`
}

// Fault is one active fault reported by the cloud.
type Fault struct {
	Description string         `json:"description"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// ControlCommand represents a command to control a device.
type ControlCommand struct {
	DeviceCode string
	Power      *bool
	Mode       *OperationMode
	TargetTemp *float64
	FanSpeed   *FanSpeed
}

// Temperature limits.
const (
	MinTemperature = 15.0
	MaxTemperature = 40.0
)

// ValidateTemperature checks if a temperature value is within valid range.
func ValidateTemperature(temp float64) error {
	if temp < MinTemperature || temp > MaxTemperature {
		return &ValidationError{
			Field:   "temperature",
			Value:   temp,
			Message: "temperature must be between 15°C and 40°C",
			Err:     ErrInvalidTemperature,
		}
	}
	return nil
}

// ValidateMode checks if an operation mode is valid.
func ValidateMode(mode OperationMode) error {
	switch mode {
	case ModeOff, ModeHeat, ModeCool, ModeAuto:
		return nil
	default:
		return &ValidationError{
			Field:   "mode",
			Value:   mode,
			Message: "invalid operation mode",
			Err:     ErrInvalidMode,
		}
	}
}

// ValidateFanSpeed checks if a fan speed is supported.
func ValidateFanSpeed(speed FanSpeed) error {
	switch speed {
	case FanAuto, FanLow:
		return nil
	default:
		return &ValidationError{
			Field:   "fanSpeed",
			Value:   speed,
			Message: "fan speed must be auto or low",
		}
	}
}

// IsOnline returns true if the device is online.
func (d *Device) IsOnline() bool {
	return d.Online
}

// IsPoweredOn returns true if the device is powered on.
func (d *DeviceData) IsPoweredOn() bool {
	return d.Power
}

// WaterTempStatusFromFlow maps the flow switch to a WaterTempStatus. A nil
// flow (not reported) gives WaterTempUnknown.
func WaterTempStatusFromFlow(flowing *bool) WaterTempStatus {
	switch {
	case flowing == nil:
		return WaterTempUnknown
	case *flowing:
		return WaterTempValid
	default:
		return WaterTempNoFlow
	}
}

// WaterTempsValid reports whether the water temperatures may be used. It is
// false only when the flow switch reports no flow; when flow is unknown
// (legacy API) it is true, since there is nothing to say otherwise.
// Callers that need confirmed flow should test
// WaterTempStatus == WaterTempValid instead.
func (d *DeviceData) WaterTempsValid() bool {
	return d.WaterTempStatus != WaterTempNoFlow
}

// ValidCurrentTemp returns CurrentTemp, or nil when there is no water flow.
func (d *DeviceData) ValidCurrentTemp() *float64 { return d.validWaterTemp(d.CurrentTemp) }

// ValidInletTemp returns InletTemp, or nil when there is no water flow.
func (d *DeviceData) ValidInletTemp() *float64 { return d.validWaterTemp(d.InletTemp) }

// ValidOutletTemp returns OutletTemp, or nil when there is no water flow.
func (d *DeviceData) ValidOutletTemp() *float64 { return d.validWaterTemp(d.OutletTemp) }

func (d *DeviceData) validWaterTemp(v float64) *float64 {
	if !d.WaterTempsValid() {
		return nil
	}
	return &v
}

// CelsiusToFahrenheit converts a temperature reported by the API (always °C)
// to °F.
func CelsiusToFahrenheit(c float64) float64 {
	return c*9/5 + 32
}

// FahrenheitToCelsius converts °F to °C, the unit the API expects.
func FahrenheitToCelsius(f float64) float64 {
	return (f - 32) * 5 / 9
}

// CelsiusDeltaToFahrenheit converts a temperature difference (such as
// outlet minus inlet) from °C to °F. Unlike an absolute temperature there is
// no 32° offset.
func CelsiusDeltaToFahrenheit(d float64) float64 {
	return d * 9 / 5
}
