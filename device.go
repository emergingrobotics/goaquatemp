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

// FanSpeed represents the fan speed setting.
type FanSpeed string

const (
	FanLow    FanSpeed = "low"
	FanMedium FanSpeed = "medium"
	FanHigh   FanSpeed = "high"
	FanAuto   FanSpeed = "auto"
)

// Device represents an Aqua Temp heat pump device.
type Device struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Online      bool      `json:"online"`
	ProductCode string    `json:"productCode"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DeviceData contains the current sensor data and status for a device.
type DeviceData struct {
	DeviceCode  string            `json:"deviceCode"`
	Timestamp   time.Time         `json:"timestamp"`
	Power       bool              `json:"power"`
	Mode        OperationMode     `json:"mode"`
	TargetTemp  float64           `json:"targetTemp"`
	CurrentTemp float64           `json:"currentTemp"`
	InletTemp   float64           `json:"inletTemp"`
	OutletTemp  float64           `json:"outletTemp"`
	AmbientTemp float64           `json:"ambientTemp"`
	CoilTemp    float64           `json:"coilTemp"`
	FanSpeed    FanSpeed          `json:"fanSpeed"`
	ErrorCode   int               `json:"errorCode"`
	RawParams   map[string]string `json:"rawParams,omitempty"`
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

// IsOnline returns true if the device is online.
func (d *Device) IsOnline() bool {
	return d.Online
}

// HasError returns true if the device has an error.
func (d *DeviceData) HasError() bool {
	return d.ErrorCode != 0
}

// IsPoweredOn returns true if the device is powered on.
func (d *DeviceData) IsPoweredOn() bool {
	return d.Power
}
