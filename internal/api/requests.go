package api

// LoginRequest represents the login API request body.
type LoginRequest struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
	Type     string `json:"type"`
}

// NewLoginRequest creates a new login request.
func NewLoginRequest(username, password string) *LoginRequest {
	return &LoginRequest{
		UserName: username,
		Password: password,
		Type:     "2", // Mobile app type
	}
}

// DeviceListRequest represents the device list API request body.
type DeviceListRequest struct {
	ProductIDs []string `json:"productIds"`
}

// NewDeviceListRequest creates a new device list request.
func NewDeviceListRequest() *DeviceListRequest {
	return &DeviceListRequest{
		ProductIDs: []string{},
	}
}

// DeviceDataRequest represents the device data API request body.
type DeviceDataRequest struct {
	DeviceCode string   `json:"device_code"`
	Protocols  []string `json:"protocal_codes"` // Note: API uses "protocal" (typo)
}

// NewDeviceDataRequest creates a new device data request.
func NewDeviceDataRequest(deviceCode string) *DeviceDataRequest {
	return &DeviceDataRequest{
		DeviceCode: deviceCode,
		Protocols: []string{
			ParamPower,
			ParamMode,
			ParamTargetTemp,
			ParamCurrentTemp,
			ParamInletTemp,
			ParamOutletTemp,
			ParamAmbientTemp,
			ParamCoilTemp,
			ParamFanSpeed,
			ParamErrorCode,
		},
	}
}

// ControlRequest represents the device control API request body.
type ControlRequest struct {
	DeviceCode   string `json:"device_code"`
	ProtocolCode string `json:"protocal_code"` // Note: API uses "protocal" (typo)
	Value        string `json:"value"`
}

// NewControlRequest creates a new control request.
func NewControlRequest(deviceCode, protocolCode, value string) *ControlRequest {
	return &ControlRequest{
		DeviceCode:   deviceCode,
		ProtocolCode: protocolCode,
		Value:        value,
	}
}

// ControlValue represents a single control parameter.
type ControlValue struct {
	ProtocolCode string `json:"protocal_code"`
	Value        string `json:"value"`
}

// MultiControlRequest represents a multi-parameter control request.
type MultiControlRequest struct {
	DeviceCode string         `json:"device_code"`
	Values     []ControlValue `json:"value"`
}

// NewMultiControlRequest creates a new multi-control request.
func NewMultiControlRequest(deviceCode string, values []ControlValue) *MultiControlRequest {
	return &MultiControlRequest{
		DeviceCode: deviceCode,
		Values:     values,
	}
}
