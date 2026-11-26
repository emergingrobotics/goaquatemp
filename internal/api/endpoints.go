package api

// API endpoint paths, without the dialect suffix (see Dialect.Path).
//
// Wire format verified against the radical-squared/aquatemp Home Assistant
// integration (custom_components/aqua_temp/common/endpoints.py).
const (
	EndpointLogin             = "/app/user/login"
	EndpointUserInfo          = "/app/user/getUserInfo"
	EndpointDeviceList        = "/app/device/deviceList"
	EndpointSharedDeviceList  = "/app/device/getMyAppectDeviceShareDataList"
	EndpointDeviceStatus      = "/app/device/getDeviceStatus"
	EndpointDeviceData        = "/app/device/getDataByCode"
	EndpointDeviceDataV5      = "/app/device/v5/getDataByCode"
	EndpointControl           = "/app/device/control"
	EndpointDeviceFaults      = "/app/device/getFaultDataByDeviceCode"
	EndpointProductInfo       = "/app/product/queryProductById"
	EndpointStatusParameters  = "/app/device/getControlDetailStatusByDeviceCode"
	EndpointControlParameters = "/app/device/getControlParamConfigByDeviceCode"
	EndpointHistory           = "/device/snapshot/listCollectData" // note: not under /app
)

// ErrCodeTokenInvalid is returned by the API (as error_code "-100", message
// "请重新登录" / "please log in again") when the x-token is missing, expired, or
// was invalidated by a login from another client.
const ErrCodeTokenInvalid = -100

// Protocol codes used when reading device data.
const (
	ParamPower     = "Power"
	ParamMode      = "Mode"
	ParamSetTemp   = "Set_Temp"
	ParamFanSpeed  = "Manual-mute"
	ParamCoolSet   = "R01" // Cooling target temperature
	ParamHeatSet   = "R02" // Heating target temperature
	ParamAutoSet   = "R03" // Auto-mode target temperature
	ParamCoolMin   = "R08"
	ParamCoolMax   = "R09"
	ParamHeatMin   = "R10"
	ParamHeatMax   = "R11"
	ParamSuction   = "T01" // Suction temperature
	ParamInletTemp = "T02" // Inlet water temperature (used as "current" temperature)
	ParamOutlet    = "T03" // Outlet water temperature
	ParamCoilTemp  = "T04" // Coil 1 temperature
	ParamAmbient   = "T05" // Ambient temperature
	ParamExhaust   = "T06" // Exhaust temperature

	ParamModeState      = "ModeState" // What the unit is doing: 0 cooling, 1 heating, 2 defrosting
	ParamCompFreq       = "O07"       // Compressor output frequency (Hz)
	ParamCompCurrent    = "O08"       // Compressor current (A)
	ParamHeatMaxFreq    = "H08"       // Compressor max frequency when heating (Hz)
	ParamCoolMaxFreq    = "H09"       // Compressor max frequency when cooling (Hz)
	ParamCompType       = "H12"       // Compressor type; 0 = fixed speed (inferred)
	ParamEnergyTotal    = "Z06"       // Accumulated electricity (probably kWh, unverified)
	ParamRealPower      = "Z04"       // Real power value (unit unverified)
	ParamInverterVolts  = "T14"       // Inverter board AC input voltage
	ParamSilentModeFlag = "2064"      // Silent display flag
)

// ModeState values.
const (
	APIStateCooling    = "0"
	APIStateHeating    = "1"
	APIStateDefrosting = "2"
)

// Status bits, readable only through the v5 data endpoint's bitCodes array.
const (
	BitCompressor         = "Sys1CompState"   // 1 = compressor running
	BitWaterFlow          = "WaterFlowSwitch" // 0 = water flowing, 1 = no flow
	BitEmergencyStop      = "EmergencySwitch" // 1 = external input holding the unit off
	BitHighPressure       = "Sys1HighPressureSwitch"
	BitLowPressure        = "Sys1LowPressureSwitch"
	BitInletSensorFault   = "InletWaterTempSensorFault"
	BitOutletSensorFault  = "OutletWaterTempSensorFault"
	BitAmbientSensorFault = "ATSensorFault"
)

// DefaultBitCodes is the set of status bits requested from the v5 endpoint.
var DefaultBitCodes = []string{
	BitCompressor, BitWaterFlow, BitEmergencyStop, BitHighPressure, BitLowPressure,
	BitInletSensorFault, BitOutletSensorFault, BitAmbientSensorFault,
}

// ParamPowerControl is the protocol code used when *setting* power. The
// reference integration lower-cases it for control requests.
const ParamPowerControl = "power"

// DefaultDataCodes is the set of protocol codes requested from getDataByCode.
var DefaultDataCodes = []string{
	ParamPower, ParamMode, ParamSetTemp, ParamFanSpeed,
	ParamCoolSet, ParamHeatSet, ParamAutoSet,
	ParamCoolMin, ParamCoolMax, ParamHeatMin, ParamHeatMax,
	ParamSuction, ParamInletTemp, ParamOutlet, ParamCoilTemp, ParamAmbient, ParamExhaust,
	ParamModeState, ParamCompFreq, ParamCompCurrent, ParamHeatMaxFreq, ParamCoolMaxFreq, ParamCompType,
	ParamEnergyTotal, ParamRealPower, ParamInverterVolts,
}

// History addresses (register addresses from the status parameter catalog)
// that have history configured on Aqua Temp pool heat pumps.
const (
	HistoryAddrInletTemp   = "2046"
	HistoryAddrOutletTemp  = "2047"
	HistoryAddrAmbientTemp = "2049"
	HistoryAddrCompFreq    = "2021"
)

// History frequency values. The names describe the app's chart span, not the
// bucket size.
const (
	HistoryFreqWeek  = "week"  // 5-minute points, labels in UTC+8
	HistoryFreqDay   = "day"   // hourly points
	HistoryFreqMonth = "month" // daily points
	HistoryFreqYear  = "year"  // monthly points
)

// Mode values. There is no "off" mode; off is Power=0.
const (
	APIModeCool = "0"
	APIModeHeat = "1"
	APIModeAuto = "2"
)

// Power values.
const (
	APIPowerOff = "0"
	APIPowerOn  = "1"
)

// Manual-mute values. The device only supports auto and low (silent) fan.
const (
	APIFanAuto = "0"
	APIFanLow  = "1"
)

// ProductIDs are the known Aqua Temp / HiTemp product IDs. The device list
// endpoints filter by these.
var ProductIDs = []string{
	"1245226668902080512", // HiTemp
	"1132174963097280512",
	"1656269521923575808",
	"1663080854333558784",
	"1596427678569979904",
	"1674238226096406528",
	"1650063968998252544",
	"1668781858447085568",
	"1186904563333062656",
	"1158905952238313472",
	"1442284873216843776",
	"1732565142225256450",
	"1548963836789501952",
	"1669159229372477440",
	"1650758828508766208",
	"1664085465655808000",
}
