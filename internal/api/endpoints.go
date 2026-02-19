package api

// API endpoint paths.
const (
	EndpointLogin      = "/app/user/login.json"
	EndpointDeviceList = "/app/device/deviceList.json"
	EndpointDeviceData = "/app/device/getDataByCode.json"
	EndpointControl    = "/app/device/control.json"
)

// Parameter codes for device data.
// These codes are used to identify specific sensor values in API responses.
const (
	ParamPower       = "Power"
	ParamMode        = "Mode"
	ParamTargetTemp  = "R02" // Target/Set temperature
	ParamCurrentTemp = "T02" // Current water temperature
	ParamInletTemp   = "T03" // Inlet temperature
	ParamOutletTemp  = "T04" // Outlet temperature
	ParamAmbientTemp = "T05" // Ambient temperature
	ParamCoilTemp    = "T06" // Coil temperature
	ParamFanSpeed    = "Manual-mute"
	ParamErrorCode   = "T14" // Error code
)

// Mode values as returned by the API.
const (
	APIModeOff  = "0"
	APIModeHeat = "1"
	APIModeCool = "2"
	APIModeAuto = "3"
)

// Power values as returned by the API.
const (
	APIPowerOff = "0"
	APIPowerOn  = "1"
)

// Fan speed values as returned by the API.
const (
	APIFanLow    = "0"
	APIFanMedium = "1"
	APIFanHigh   = "2"
	APIFanAuto   = "3"
)
