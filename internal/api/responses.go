package api

import (
	"strconv"
	"time"
)

// BaseResponse represents the common response structure from the API.
type BaseResponse struct {
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_msg"`
}

// IsSuccess returns true if the response indicates success.
func (r *BaseResponse) IsSuccess() bool {
	return r.ErrorCode == 0
}

// LoginResponse represents the login API response.
type LoginResponse struct {
	BaseResponse
	ObjectResult *LoginResult `json:"objectResult"`
}

// LoginResult contains the authentication result data.
type LoginResult struct {
	Token     string `json:"x-token"`
	UserID    string `json:"userId"`
	ExpiresIn int    `json:"expiresIn"`
}

// DeviceListResponse represents the device list API response.
type DeviceListResponse struct {
	BaseResponse
	ObjectResult []DeviceInfo `json:"objectResult"`
}

// DeviceInfo represents a device in the device list.
type DeviceInfo struct {
	DeviceCode   string `json:"device_code"`
	DeviceNick   string `json:"device_nick"`
	DeviceType   string `json:"device_type"`
	OnlineStatus string `json:"online_status"`
	ProductCode  string `json:"product_code"`
	CreateTime   string `json:"create_time"`
}

// IsOnline returns true if the device is online.
func (d *DeviceInfo) IsOnline() bool {
	return d.OnlineStatus == "1"
}

// ParsedCreateTime returns the parsed creation time.
func (d *DeviceInfo) ParsedCreateTime() time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", d.CreateTime)
	return t
}

// DeviceDataResponse represents the device data API response.
type DeviceDataResponse struct {
	BaseResponse
	ObjectResult []Parameter `json:"objectResult"`
}

// Parameter represents a single device parameter.
type Parameter struct {
	Code  string `json:"protocal_code"`
	Value string `json:"value"`
}

// ControlResponse represents the control API response.
type ControlResponse struct {
	BaseResponse
}

// ParsedDeviceData holds the parsed device data values.
type ParsedDeviceData struct {
	Power       bool
	Mode        string
	TargetTemp  float64
	CurrentTemp float64
	InletTemp   float64
	OutletTemp  float64
	AmbientTemp float64
	CoilTemp    float64
	FanSpeed    string
	ErrorCode   int
	RawParams   map[string]string
}

// ParseDeviceData parses the device data response into structured data.
func (r *DeviceDataResponse) ParseDeviceData() *ParsedDeviceData {
	data := &ParsedDeviceData{
		RawParams: make(map[string]string),
	}

	for _, p := range r.ObjectResult {
		data.RawParams[p.Code] = p.Value

		switch p.Code {
		case ParamPower:
			data.Power = p.Value == APIPowerOn
		case ParamMode:
			data.Mode = p.Value
		case ParamTargetTemp:
			data.TargetTemp = parseFloat(p.Value)
		case ParamCurrentTemp:
			data.CurrentTemp = parseFloat(p.Value)
		case ParamInletTemp:
			data.InletTemp = parseFloat(p.Value)
		case ParamOutletTemp:
			data.OutletTemp = parseFloat(p.Value)
		case ParamAmbientTemp:
			data.AmbientTemp = parseFloat(p.Value)
		case ParamCoilTemp:
			data.CoilTemp = parseFloat(p.Value)
		case ParamFanSpeed:
			data.FanSpeed = p.Value
		case ParamErrorCode:
			data.ErrorCode = parseInt(p.Value)
		}
	}

	return data
}

func parseFloat(value string) float64 {
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

func parseInt(value string) int {
	i, _ := strconv.Atoi(value)
	return i
}
